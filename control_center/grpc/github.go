package grpc

import (
	"control_center/config"
	"control_center/models"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func githubClientID() string     { return os.Getenv("GITHUB_CLIENT_ID") }
func githubClientSecret() string { return os.Getenv("GITHUB_CLIENT_SECRET") }
func githubRedirectURL() string  { return os.Getenv("GITHUB_REDIRECT_URL") }

// handleGitHubLogin redirects the user to GitHub OAuth authorization.
func handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:     "github_oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	redirectURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:public_key&state=%s",
		githubClientID(),
		url.QueryEscape(githubRedirectURL()),
		state,
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// handleGitHubCallback exchanges the OAuth code, fetches SSH keys,
// stores them in the DB, and redirects to the frontend with the session ID.
func handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("github_oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "github_oauth_state", MaxAge: -1, Path: "/"})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := exchangeGitHubCode(code)
	if err != nil {
		log.Println("GitHub token exchange failed:", err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	login, err := fetchGitHubLogin(token)
	if err != nil {
		log.Println("GitHub user fetch failed:", err)
		http.Error(w, "user fetch failed", http.StatusInternalServerError)
		return
	}

	keys, err := fetchGitHubSSHKeys(token)
	if err != nil {
		log.Println("GitHub SSH keys fetch failed:", err)
		keys = []string{}
	}

	keysJSON, _ := json.Marshal(keys)
	sessionID := randomState()
	config.Database.Create(&models.GitHubSession{
		ID:      sessionID,
		Login:   login,
		SSHKeys: string(keysJSON),
	})
	// Clean sessions older than 1 hour
	config.Database.Where("created_at < ?", time.Now().Add(-time.Hour)).Delete(&models.GitHubSession{})

	http.Redirect(w, r, "/?github_session="+sessionID, http.StatusFound)
}

// handleGitHubSession returns the stored login + SSH keys for a session ID.
func handleGitHubSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	var sess models.GitHubSession
	if err := config.Database.First(&sess, "id = ?", sessionID).Error; err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	var keys []string
	json.Unmarshal([]byte(sess.SSHKeys), &keys)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"login": sess.Login,
		"keys":  keys,
	})
}

// handleGitHubPublicKeys returns public SSH keys for a GitHub login (no auth needed).
func handleGitHubPublicKeys(w http.ResponseWriter, r *http.Request) {
	login := r.URL.Query().Get("login")
	if login == "" {
		http.Error(w, "missing login", http.StatusBadRequest)
		return
	}
	keys, err := fetchGitHubKeysPublic(login)
	if err != nil || len(keys) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"keys": []string{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"keys": keys})
}

// handleGitHubStudents returns all GitHub-connected students from the last 24h (for admin).
func handleGitHubStudents(w http.ResponseWriter, r *http.Request) {
	var sessions []models.GitHubSession
	config.Database.Where("created_at > ?", time.Now().Add(-24*time.Hour)).Find(&sessions)
	type entry struct {
		Login string   `json:"login"`
		Keys  []string `json:"keys"`
	}
	var result []entry
	for _, s := range sessions {
		var keys []string
		json.Unmarshal([]byte(s.SSHKeys), &keys)
		result = append(result, entry{Login: s.Login, Keys: keys})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func exchangeGitHubCode(code string) (string, error) {
	resp, err := http.PostForm("https://github.com/login/oauth/access_token", url.Values{
		"client_id":     {githubClientID()},
		"client_secret": {githubClientSecret()},
		"code":          {code},
		"redirect_uri":  {githubRedirectURL()},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return "", err
	}
	token := vals.Get("access_token")
	if token == "" {
		return "", fmt.Errorf("no access_token in response: %s", body)
	}
	return token, nil
}

func fetchGitHubLogin(token string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var user struct {
		Login string `json:"login"`
	}
	json.NewDecoder(resp.Body).Decode(&user)
	return user.Login, nil
}

func fetchGitHubSSHKeys(token string) ([]string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var keys []struct {
		Key string `json:"key"`
	}
	json.NewDecoder(resp.Body).Decode(&keys)
	var result []string
	for _, k := range keys {
		if k.Key != "" {
			result = append(result, k.Key)
		}
	}
	return result, nil
}

func fetchGitHubKeysPublic(login string) ([]string, error) {
	resp, err := http.Get("https://github.com/" + login + ".keys")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if line != "" {
			keys = append(keys, line)
		}
	}
	return keys, nil
}

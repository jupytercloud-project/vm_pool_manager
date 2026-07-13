package grpc

import (
	"context"
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
	"time"

	"github.com/danielgtaylor/huma/v2"
)

var githubHTTPClient = &http.Client{Timeout: 10 * time.Second}

// registerGitHubHuma : endpoints JSON /api/github/*. login + callback restent des handlers
// bruts (redirections OAuth 302, pas du JSON).
func registerGitHubHuma(api huma.API) {
	// GET /api/github/session?id= — login + clés SSH d'une session GitHub.
	huma.Register(api, huma.Operation{
		OperationID: "github-session", Method: http.MethodGet, Path: "/api/github/session",
		Summary: "Session GitHub (login + clés)", Tags: []string{"github"},
	}, func(ctx context.Context, in *struct {
		ID string `query:"id"`
	}) (*AnyOutput, error) {
		if in.ID == "" {
			return nil, huma.Error400BadRequest("missing id")
		}
		var sess models.GitHubSession
		if err := config.Database.First(&sess, "id = ?", in.ID).Error; err != nil {
			return nil, huma.Error404NotFound("session not found")
		}
		var keys []string
		json.Unmarshal([]byte(sess.SSHKeys), &keys)
		return &AnyOutput{Body: map[string]any{"login": sess.Login, "keys": keys}}, nil
	})

	// GET /api/github/public-keys?login= — clés SSH publiques d'un login GitHub.
	huma.Register(api, huma.Operation{
		OperationID: "github-public-keys", Method: http.MethodGet, Path: "/api/github/public-keys",
		Summary: "Clés SSH publiques d'un login GitHub", Tags: []string{"github"},
	}, func(ctx context.Context, in *struct {
		Login string `query:"login"`
	}) (*AnyOutput, error) {
		if in.Login == "" {
			return nil, huma.Error400BadRequest("missing login")
		}
		keys, err := fetchGitHubKeysPublic(in.Login)
		if err != nil || len(keys) == 0 {
			return &AnyOutput{Body: map[string]any{"keys": []string{}}}, nil
		}
		return &AnyOutput{Body: map[string]any{"keys": keys}}, nil
	})

	// GET /api/github/students — élèves connectés via GitHub (24 h). Réponse = tableau JSON.
	huma.Register(api, huma.Operation{
		OperationID: "github-students", Method: http.MethodGet, Path: "/api/github/students",
		Summary: "Élèves connectés via GitHub (24 h)", Tags: []string{"github"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		var sessions []models.GitHubSession
		config.Database.Where("created_at > ?", time.Now().Add(-24*time.Hour)).Find(&sessions)
		type entry struct {
			Login string   `json:"login"`
			Keys  []string `json:"keys"`
		}
		result := []entry{}
		for _, s := range sessions {
			var keys []string
			json.Unmarshal([]byte(s.SSHKeys), &keys)
			result = append(result, entry{Login: s.Login, Keys: keys})
		}
		return &AnyOutput{Body: result}, nil
	})
}

func randomState() string {
	b := make([]byte, 16) // 128 bits d'entropie : suffisant pour un state/session id non devinable
	if _, err := rand.Read(b); err != nil {
		// SÉCURITÉ : échouer fermé — sinon state/session_id prévisible (zéros).
		panic("crypto/rand indisponible: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func githubClientID() string     { return os.Getenv("GITHUB_CLIENT_ID") }
func githubClientSecret() string { return os.Getenv("GITHUB_CLIENT_SECRET") }
func githubRedirectURL() string  { return os.Getenv("GITHUB_REDIRECT_URL") }

func githubConfigured() bool {
	return githubClientID() != "" && githubClientSecret() != "" && githubRedirectURL() != ""
}

// handleGitHubLogin redirects the user to GitHub OAuth authorization.
func handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	if !githubConfigured() {
		http.Error(w, "GitHub OAuth not configured", http.StatusServiceUnavailable)
		return
	}
	state := randomState()
	config.Database.Create(&models.GitHubOAuthState{State: state})
	// Clean old states
	config.Database.Where("created_at < ?", time.Now().Add(-10*time.Minute)).Delete(&models.GitHubOAuthState{})
	redirectURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&scope=read:user&state=%s",
		githubClientID(),
		state,
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// handleGitHubCallback exchanges the OAuth code, fetches SSH keys,
// stores them in the DB, and redirects to the frontend with the session ID.
func handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateParam := r.URL.Query().Get("state")
	var oauthState models.GitHubOAuthState
	if err := config.Database.First(&oauthState, "state = ?", stateParam).Error; err != nil {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	config.Database.Delete(&oauthState)

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

	// SSH keys are public — fetch without token using the public API
	keys, err := fetchGitHubKeysPublic(login)
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

	http.Redirect(w, r, "/student?github_session="+sessionID, http.StatusFound)
}

func exchangeGitHubCode(code string) (string, error) {
	resp, err := githubHTTPClient.PostForm("https://github.com/login/oauth/access_token", url.Values{
		"client_id":     {githubClientID()},
		"client_secret": {githubClientSecret()},
		"code":          {code},
		"redirect_uri":  {githubRedirectURL()},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
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
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	if user.Login == "" {
		return "", fmt.Errorf("empty login in GitHub response")
	}
	return user.Login, nil
}

func fetchGitHubSSHKeys(token string) ([]string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var keys []struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, err
	}
	var result []string
	for _, k := range keys {
		if k.Key != "" {
			result = append(result, k.Key)
		}
	}
	return result, nil
}

func fetchGitHubKeysPublic(login string) ([]string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/users/"+login+"/keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var keys []struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, err
	}
	var result []string
	for _, k := range keys {
		if k.Key != "" {
			result = append(result, k.Key)
		}
	}
	return result, nil
}

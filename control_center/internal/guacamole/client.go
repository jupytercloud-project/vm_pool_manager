package guacamole

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

type Client struct {
	baseURL   string
	publicURL string
	adminUser string
	adminPass string
	sshUser   string
	sshKeyPEM string

	tokenMu      sync.Mutex
	cachedToken  string
	tokenExpires time.Time
}

// NewClientFromEnv creates a Client from environment variables.
// Returns nil, nil if GUACAMOLE_URL is not set (feature disabled).
func NewClientFromEnv() (*Client, error) {
	baseURL := os.Getenv("GUACAMOLE_URL")
	if baseURL == "" {
		return nil, nil
	}
	keyPath := os.Getenv("SSH_PRIVATE_KEY_PATH")
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("guacamole: read SSH key %s: %w", keyPath, err)
	}
	sshUser := os.Getenv("GUACAMOLE_SSH_USER")
	if sshUser == "" {
		sshUser = "vmuser"
	}
	publicURL := os.Getenv("GUACAMOLE_PUBLIC_URL")
	if publicURL == "" {
		publicURL = baseURL
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		publicURL: strings.TrimRight(publicURL, "/"),
		adminUser: os.Getenv("GUACAMOLE_ADMIN_USER"),
		adminPass: os.Getenv("GUACAMOLE_ADMIN_PASS"),
		sshUser:   sshUser,
		sshKeyPEM: string(keyPEM),
	}, nil
}

func (c *Client) getToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.cachedToken != "" && time.Now().Before(c.tokenExpires) {
		return c.cachedToken, nil
	}

	resp, err := httpClient.PostForm(c.baseURL+"/api/tokens", url.Values{
		"username": {c.adminUser},
		"password": {c.adminPass},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("guacamole auth %d: %s", resp.StatusCode, body)
	}
	var result struct {
		AuthToken string `json:"authToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AuthToken == "" {
		return "", fmt.Errorf("guacamole: empty auth token")
	}
	c.cachedToken = result.AuthToken
	c.tokenExpires = time.Now().Add(50 * time.Minute)
	return result.AuthToken, nil
}

// CreateSSHConnection registers a new SSH connection in Guacamole and returns its identifier.
func (c *Client) CreateSSHConnection(name, ip string) (string, error) {
	token, err := c.getToken()
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"name":     name,
		"protocol": "ssh",
		"parameters": map[string]string{
			"hostname":    ip,
			"port":        "22",
			"username":    c.sshUser,
			"private-key": c.sshKeyPEM,
		},
		"attributes": map[string]string{},
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/session/data/mysql/connections?token=%s", c.baseURL, token),
		strings.NewReader(string(data)),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("guacamole create connection %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Identifier string `json:"identifier"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Identifier == "" {
		return "", fmt.Errorf("guacamole: empty connection identifier")
	}
	return result.Identifier, nil
}

// DeleteConnection removes a Guacamole connection by its identifier.
func (c *Client) DeleteConnection(connID string) error {
	token, err := c.getToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/session/data/mysql/connections/%s?token=%s", c.baseURL, connID, token),
		nil,
	)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// BuildClientURL returns the absolute URL to open a Guacamole SSH session.
// Uses GUACAMOLE_PUBLIC_URL so the browser hits Guacamole directly without going through the Vite proxy.
func (c *Client) BuildClientURL(connID string) string {
	payload := connID + "\x00c\x00mysql"
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	return c.publicURL + "/#/client/" + encoded
}

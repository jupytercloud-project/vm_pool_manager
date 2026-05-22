package oidc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type glauthUser struct {
	Name         string   `json:"name"`
	OtherGroups  []int    `json:"otherGroups"`
	PassSHA256   string   `json:"passSHA256"`
	PrimaryGroup int      `json:"primaryGroup"`
	Mail         string   `json:"mail"`
	UIDNumber    int      `json:"uidNumber,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

func glauthAPIBase() string {
	u := os.Getenv("GLAUTH_API_URL")
	if u == "" {
		u = "http://localhost:5555"
	}
	return u
}

// CreateLDAPUser adds a user to GLAuth via its REST API.
// passsha256 must be the SHA-256 hex hash of the password.
func CreateLDAPUser(name, email, passsha256 string, isAdmin bool) error {
	primaryGroup := 5502 // users group
	otherGroups := []int{}
	if isAdmin {
		primaryGroup = 5501 // admins group
	}

	payload := glauthUser{
		Name:         name,
		Mail:         email,
		PassSHA256:   passsha256,
		PrimaryGroup: primaryGroup,
		OtherGroups:  otherGroups,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(glauthAPIBase()+"/v2/users", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("glauth api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("glauth api returned %d", resp.StatusCode)
	}
	return nil
}

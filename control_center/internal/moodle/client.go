// Package moodle est un client minimal des Web Services REST de Moodle.
// Config via env : MOODLE_URL (ex: http://localhost:8081) + MOODLE_TOKEN (token du service cpm_service).
package moodle

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	http    *http.Client
}

// Configured indique si Moodle est paramétré (pour activer conditionnellement endpoints/UI).
func Configured() bool {
	return os.Getenv("MOODLE_URL") != "" && os.Getenv("MOODLE_TOKEN") != ""
}

// New construit un client à partir de MOODLE_URL / MOODLE_TOKEN.
func New() (*Client, error) {
	base := strings.TrimRight(os.Getenv("MOODLE_URL"), "/")
	tok := os.Getenv("MOODLE_TOKEN")
	if base == "" || tok == "" {
		return nil, fmt.Errorf("Moodle non configuré (MOODLE_URL / MOODLE_TOKEN manquants)")
	}
	return &Client{BaseURL: base, Token: tok, http: &http.Client{Timeout: 20 * time.Second}}, nil
}

// BaseHost renvoie l'URL publique de Moodle (pour les liens UI / deep-links).
func (c *Client) BaseHost() string { return c.BaseURL }

// callWith appelle une fonction WS avec un token donné et renvoie le JSON brut.
// Détecte l'enveloppe d'erreur Moodle ({exception,errorcode,message}, renvoyée en HTTP 200).
func (c *Client) callWith(token, fn string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("wstoken", token)
	params.Set("wsfunction", fn)
	params.Set("moodlewsrestformat", "json")

	resp, err := c.http.PostForm(c.BaseURL+"/webservice/rest/server.php", params)
	if err != nil {
		return nil, fmt.Errorf("appel Moodle %s: %w", fn, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var e struct {
		Exception string `json:"exception"`
		ErrorCode string `json:"errorcode"`
		Message   string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil && e.Exception != "" {
		return nil, fmt.Errorf("Moodle %s: %s (%s)", fn, e.Message, e.ErrorCode)
	}
	return body, nil
}

func (c *Client) call(fn string, params url.Values) ([]byte, error) {
	return c.callWith(c.Token, fn, params)
}

// ─── Identité / auth ────────────────────────────────────────────────────────

type SiteInfo struct {
	SiteName        string `json:"sitename"`
	Username        string `json:"username"`
	UserID          int    `json:"userid"`
	FullName        string `json:"fullname"`
	Release         string `json:"release"`
	UserIsSiteAdmin bool   `json:"userissiteadmin"`
}

type MoodleUserInfo struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"fullname"`
}

// userByField interroge core_user_get_users_by_field avec le token de SERVICE (admin),
// qui peut lire l'email de n'importe quel utilisateur (le token utilisateur, lui, ne le peut pas).
func (c *Client) userByField(field, value string) (*MoodleUserInfo, error) {
	v := url.Values{}
	v.Set("field", field)
	v.Set("values[0]", value)
	body, err := c.call("core_user_get_users_by_field", v)
	if err != nil {
		return nil, err
	}
	var users []MoodleUserInfo
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("user_by_%s: %w", field, err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("utilisateur introuvable (%s=%s)", field, value)
	}
	return &users[0], nil
}

// UserByUsername renvoie l'identité d'un utilisateur à partir de son login.
func (c *Client) UserByUsername(username string) (*MoodleUserInfo, error) {
	return c.userByField("username", username)
}

// SiteInfo via le token de service (sanity check).
func (c *Client) SiteInfo() (*SiteInfo, error) { return c.siteInfo(c.Token) }

func (c *Client) siteInfo(token string) (*SiteInfo, error) {
	body, err := c.callWith(token, "core_webservice_get_site_info", nil)
	if err != nil {
		return nil, err
	}
	var si SiteInfo
	if err := json.Unmarshal(body, &si); err != nil {
		return nil, fmt.Errorf("site_info: %w", err)
	}
	return &si, nil
}

// LoginToken valide des identifiants Moodle via login/token.php et renvoie un token utilisateur.
// service vide => "moodle_mobile_app".
func (c *Client) LoginToken(username, password, service string) (string, error) {
	if service == "" {
		service = "moodle_mobile_app"
	}
	v := url.Values{}
	v.Set("username", username)
	v.Set("password", password)
	v.Set("service", service)
	resp, err := c.http.PostForm(c.BaseURL+"/login/token.php", v)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Token     string `json:"token"`
		Error     string `json:"error"`
		ErrorCode string `json:"errorcode"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("login Moodle: réponse illisible")
	}
	if r.Error != "" {
		return "", fmt.Errorf("login Moodle: %s", r.Error)
	}
	if r.Token == "" {
		return "", fmt.Errorf("login Moodle: identifiants invalides")
	}
	return r.Token, nil
}

// IdentityFromCredentials valide les identifiants et renvoie l'identité de l'utilisateur.
func (c *Client) IdentityFromCredentials(username, password, service string) (*SiteInfo, error) {
	token, err := c.LoginToken(username, password, service)
	if err != nil {
		return nil, err
	}
	return c.siteInfo(token)
}

// ─── Cours / inscriptions ───────────────────────────────────────────────────

type Course struct {
	ID        int    `json:"id"`
	ShortName string `json:"shortname"`
	FullName  string `json:"fullname"`
}

// GetCourses liste les cours (hors cours-site id=1).
// On utilise core_course_get_courses_by_field (sans filtre) : core_course_get_courses
// échoue sur un verrou de cache (ex_unabletolock) avec cette image Moodle.
func (c *Client) GetCourses() ([]Course, error) {
	body, err := c.call("core_course_get_courses_by_field", nil)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Courses []Course `json:"courses"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("get_courses: %w", err)
	}
	out := make([]Course, 0, len(wrap.Courses))
	for _, co := range wrap.Courses {
		if co.ID != 1 {
			out = append(out, co)
		}
	}
	return out, nil
}

type EnrolledUser struct {
	ID       int    `json:"id"`
	FullName string `json:"fullname"`
	Email    string `json:"email"`
	Roles    []struct {
		ShortName string `json:"shortname"`
	} `json:"roles"`
}

// IsTeacher indique si l'utilisateur a un rôle enseignant/manager dans le cours.
func (u EnrolledUser) IsTeacher() bool {
	for _, r := range u.Roles {
		switch r.ShortName {
		case "editingteacher", "teacher", "manager", "coursecreator":
			return true
		}
	}
	return false
}

// GetEnrolledUsers renvoie les utilisateurs inscrits à un cours.
func (c *Client) GetEnrolledUsers(courseID int) ([]EnrolledUser, error) {
	v := url.Values{}
	v.Set("courseid", strconv.Itoa(courseID))
	body, err := c.call("core_enrol_get_enrolled_users", v)
	if err != nil {
		return nil, err
	}
	var users []EnrolledUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("get_enrolled_users: %w", err)
	}
	return users, nil
}

// ─── Devoirs (mod_assign) ───────────────────────────────────────────────────

type Assignment struct {
	ID       int     `json:"id"`
	CMID     int     `json:"cmid"`
	Name     string  `json:"name"`
	MaxGrade float64 `json:"max_grade"`
}

// GetAssignments liste les activités "devoir" d'un cours (cible du push de notes).
func (c *Client) GetAssignments(courseID int) ([]Assignment, error) {
	v := url.Values{}
	v.Set("courseids[0]", strconv.Itoa(courseID))
	body, err := c.call("mod_assign_get_assignments", v)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Courses []struct {
			Assignments []struct {
				ID    int         `json:"id"`
				CMID  int         `json:"cmid"`
				Name  string      `json:"name"`
				Grade json.Number `json:"grade"`
			} `json:"assignments"`
		} `json:"courses"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("get_assignments: %w", err)
	}
	var out []Assignment
	for _, co := range resp.Courses {
		for _, a := range co.Assignments {
			max, _ := a.Grade.Float64()
			if max <= 0 {
				max = 100
			}
			out = append(out, Assignment{ID: a.ID, CMID: a.CMID, Name: a.Name, MaxGrade: max})
		}
	}
	return out, nil
}

// ─── Notes ──────────────────────────────────────────────────────────────────

// SaveAssignGrade pousse une note sur une activité "devoir" (mod_assign) pour un utilisateur.
func (c *Client) SaveAssignGrade(assignmentID, moodleUserID int, grade float64, feedback string) error {
	v := url.Values{}
	v.Set("assignmentid", strconv.Itoa(assignmentID))
	v.Set("userid", strconv.Itoa(moodleUserID))
	v.Set("grade", strconv.FormatFloat(grade, 'f', 2, 64))
	v.Set("attemptnumber", "-1")
	v.Set("addattempt", "0")
	v.Set("workflowstate", "")
	v.Set("applytoall", "1")
	if feedback != "" {
		v.Set("plugindata[assignfeedbackcomments_editor][text]", feedback)
		v.Set("plugindata[assignfeedbackcomments_editor][format]", "1")
	}
	_, err := c.call("mod_assign_save_grade", v)
	return err
}

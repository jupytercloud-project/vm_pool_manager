// Package xcatalogue interroge les endpoints JSON de l'enseignement de l'X
// (catalogue de cours + affectations profs/élèves + groupes), fournis par la DSI.
//
// Config via env :
//   - XCOURSES_URL   : base des endpoints (défaut https://www.enseignement.polytechnique.fr/data/moodle)
//   - XCOURSES_TOKEN : token d'accès fixe pour les endpoints protégés (affectations), envoyé en POST.
//
// Le catalogue est public (GET). Les affectations sont protégées (token POST + filtrage IP côté DSI).
// IMPORTANT : le token vient d'une variable d'environnement, jamais du code/Git.
package xcatalogue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultBaseURL = "https://www.enseignement.polytechnique.fr/data/moodle"

// Course : une entrée du catalogue public.
type Course struct {
	ID       string `json:"id"`       // shortname = CODE_EP-ANNÉE (clé de jointure unique)
	Title    string `json:"title"`    // ex: "CSC_41M03_EP - ... (2025-2026)"
	Category string `json:"category"` // ex: "Bachelor 1"
}

// Member : une affectation à un cours (prof ou élève).
type Member struct {
	Username string `json:"username"` // uid@polytechnique.fr (login SAML)
	Role     string `json:"role"`     // "editingteacher" | "student"
}

// IsTeacher indique un encadrant (rôle editingteacher).
func (m Member) IsTeacher() bool { return m.Role == "editingteacher" }

// GroupMember : appartenance d'un élève à un groupe (TD/PC).
type GroupMember struct {
	Username  string `json:"username"`
	GroupID   string `json:"id_groupe"`
	GroupName string `json:"nom_groupe"`
}

// Client appelle les endpoints de l'enseignement.
type Client struct {
	BaseURL string
	Token   string
	http    *http.Client
}

// mockEnabled : XCOURSES_MOCK=1 fait renvoyer des affectations FICTIVES (test du flux
// d'import avant l'obtention du vrai token DSI). Le mock ne s'active QUE si aucun token
// réel n'est configuré → dès que XCOURSES_TOKEN est renseigné, on passe au vrai endpoint.
func mockEnabled() bool { return os.Getenv("XCOURSES_MOCK") == "1" }

// Configured indique si les endpoints PROTÉGÉS sont utilisables (token présent, ou mock).
// Le catalogue, lui, est public et marche sans token.
func Configured() bool { return os.Getenv("XCOURSES_TOKEN") != "" || mockEnabled() }

// New construit un client (l'URL a un défaut public ; le token est optionnel pour le catalogue).
func New() *Client {
	base := strings.TrimRight(os.Getenv("XCOURSES_URL"), "/")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		BaseURL: base,
		Token:   os.Getenv("XCOURSES_TOKEN"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

// Catalogue récupère la liste des cours (public). year/dep optionnels (filtres GET).
func (c *Client) Catalogue(year, dep string) ([]Course, error) {
	q := url.Values{}
	if year != "" {
		q.Set("year", year)
	}
	if dep != "" {
		q.Set("dep", dep)
	}
	u := c.BaseURL + "/catalogue.php"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("catalogue: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalogue: HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out []Course
	if err := decodeJSONOrNDJSON(body, &out); err != nil {
		return nil, fmt.Errorf("catalogue: parse: %w", err)
	}
	return out, nil
}

// CourseMembers récupère profs + élèves d'un cours (endpoint protégé : token POST).
func (c *Client) CourseMembers(id string) ([]Member, error) {
	if mockEnabled() { // mock prioritaire : permet de garder le token en .env tout en testant en local (IP non whitelistée)
		return mockMembers(), nil
	}
	body, err := c.postProtected("affectations_cours.php", id)
	if err != nil {
		return nil, err
	}
	var out []Member
	if err := decodeJSONOrNDJSON(body, &out); err != nil {
		return nil, fmt.Errorf("affectations_cours: parse: %w", err)
	}
	return out, nil
}

// CourseGroups récupère les groupes (TD/PC) d'un cours (endpoint protégé : token POST).
func (c *Client) CourseGroups(id string) ([]GroupMember, error) {
	if mockEnabled() { // mock prioritaire (cf. CourseMembers)
		return mockGroups(id), nil
	}
	body, err := c.postProtected("affectations_groupes.php", id)
	if err != nil {
		return nil, err
	}
	var out []GroupMember
	if err := decodeJSONOrNDJSON(body, &out); err != nil {
		return nil, fmt.Errorf("affectations_groupes: parse: %w", err)
	}
	return out, nil
}

// postProtected appelle un endpoint protégé en POST avec l'id du cours et le token.
// NB : le nom exact du champ token (ici "token") est à confirmer avec la DSI ;
// changer ici si besoin sans toucher au reste.
func (c *Client) postProtected(endpoint, id string) ([]byte, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("XCOURSES_TOKEN manquant (endpoint protégé indisponible)")
	}
	form := url.Values{}
	form.Set("id", id)
	form.Set("token", c.Token)
	u := c.BaseURL + "/" + endpoint + "?id=" + url.QueryEscape(id)
	resp, err := c.http.Post(u, "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%s: accès refusé (token/IP) — HTTP %d", endpoint, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", endpoint, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// mockMembers : affectations fictives pour tester le flux d'import (XCOURSES_MOCK=1).
// admin@polytechnique.fr est inclus comme élève : en te loggant en admin (.edu),
// la tolérance d'uid à l'attribution te permet de récupérer une VM pour valider la chaîne.
func mockMembers() []Member {
	return []Member{
		{Username: "baptiste.desprez@polytechnique.fr", Role: "editingteacher"},
		{Username: "admin@polytechnique.fr", Role: "student"},
		{Username: "jean.dupont@polytechnique.fr", Role: "student"},
		{Username: "marie.curie@polytechnique.fr", Role: "student"},
		{Username: "paul.martin@polytechnique.fr", Role: "student"},
		{Username: "lucie.bernard@polytechnique.fr", Role: "student"},
		{Username: "hugo.petit@polytechnique.fr", Role: "student"},
		{Username: "emma.durand@polytechnique.fr", Role: "student"},
		{Username: "louis.moreau@polytechnique.fr", Role: "student"},
		{Username: "chloe.laurent@polytechnique.fr", Role: "student"},
	}
}

// mockGroups : répartit les élèves fictifs en 2 groupes (PC-1 / PC-2).
func mockGroups(id string) []GroupMember {
	var out []GroupMember
	i := 0
	for _, m := range mockMembers() {
		if m.IsTeacher() {
			continue
		}
		grp, name := id+"-GROUP-1", "PC-1"
		if i%2 == 1 {
			grp, name = id+"-GROUP-2", "PC-2"
		}
		out = append(out, GroupMember{Username: m.Username, GroupID: grp, GroupName: name})
		i++
	}
	return out
}

// decodeJSONOrNDJSON accepte soit un tableau JSON ([{...},{...}]), soit du JSON-Lines
// (un objet par ligne) — les exemples DSI montrent les deux formes selon l'endpoint.
func decodeJSONOrNDJSON[T any](body []byte, out *[]T) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(trimmed, out)
	}
	sc := bufio.NewScanner(bytes.NewReader(trimmed))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return err
		}
		*out = append(*out, item)
	}
	return sc.Err()
}

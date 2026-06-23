package grpc

import (
	"fmt"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"
)

// Contrôle d'accès des reverse-proxies applicatifs (JupyterLab, code-server).
//
// Principe : la VM cible n'est JAMAIS choisie par le client à partir d'une IP — elle est
// résolue côté serveur depuis l'identité authentifiée et le pool. Un élève ne peut donc
// pas atteindre la VM d'un autre par construction (anti-IDOR). Trois cas ouvrent l'accès :
//   - sa propre VM (target == self) ;
//   - l'équipe pédagogique (admin/prof/ta) → n'importe quelle VM du pool ;
//   - (grant) un partage valide émis par la cible, présenté via mot de passe.

// Ports applicatifs résolus côté serveur.
const (
	portJupyter      = 8888 // JupyterLab
	portCodeServerRW = 8443 // code-server (lecture + écriture)
	portCodeServerRO = 8444 // code-server lecture seule (instance dédiée, Phase C)
)

// instructorProxyAllowed autorise l'accès au proxy de la VM « propriétaire » d'un pool
// (la VM instructeur servie par /api/jupyter-proxy/{pool}/{owner}). Réservé à l'équipe
// pédagogique ou au propriétaire lui-même ; un élève n'a aucune raison d'y accéder.
func instructorProxyAllowed(id httpIdentity, ownerID string) bool {
	if isStaff(id.Role) {
		return true
	}
	return id.Email != "" && strings.EqualFold(strings.TrimSpace(id.Email), strings.TrimSpace(ownerID))
}

// poolExists vérifie qu'un couple (serverpool_id, owner) correspond à un pool réel.
func poolExists(poolID, ownerID string) bool {
	var count int64
	config.Database.Model(&models.Serverpool{}).
		Where("serverpool_id = ? AND user_id = ?", poolID, ownerID).
		Count(&count)
	return count > 0
}

// requireProxyIdentity extrait l'identité authentifiée d'une requête de proxy. Les routes
// de proxy passent par httpAuthMiddleware (préfixe /api/), donc l'identité est déjà dans
// le contexte ; on refuse proprement si elle manque (défense en profondeur).
func requireProxyIdentity(w http.ResponseWriter, r *http.Request) (httpIdentity, bool) {
	id, ok := identityFrom(r.Context())
	if !ok || id.Email == "" {
		http.Error(w, "authentification requise", http.StatusUnauthorized)
		return httpIdentity{}, false
	}
	return id, true
}

// resolveInstructorVM renvoie (UUID, IP) de la VM instructeur d'un pool : la plus ancienne
// VM du couple (pool, owner). C'est la convention déjà utilisée par le flux nbgrader.
func resolveInstructorVM(poolID, ownerID string) (string, string, error) {
	var server models.Server
	if err := config.Database.
		Where("serverpool_id = ? AND user_id = ?", poolID, ownerID).
		Order("created_at ASC").
		First(&server).Error; err != nil {
		return "", "", fmt.Errorf("VM instructeur introuvable pour le pool %s", poolID)
	}
	if server.IP_Address == "" {
		return "", "", fmt.Errorf("la VM instructeur n'a pas encore d'IP")
	}
	return server.ID, server.IP_Address, nil
}

// resolveStudentVM renvoie (UUID, IP) de la VM affectée à un élève (par identité) dans un
// pool. La jointure élève↔VM passe par list_students.student.ip (renseigné à
// l'attribution), puis on retrouve le server par son IP pour obtenir son UUID. La
// comparaison d'identité tolère email exact OU MoodleEmail (login ↔ ligne student).
func resolveStudentVM(poolID, ownerID, studentEmail string) (string, string, error) {
	studentEmail = strings.TrimSpace(studentEmail)
	if studentEmail == "" {
		return "", "", fmt.Errorf("identité élève vide")
	}
	var pool models.Serverpool
	if err := config.Database.
		Where("serverpool_id = ? AND user_id = ?", poolID, ownerID).
		First(&pool).Error; err != nil {
		return "", "", fmt.Errorf("pool introuvable")
	}
	var list models.ListStudents
	if err := config.Database.Preload("Students").Where("pool_id = ?", pool.ID).First(&list).Error; err != nil {
		return "", "", fmt.Errorf("liste d'élèves introuvable pour ce pool")
	}
	for _, st := range list.Students {
		if strings.EqualFold(st.Name, studentEmail) || strings.EqualFold(st.MoodleEmail, studentEmail) {
			if st.IP == "" {
				return "", "", fmt.Errorf("aucune VM affectée à %s", studentEmail)
			}
			var server models.Server
			if err := config.Database.
				Where("serverpool_id = ? AND ip_address = ?", poolID, st.IP).
				First(&server).Error; err != nil {
				return "", "", fmt.Errorf("VM (%s) introuvable en base", st.IP)
			}
			return server.ID, st.IP, nil
		}
	}
	return "", "", fmt.Errorf("élève %s absent de ce pool", studentEmail)
}

// proxyTarget = VM résolue + mode, prête à être figée dans une ProxySession.
type proxyTarget struct {
	Target string // identité de la VM cible (email)
	VMID   string // UUID de la VM (segment de chemin du proxy)
	IP     string
	Port   int
	Mode   string // "read" | "write"
}

// resolveProxyTarget applique le contrôle d'accès et résout la VM cible côté serveur.
//
//	kind   : "jupyter" | "vscode"
//	target : "self" (sa VM) | "instructor" (VM instructeur) | <email élève> (staff only)
//	mode   : "read" | "write" (vscode ; ignoré pour jupyter)
//
// Renvoie une erreur HTTP prête à servir si l'accès est refusé.
func resolveProxyTarget(id httpIdentity, kind, poolID, ownerID, target, mode string) (proxyTarget, int, error) {
	if !poolExists(poolID, ownerID) {
		return proxyTarget{}, http.StatusNotFound, fmt.Errorf("pool introuvable")
	}
	staff := isStaff(id.Role)
	target = strings.TrimSpace(target)
	if target == "" {
		target = "self"
	}
	if mode != "read" {
		mode = "write"
	}

	switch target {
	case "instructor":
		if !instructorProxyAllowed(id, ownerID) {
			return proxyTarget{}, http.StatusForbidden, fmt.Errorf("accès refusé")
		}
		vmID, ip, err := resolveInstructorVM(poolID, ownerID)
		if err != nil {
			return proxyTarget{}, http.StatusServiceUnavailable, err
		}
		return proxyTarget{Target: ownerID, VMID: vmID, IP: ip, Port: kindPort(kind, "write"), Mode: "write"}, 0, nil

	case "self":
		// Sa propre VM : un élève accède en écriture à sa machine ; un membre du staff
		// « self » signifie la VM instructeur du pool.
		if staff {
			vmID, ip, err := resolveInstructorVM(poolID, ownerID)
			if err != nil {
				return proxyTarget{}, http.StatusServiceUnavailable, err
			}
			return proxyTarget{Target: ownerID, VMID: vmID, IP: ip, Port: kindPort(kind, "write"), Mode: "write"}, 0, nil
		}
		vmID, ip, err := resolveStudentVM(poolID, ownerID, id.Email)
		if err != nil {
			return proxyTarget{}, http.StatusServiceUnavailable, err
		}
		return proxyTarget{Target: id.Email, VMID: vmID, IP: ip, Port: kindPort(kind, "write"), Mode: "write"}, 0, nil

	default:
		// Cibler explicitement la VM d'un autre élève : réservé au staff (le prof revoit
		// le code d'un élève). Entre élèves, c'est le flux « grant » (cf. join) qui ouvre
		// la session, jamais ce chemin.
		if !staff {
			return proxyTarget{}, http.StatusForbidden, fmt.Errorf("accès refusé : seul l'enseignant peut ouvrir la VM d'un autre élève")
		}
		vmID, ip, err := resolveStudentVM(poolID, ownerID, target)
		if err != nil {
			return proxyTarget{}, http.StatusServiceUnavailable, err
		}
		return proxyTarget{Target: target, VMID: vmID, IP: ip, Port: kindPort(kind, mode), Mode: mode}, 0, nil
	}
}

// kindPort résout le port applicatif selon le type et le mode.
func kindPort(kind, mode string) int {
	if kind == "vscode" {
		if mode == "read" {
			return portCodeServerRO
		}
		return portCodeServerRW
	}
	return portJupyter
}

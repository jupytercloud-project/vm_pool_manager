package grpc

import (
	"context"
	"net/http"
	"strings"

	"control_center/config"
	"control_center/models"

	"github.com/danielgtaylor/huma/v2"
)

// Rôles canoniques de la plateforme.
const (
	RoleAdmin     = "admin"
	RoleProf      = "prof"
	RoleTA        = "ta"        // assistant / chargé de TD
	RoleStudent   = "student"   // élève (défaut)
	RoleChercheur = "chercheur" // usage recherche / calcul
)

var validRoles = map[string]bool{
	RoleAdmin: true, RoleProf: true, RoleTA: true, RoleStudent: true, RoleChercheur: true,
}

// isStaff : rôles autorisés à gérer pools / cours / étudiants (équipe pédagogique).
func isStaff(role string) bool {
	return role == RoleAdmin || role == RoleProf || role == RoleTA
}

// findUserByEmailOrUID cherche un utilisateur par email exact, puis (à défaut) par uid
// (partie avant @) — les cours de l'X renvoient uid@polytechnique.fr alors que le login
// établissement peut être en .edu : comparer l'uid évite l'échec de résolution de rôle.
func findUserByEmailOrUID(email string) (models.User, bool) {
	le := strings.ToLower(strings.TrimSpace(email))
	var u models.User
	if le == "" {
		return u, false
	}
	if err := config.Database.Where("LOWER(email) = ?", le).First(&u).Error; err == nil {
		return u, true
	}
	if err := config.Database.
		Where("LOWER(split_part(email, '@', 1)) = LOWER(split_part(?, '@', 1))", le).
		First(&u).Error; err == nil {
		return u, true
	}
	return models.User{}, false
}

// resolveRole détermine le rôle effectif d'un email authentifié.
// Le groupe OIDC "admins" force admin ; sinon on lit le rôle en base (un admin peut
// l'attribuer) ; à défaut on crée une ligne user en "student".
func resolveRole(email string, inAdminsGroup bool) string {
	if strings.TrimSpace(email) == "" {
		return RoleStudent
	}
	u, found := findUserByEmailOrUID(email)
	if inAdminsGroup {
		if !found {
			config.Database.Create(&models.User{Email: strings.ToLower(email), Name: email, Role: RoleAdmin})
		}
		return RoleAdmin
	}
	if found {
		if u.Role == "" {
			return RoleStudent
		}
		return u.Role
	}
	config.Database.Create(&models.User{Email: strings.ToLower(email), Name: email, Role: RoleStudent})
	return RoleStudent
}

// upsertUserRole crée ou met à jour le rôle d'un utilisateur (par email).
func upsertUserRole(email, role string) error {
	le := strings.ToLower(strings.TrimSpace(email))
	if le == "" || !validRoles[role] {
		return nil
	}
	var u models.User
	if err := config.Database.Where("LOWER(email) = ?", le).First(&u).Error; err != nil {
		return config.Database.Create(&models.User{Email: le, Name: le, Role: role}).Error
	}
	return config.Database.Model(&u).Update("role", role).Error
}

// GET /api/me est désormais servi par HUMA (registerHumaRoutes dans huma.go).

// registerAdminUsersHuma : GET /api/admin/users + POST /api/admin/users/role (admin uniquement).
func registerAdminUsersHuma(api huma.API) {
	// GET /api/admin/users — liste des utilisateurs et rôles.
	huma.Register(api, huma.Operation{
		OperationID: "list-users", Method: http.MethodGet, Path: "/api/admin/users",
		Summary: "Lister les utilisateurs et rôles", Tags: []string{"admin"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		var users []models.User
		config.Database.Order("email ASC").Find(&users)
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			out = append(out, map[string]any{"email": u.Email, "name": u.Name, "role": u.Role})
		}
		return &AnyOutput{Body: map[string]any{"users": out, "roles": []string{
			RoleAdmin, RoleProf, RoleTA, RoleChercheur, RoleStudent,
		}}}, nil
	})

	// POST /api/admin/users/role {email, role} — attribue un rôle.
	huma.Register(api, huma.Operation{
		OperationID: "set-user-role", Method: http.MethodPost, Path: "/api/admin/users/role",
		Summary: "Attribuer un rôle", Tags: []string{"admin"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
	}) (*AnyOutput, error) {
		if strings.TrimSpace(in.Body.Email) == "" || !validRoles[in.Body.Role] {
			return nil, huma.Error400BadRequest("email et rôle valides requis")
		}
		if err := upsertUserRole(in.Body.Email, in.Body.Role); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return &AnyOutput{Body: map[string]any{"email": strings.ToLower(in.Body.Email), "role": in.Body.Role}}, nil
	})
}

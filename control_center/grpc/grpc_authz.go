package grpc

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"

	"control_center/internal/oidc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcWebAllowedOrigins : ensemble d'origines autorisées pour gRPC-Web, lu une fois depuis
// CORS_ALLOWED_ORIGINS. Vide => toutes origines acceptées (compat) avec avertissement.
var (
	grpcWebOriginsOnce sync.Once
	grpcWebOrigins     map[string]bool
)

func loadGRPCWebOrigins() {
	grpcWebOrigins = map[string]bool{}
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		log.Printf("[secu] CORS_ALLOWED_ORIGINS non défini : gRPC-Web accepte TOUTES les origines. " +
			"Définir la liste des origines (ex: https://mon-domaine) en production.")
		return
	}
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			grpcWebOrigins[strings.ToLower(o)] = true
		}
	}
}

// grpcWebOriginAllowed applique l'allowlist (défense en profondeur CSRF pour gRPC-Web).
func grpcWebOriginAllowed(origin string) bool {
	grpcWebOriginsOnce.Do(loadGRPCWebOrigins)
	return originInAllowlist(origin)
}

// originInAllowlist : matcher pur (sans sync.Once), testable. Allowlist vide → tout autorisé.
func originInAllowlist(origin string) bool {
	if len(grpcWebOrigins) == 0 {
		return true // non configuré → compat historique
	}
	return grpcWebOrigins[strings.ToLower(strings.TrimSpace(origin))]
}

// sensitiveGRPCMethods : méthodes gRPC à fort pouvoir (exécution de cloud-init arbitraire,
// provisioning/suppression d'infra, gestion des élèves d'un pool). Le serveur gRPC (et
// gRPC-Web) n'accepte que des JWT OIDC (Dex) — donc déjà réservé au personnel établissement —
// mais on ajoute une AUTORISATION par méthode (défense en profondeur) : un principal de rôle
// « student » (compte OIDC non-staff) ne doit jamais pouvoir créer un cloud-init (RCE root sur
// les VMs) ni provisionner/détruire des pools. Les profs, TA, admins et chercheurs restent
// autorisés (un chercheur a un usage légitime de calcul). Les lectures et le flux temps réel
// (UserService) ne sont pas gatés ici.
var sensitiveGRPCMethods = map[string]bool{
	"/frontcontrol.AuthService/CreateUser":     true,
	"/frontcontrol.ConfigService/CreateConfig": true,
	"/frontcontrol.ConfigService/UpdateConfig": true,
	"/frontcontrol.ConfigService/DeleteConfig": true,
	"/frontcontrol.PoolService/CreatePool":     true,
	"/frontcontrol.PoolService/DeletePool":     true,
	"/frontcontrol.PoolService/RebuildServer":  true,
	"/frontcontrol.PoolService/AddServer":      true,
	"/frontcontrol.PoolService/AddSSHKeys":     true,
	"/frontcontrol.PoolService/AddStudents":    true,
	"/frontcontrol.PoolService/DeleteStudent":  true,
	"/frontcontrol.PoolService/ListStudents":   true,
}

// methodDeniedForRole : décision d'autorisation PURE (sans I/O), testable isolément.
// Une méthode sensible est refusée à un rôle « student » (ou inconnu/vide) ; tous les autres
// rôles (prof, ta, admin, chercheur) sont autorisés. Les méthodes non sensibles passent.
func methodDeniedForRole(fullMethod, role string) bool {
	if !sensitiveGRPCMethods[fullMethod] {
		return false
	}
	return role == RoleStudent || role == ""
}

// grpcRoleFromCtx dérive le rôle applicatif à partir des claims OIDC validés et injectés
// par oidcmw.UnaryInterceptor (email + appartenance au groupe "admins").
func grpcRoleFromCtx(ctx context.Context) string {
	email, _ := oidc.EmailFromContext(ctx)
	admin := false
	for _, g := range oidc.GroupsFromContext(ctx) {
		if g == "admins" {
			admin = true
		}
	}
	return resolveRole(email, admin)
}

// authzUnaryInterceptor refuse les méthodes sensibles aux principaux de rôle « student ».
// À chaîner APRÈS oidcmw.UnaryInterceptor (les claims doivent déjà être en contexte).
func authzUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if sensitiveGRPCMethods[info.FullMethod] && methodDeniedForRole(info.FullMethod, grpcRoleFromCtx(ctx)) {
		return nil, status.Error(codes.PermissionDenied, "action réservée à l'équipe pédagogique")
	}
	return handler(ctx, req)
}

// authzStreamInterceptor : équivalent pour les RPC en flux.
func authzStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if sensitiveGRPCMethods[info.FullMethod] && methodDeniedForRole(info.FullMethod, grpcRoleFromCtx(ss.Context())) {
		return status.Error(codes.PermissionDenied, "action réservée à l'équipe pédagogique")
	}
	return handler(srv, ss)
}

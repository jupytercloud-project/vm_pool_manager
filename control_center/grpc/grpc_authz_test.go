package grpc

import "testing"

// TestMethodDeniedForRole vérifie le role-gate par méthode gRPC (défense en profondeur) :
// les méthodes sensibles (cloud-init arbitraire, provisioning, gestion des élèves, création de
// comptes) sont refusées au rôle « student » / inconnu, et autorisées aux autres rôles.
func TestMethodDeniedForRole(t *testing.T) {
	sensitive := []string{
		"/frontcontrol.ConfigService/CreateConfig",
		"/frontcontrol.ConfigService/UpdateConfig",
		"/frontcontrol.ConfigService/DeleteConfig",
		"/frontcontrol.PoolService/CreatePool",
		"/frontcontrol.PoolService/DeletePool",
		"/frontcontrol.PoolService/AddStudents",
		"/frontcontrol.PoolService/DeleteStudent",
		"/frontcontrol.AuthService/CreateUser",
	}
	// Le rôle student (et un rôle vide/inconnu) doit être refusé sur chaque méthode sensible.
	for _, m := range sensitive {
		for _, r := range []string{RoleStudent, ""} {
			if !methodDeniedForRole(m, r) {
				t.Errorf("méthode sensible %q devrait être REFUSÉE au rôle %q", m, r)
			}
		}
		// Les rôles de confiance passent.
		for _, r := range []string{RoleProf, RoleTA, RoleAdmin, RoleChercheur} {
			if methodDeniedForRole(m, r) {
				t.Errorf("méthode sensible %q devrait être AUTORISÉE au rôle %q", m, r)
			}
		}
	}

	// Les méthodes NON sensibles passent pour tout le monde, y compris student.
	nonSensitive := []string{
		"/frontcontrol.AttribVMService/AttribVMinPool",
		"/frontcontrol.AuthService/AuthenticateUser",
		"/frontcontrol.GatherDataService/GetAllImages",
		"/frontcontrol.PoolService/GetAllPools",
		"/frontcontrol.UserService/UpdateDataUser",
	}
	for _, m := range nonSensitive {
		for _, r := range []string{RoleStudent, RoleProf, RoleAdmin, ""} {
			if methodDeniedForRole(m, r) {
				t.Errorf("méthode non sensible %q ne devrait jamais être refusée (rôle %q)", m, r)
			}
		}
	}
}

// TestGRPCWebOriginAllowed vérifie l'allowlist CORS gRPC-Web : non configurée → tout passe
// (compat) ; configurée → seules les origines listées sont acceptées. On appelle directement
// loadGRPCWebOrigins() (et non grpcWebOriginAllowed) pour contourner le sync.Once entre cas.
func TestGRPCWebOriginAllowed(t *testing.T) {
	t.Run("non configuré → tout autorisé", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", "")
		loadGRPCWebOrigins()
		if !originInAllowlist("https://n-importe-quoi.example") {
			t.Error("origine devrait être autorisée quand CORS_ALLOWED_ORIGINS est vide")
		}
	})

	t.Run("allowlist stricte", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.exemple.fr, https://10.202.3.109")
		loadGRPCWebOrigins()
		if !originInAllowlist("https://app.exemple.fr") {
			t.Error("origine listée devrait être autorisée")
		}
		if !originInAllowlist("HTTPS://APP.EXEMPLE.FR") {
			t.Error("la comparaison d'origine doit être insensible à la casse")
		}
		if originInAllowlist("https://evil.example") {
			t.Error("origine non listée devrait être REFUSÉE")
		}
	})

	// Restaure l'état par défaut pour ne pas polluer d'autres tests du package.
	t.Cleanup(func() { grpcWebOrigins = map[string]bool{} })
}

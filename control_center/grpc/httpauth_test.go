package grpc

import "testing"

// TestIsAdminPath vérifie le périmètre des routes réservées à l'équipe pédagogique. En
// particulier /api/inventory (qui exposait toutes les VMs/IP/élèves/URLs) DOIT être gaté —
// test de régression du correctif #2.
func TestIsAdminPath(t *testing.T) {
	staffOnly := []string{
		"/api/inventory",
		"/api/inventory/anything",
		"/api/nbgrader/grades",
		"/api/nbgrader/export-csv",
		"/api/moodle/push-grades",
		"/api/vm/rebuild", // rebuild/resize restent staff-only (vm/action est ouvert au chercheur)
		"/api/pool/label",
		"/api/github/students",
	}
	for _, p := range staffOnly {
		if !isAdminPath(p) {
			t.Errorf("route %q devrait être réservée au staff (isAdminPath=true)", p)
		}
	}

	// Routes accessibles aux étudiants authentifiés → ne doivent PAS être gatées staff.
	nonStaff := []string{
		"/api/me",
		"/api/app-status",
		"/api/guac-url",
		"/api/nbgrader/submit",
		"/api/proxy-session",
		"/api/announcement",
	}
	for _, p := range nonStaff {
		if isAdminPath(p) {
			t.Errorf("route %q ne devrait pas être réservée au staff", p)
		}
	}
}

// TestIsResearcherPath vérifie le périmètre self-service chercheur : jobs/usage/storage/pricing +
// /api/vm/action y sont ; l'éducation (nbgrader/moodle), l'inventaire et vm/rebuild n'y sont PAS.
func TestIsResearcherPath(t *testing.T) {
	researcher := []string{
		"/api/jobs", "/api/jobs/sweep", "/api/jobs/cancel",
		"/api/usage", "/api/storage", "/api/pricing", "/api/vm/action",
	}
	for _, p := range researcher {
		if !isResearcherPath(p) {
			t.Errorf("route %q devrait être accessible au chercheur (isResearcherPath=true)", p)
		}
	}
	notResearcher := []string{
		"/api/inventory", "/api/nbgrader/grades", "/api/moodle/import",
		"/api/vm/rebuild", "/api/vm/resize", "/api/pool/progress", "/api/admin/users",
	}
	for _, p := range notResearcher {
		if isResearcherPath(p) {
			t.Errorf("route %q ne devrait PAS être un chemin chercheur", p)
		}
	}
	// Un chemin chercheur ne doit pas être bloqué par le gate staff (sauf s'il est aussi admin).
	// /api/vm/action est researcher ET matche le préfixe admin /api/vm/ → le middleware vérifie
	// isResearcherPath EN PREMIER, donc on documente ici les deux faits.
	if !isResearcherPath("/api/vm/action") {
		t.Error("/api/vm/action doit être un chemin chercheur")
	}
	if !isAdminPath("/api/vm/action") {
		t.Error("/api/vm/action matche aussi le préfixe admin /api/vm/ (ordre du middleware important)")
	}
}

// TestPublicHTTPPaths vérifie que seules les routes réellement publiques (login, callbacks,
// statut, métriques) sont exemptées d'authentification — et qu'aucune route sensible ne l'est.
func TestPublicHTTPPaths(t *testing.T) {
	mustBePublic := []string{
		"/api/moodle/login", "/api/github/login", "/auth/github/callback",
		"/vm-registrar", "/metrics", "/api/announcement",
	}
	for _, p := range mustBePublic {
		if !publicHTTPPaths[p] {
			t.Errorf("route %q devrait être publique", p)
		}
	}

	// Aucune route d'inventaire / d'admin / de notation ne doit être publique.
	mustNotBePublic := []string{
		"/api/inventory", "/api/admin/users", "/api/nbgrader/grades",
		"/api/vm/start", "/api/proxy-session",
	}
	for _, p := range mustNotBePublic {
		if publicHTTPPaths[p] {
			t.Errorf("route sensible %q ne doit JAMAIS être publique", p)
		}
	}
}

// TestIsStaff vérifie le périmètre des rôles « équipe pédagogique » (sous-jacent à tout le
// gating). chercheur et student ne sont PAS staff.
func TestIsStaff(t *testing.T) {
	for _, r := range []string{RoleAdmin, RoleProf, RoleTA} {
		if !isStaff(r) {
			t.Errorf("%q devrait être staff", r)
		}
	}
	for _, r := range []string{RoleStudent, RoleChercheur, "", "n'importe quoi"} {
		if isStaff(r) {
			t.Errorf("%q ne devrait pas être staff", r)
		}
	}
}

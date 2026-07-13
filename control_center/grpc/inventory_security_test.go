package grpc

import (
	"context"
	"testing"
)

// TestIPBelongsToCaller_Guards couvre les garde-fous d'appartenance (anti-IDOR/SSRF) qui ne
// dépendent pas de la base : une entrée qui n'est pas une IP valide est refusée AVANT toute
// requête (pas de scan d'hôte arbitraire), et un contexte sans identité authentifiée est
// refusé. Les branches staff/élève (qui interrogent la base) sont couvertes par le smoke-test.
func TestIPBelongsToCaller_Guards(t *testing.T) {
	// 1. Entrées non-IP : refusées immédiatement (empêche d'utiliser un hôte/URL arbitraire).
	bad := []string{"", "not-an-ip", "127.0.0.1; rm -rf /", "evil.example", "10.0.0.1:8080", "::gg"}
	for _, ip := range bad {
		if ipBelongsToCaller(context.Background(), ip) {
			t.Errorf("ipBelongsToCaller devrait refuser une entrée non-IP: %q", ip)
		}
	}

	// 2. IP valide mais contexte SANS identité authentifiée → refusé (pas d'accès anonyme).
	if ipBelongsToCaller(context.Background(), "10.202.3.50") {
		t.Error("ipBelongsToCaller devrait refuser un contexte sans identité")
	}
}

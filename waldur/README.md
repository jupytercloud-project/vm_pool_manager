# Waldur — intégration (scaffolding)

Ce dossier **ne contient pas** le déploiement complet de Waldur (il vit dans son dépôt officiel,
versionné et volumineux). Il fournit le **plan + les paramètres d'intégration** à notre OpenStack.
Doc détaillée : [`doc/14-waldur.md`](../doc/14-waldur.md).

## 1. Déployer Waldur (VM dédiée ≥ 8 Go RAM, Docker)

```bash
git clone https://github.com/waldur/waldur-docker-compose.git
cd waldur-docker-compose
cp .env.example .env          # domaine, secrets, SMTP…
docker compose up -d          # migrations initiales : quelques minutes
docker exec -t waldur-mastermind-worker waldur createstaffuser -u admin -p '<motdepasse>' -e admin@polytechnique.edu
docker exec -t waldur-mastermind-worker waldur load_categories vpc vm storage
```

HomePort : `https://<host>/` · API : `/api` · santé : `/health-check`.
Prod : préférer le chart Helm `waldur-helm`.

## 2. Brancher notre OpenStack

1. Créer une **application credential** OpenStack dédiée (compte admin de service), noter son
   `id`/`secret`, le domaine, le projet admin et l'`external_network_id`.
2. Renseigner `.env` (ci-contre) puis créer une **offre Marketplace `OpenStack.Tenant`** dans
   Waldur (UI Admin ou API) avec les `secret_options` — voir `openstack-offering.example.json`.
3. Vérifier la synchro (images/flavors/quotas) puis l'import des ressources existantes.

## 3. SSO (recommandé)

Faire pointer le Keycloak de Waldur et notre Dex sur le **même annuaire LDAP établissement**
pour une identité unique (chercheurs/staff). Voir `doc/02-authentification.md`.

## Fichiers

- `.env.example` — paramètres d'intégration OpenStack + IdP (à copier en `.env`, **gitignoré**).
- `openstack-offering.example.json` — gabarit d'offre `OpenStack.Tenant` (`secret_options`).

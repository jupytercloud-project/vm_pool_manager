# Authentification & connexion

Il y a **deux** mécanismes d'authentification distincts :

| Public | Méthode | Usage |
|--------|---------|-------|
| **Enseignants / admins** | OIDC (Dex + GLAuth) avec PKCE | Accès à toute l'interface d'administration |
| **Étudiants** | OAuth GitHub | Récupération automatique de la clé SSH publique pour accéder à leur VM |

## 1. Connexion enseignant (OIDC / Dex)

Dex (`auth/dex.yaml`) est le fournisseur OIDC ; GLAuth (`auth/glauth.cfg`) est l'annuaire LDAP
des comptes. Le frontend fait un flux **Authorization Code + PKCE**.

```mermaid
sequenceDiagram
    autonumber
    participant U as Enseignant
    participant FE as Frontend (authStore)
    participant DEX as Dex (/dex)
    participant CC as Control Center

    U->>FE: Clique « Se connecter »
    FE->>FE: génère code_verifier + code_challenge (PKCE)
    FE->>DEX: redirige /dex/auth?client_id=cloudpoolmanager&redirect_uri=/auth/callback&code_challenge=…
    DEX->>U: formulaire de login (compte GLAuth)
    U->>DEX: identifiants
    DEX->>FE: redirige /auth/callback?code=…
    FE->>DEX: POST /dex/token (code + code_verifier)
    DEX->>FE: id_token (JWT) + access_token
    FE->>FE: décode le JWT → email, groups → role (admin si groupe 'admins')
    Note over FE: authStore.set({token, accessToken, email, role})
    FE->>CC: appels gRPC avec header Authorization: Bearer <token>
    CC->>CC: oidcmw.UnaryInterceptor valide le JWT (JWKS Dex)
```

**Points clés :**
- Le rôle est déduit des `groups` du JWT : `admins` → `admin`, sinon `student`
  (`frontend/src/lib/store/authStore.ts`).
- Le transport gRPC authentifié injecte `Authorization: Bearer <accessToken>`
  (`frontend/src/lib/grpc/transport.ts`).
- Côté serveur, l'intercepteur OIDC (`control_center/internal/oidc/middleware.go`) valide le
  token sur **toutes** les méthodes gRPC **sauf** la liste `publicMethods`
  (`control_center/grpc/server.go`), qui exempte :
  `AttribVMService/AttribVMinPool`, `AttribVMService/ReturnPoolWithKey`,
  `AuthService/AuthenticateUser`, `AuthService/CreateUser`.
- Échec d'auth → `codes.Unauthenticated` (« missing metadata / authorization header / invalid token »).

**Redirections / config** : Dex et le frontend doivent partager le même hôte canonique
(`+layout.svelte` force `https://<IP>`). Les `redirectURIs` de Dex doivent inclure
`https://<IP>/auth/callback`.

## 2. Connexion étudiant (OAuth GitHub)

L'étudiant n'a pas de compte : il s'authentifie via **GitHub**, ce qui permet de récupérer
automatiquement sa **clé SSH publique** (API publique GitHub) pour accéder à sa VM.

```mermaid
sequenceDiagram
    autonumber
    participant S as Étudiant
    participant FE as Frontend (/student)
    participant CC as Control Center (/api/github/*)
    participant GH as github.com

    S->>FE: Clique « Se connecter avec GitHub »
    Note over FE: lien <a data-sveltekit-reload> → navigation pleine page
    FE->>CC: GET /api/github/login
    CC->>CC: crée un state (CSRF) en base
    CC->>GH: redirige /login/oauth/authorize?client_id=…&scope=read:user&state=…
    GH->>S: autorisation
    GH->>CC: redirige /auth/github/callback?code=…&state=…
    CC->>CC: vérifie le state, supprime-le
    CC->>GH: POST /login/oauth/access_token (code + secret)
    GH->>CC: access_token
    CC->>GH: GET /user → login
    CC->>GH: GET https://github.com/<login>.keys (clés SSH publiques)
    CC->>CC: crée une GitHubSession {login, ssh_keys}
    CC->>FE: redirige /student?github_session=<id>
    FE->>CC: GET /api/github/session?id=<id>
    CC->>FE: { login, keys }
    Note over FE: githubStore.set({login, keys}) — clé SSH récupérée automatiquement
```

**Handlers** : `control_center/grpc/github.go`
(`handleGitHubLogin`, `handleGitHubCallback`, `handleGitHubSession`).

**Pièges connus ⚠️**
- Le lien de login doit porter `data-sveltekit-reload` (`frontend/src/routes/student/+page.svelte`),
  sinon le routeur SPA SvelteKit intercepte le clic et il faut cliquer **plusieurs fois**.
- L'URL de callback enregistrée dans l'**OAuth App GitHub** doit être **exactement**
  `https://<IP>/auth/github/callback` (mêmes schéma/port que `GITHUB_REDIRECT_URL`), sinon
  l'échange de token échoue (`redirect_uri mismatch`).
- Le callback arrive sur Caddy (`/auth/*` → control center `:50055`) : **Caddy doit tourner**.

## Carte des routes d'auth (Caddy)

```mermaid
flowchart LR
    A["/auth/callback"] -->|Vite| FE["Frontend :5173"]
    B["/auth/* (github)"] -->|REST| CC["Control Center :50055"]
    C["/dex/*"] --> DEX["Dex :5556"]
    D["/rpc/* (gRPC-Web)"] --> G["Control Center :50051"]
```
Voir `caddy/Caddyfile.native`.

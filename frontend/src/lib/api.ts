import { get } from 'svelte/store';
import { authStore } from './store/authStore';
import { moodleStudentStore } from './store/moodleStudentStore';
import { githubStore } from './store/githubStore';

// apiFetch ajoute automatiquement l'en-tête d'authentification aux appels REST `/api/*`.
// Le jeton est, dans l'ordre :
//   1. l'accessToken OIDC (profs connectés via Dex → JWT signé), ou
//   2. la session Moodle de l'élève (session_id renvoyé par /api/moodle/login).
// Le backend (httpAuthMiddleware) accepte les deux : il valide d'abord le JWT, sinon
// résout la session côté base. Les routes publiques ignorent simplement l'en-tête.
export function authToken(): string {
  const auth = get(authStore);
  // Pour OIDC, on privilégie l'ID token : il contient de façon fiable les claims
  // `email` et `groups` (rôle admin), contrairement à l'access token. Les deux sont
  // des JWT signés par Dex et validés par le même JWKS.
  if (auth?.token && auth.token.split('.').length === 3) return auth.token;
  if (auth?.accessToken) return auth.accessToken;
  const ms = get(moodleStudentStore);
  if (ms?.session) return ms.session;
  const gh = get(githubStore);
  if (gh?.session) return gh.session;
  return '';
}

export function apiFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  const tok = authToken();
  if (tok && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${tok}`);
  }
  return fetch(input, { ...init, headers });
}

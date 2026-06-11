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

let redirecting = false;

// handleAuthExpired : appelée quand le serveur renvoie 401 alors qu'on avait pourtant
// envoyé un jeton → la session/le token est invalide ou expiré (l'ID token OIDC dure 24 h).
// On nettoie l'état et on renvoie vers l'écran de connexion (au lieu de laisser une page
// « connectée » mais cassée).
function handleAuthExpired() {
  if (typeof window === 'undefined' || redirecting) return;
  redirecting = true;
  authStore.set(null);
  moodleStudentStore.set(null);
  githubStore.set(null);
  const p = window.location.pathname;
  if (p !== '/' && p !== '/login') window.location.href = '/';
}

export async function apiFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  const tok = authToken();
  if (tok && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${tok}`);
  }
  const res = await fetch(input, { ...init, headers });
  // 401 alors qu'on avait un jeton = session expirée → déconnexion propre.
  if (res.status === 401 && tok) handleAuthExpired();
  return res;
}

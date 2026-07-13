import { writable } from 'svelte/store';
import { goto } from '$app/navigation';
import { resetAll } from './serverpoolStore';

interface AuthData {
  token: string;       // ID token (JWT from Dex)
  accessToken: string; // access token for gRPC Bearer header
  email: string;
  role: string;
  name: string;
}

function createAuthStore() {
  let initial: AuthData | null = null;

  if (typeof window !== 'undefined') {
    const saved = localStorage.getItem('authData');
    if (saved) {
      try {
        const data: AuthData = JSON.parse(saved);
        if (data.token) initial = data;
      } catch { /* ignore */ }
    }
  }

  const store = writable<AuthData | null>(initial);

  store.subscribe((auth) => {
    if (typeof window === 'undefined') return;
    if (auth) localStorage.setItem('authData', JSON.stringify(auth));
    else localStorage.removeItem('authData');
  });

  return store;
}

export const authStore = createAuthStore();

// ---- OIDC PKCE helpers ----

function randomString(len = 43): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~';
  const arr = new Uint8Array(len);
  crypto.getRandomValues(arr);
  return Array.from(arr).map(b => chars[b % chars.length]).join('');
}

function dexBase(): string {
  if (typeof window === 'undefined') return 'http://localhost:5556/dex';
  // Use Caddy proxy so Dex is always reachable regardless of which IP/port the user connects from
  return `${window.location.origin}/dex`;
}

// base64url sans padding (format attendu pour un code_challenge PKCE S256).
function base64urlFromBytes(bytes: Uint8Array): string {
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// SÉCURITÉ : on dérive le code_challenge en S256 (SHA-256 du verifier) dès que crypto.subtle
// est disponible (contexte sécurisé : HTTPS ou localhost) — la prod est servie en HTTPS via
// Caddy. Repli sur 'plain' UNIQUEMENT si crypto.subtle est absent (dev en HTTP simple).
// 'plain' transmet le verifier en clair dans l'URL d'autorisation (interceptable) ; S256 ne
// transmet que son empreinte → l'interception de l'URL ne permet pas de rejouer le code.
async function deriveCodeChallenge(verifier: string): Promise<{ challenge: string; method: string }> {
  if (typeof crypto !== 'undefined' && crypto.subtle) {
    try {
      const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
      return { challenge: base64urlFromBytes(new Uint8Array(digest)), method: 'S256' };
    } catch {
      /* contexte non sécurisé → repli plain */
    }
  }
  return { challenge: verifier, method: 'plain' };
}

export async function startOIDCLogin() {
  const state = randomString();
  const codeVerifier = randomString();

  sessionStorage.setItem('oidc_state', state);
  sessionStorage.setItem('oidc_code_verifier', codeVerifier);

  const redirectUri = window.location.origin + '/auth/callback';
  const { challenge, method } = await deriveCodeChallenge(codeVerifier);

  const url = new URL(dexBase() + '/auth');
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('client_id', 'cloudpoolmanager');
  url.searchParams.set('redirect_uri', redirectUri);
  url.searchParams.set('scope', 'openid email profile groups offline_access');
  url.searchParams.set('state', state);
  url.searchParams.set('code_challenge', challenge);
  url.searchParams.set('code_challenge_method', method);

  window.location.href = url.toString();
}

function parseJWT(token: string): Record<string, unknown> {
  try {
    const payload = token.split('.')[1];
    return JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')));
  } catch {
    return {};
  }
}

export async function loginOIDC(idToken: string, accessToken: string) {
  const claims = parseJWT(idToken);
  const email = (claims.email as string) ?? '';
  const name = (claims.name as string) ?? email;
  const groups = (claims.groups as string[]) ?? [];
  const role = groups.includes('admins') ? 'admin' : 'student';

  authStore.set({ token: idToken, accessToken, email, role, name });
}

export function login(token: string, email: string) {
  const parts = token.split(':');
  const role = parts.length >= 1 ? parts[0] : 'student';
  authStore.set({ token, accessToken: token, email, role, name: email });
}

export function logout() {
  authStore.set(null);
  resetAll();
  window.location.href = '/';
}

// Legacy tryLogin (kept for backward compat during transition)
export async function tryLogin(email: string, password: string) {
  if (!email || !password) return { success: false, error: 'Champs non rempli' };
  try {
    const { authenticateUser } = await import('$lib/grpc/authService/authService');
    const result = await authenticateUser(email, password);
    if (!result.success || !result.token) return { success: false, error: 'Erreur lors de la connexion' };
    login(result.token, email);
    return { success: true };
  } catch {
    return { success: false, error: 'Erreur backend' };
  }
}

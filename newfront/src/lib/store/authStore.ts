import { writable } from 'svelte/store';
import { jwtDecode } from 'jwt-decode';
import { goto } from '$app/navigation';
import { authenticateUser } from '$lib/grpc/authService/authService';
import { subscribeUserUpdate } from '$lib/grpc/userUpdateService/userService';
import { loadAll } from './serverpoolStore';

interface JwtPayload {
  exp: number;
  [key: string]: any;
}

interface AuthData {
  token: string;
  email: string;
}

// Store pour le token JWT
export const authStore = writable<AuthData | null>(null);

let stopStream: (() => void) | null = null;

authStore.subscribe((auth) => {
  if (!auth) {
    if (stopStream) {
      stopStream();
      stopStream = null;
      console.log("Stream arreté");
    }
    return;
  }

  console.log("Stream start for ", auth.email);
  let active = true;
  subscribeUserUpdate(auth.email, (data) => {
    if (!active) return;
    console.log("Update user :", data);
    //todo
  });
  stopStream = () => {
    active = false;
  };
});

// Vérifie si un token JWT est valide
function isTokenValid(token: string): boolean {
  try {
    const decoded = jwtDecode<JwtPayload>(token);
    return decoded.exp > Date.now() / 1000;
  } catch {
    return false;
  }
}

// Initialisation côté client
if (typeof window !== 'undefined') {
  const saved = localStorage.getItem('authData');
  if (saved) {
    const data: AuthData = JSON.parse(saved);
    if (data.token && isTokenValid(data.token)) {
      authStore.set(data);
    } else {
      localStorage.removeItem('authData');
      authStore.set(null);
    }
  }
}

export function login(token: string, email: string) {
  const data: AuthData = { token, email };
  localStorage.setItem('authData', JSON.stringify(data));
  authStore.set(data);
  loadAll(email);
}

export function logout() {
  localStorage.removeItem('authData');
  authStore.set(null);
  goto("/");
}

export async function tryLogin(email: string, password: string) {
  if (!email || !password) {
    return { success: false, error: 'Champs non rempli' };
  }

  try {
    const result = await authenticateUser(email, password);

    if (!result.success || !result.token) {
      return { success: false, error: 'Erreur lors de la connexion' };
    }

    login(result.token, email);

    return { success: true };
  } catch (err) {
    console.error(err);
    return { success: false, error: 'Erreur backend' };
  }
}

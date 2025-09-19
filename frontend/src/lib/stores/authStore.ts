import { writable } from 'svelte/store';
import {jwtDecode} from 'jwt-decode';
import { goto } from '$app/navigation';

interface JwtPayload {
  exp: number;
  [key: string]: any;
}

export const authStore = writable<string | null>(null);

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
  const token = localStorage.getItem('authToken');
  if (token && isTokenValid(token)) {
    authStore.set(token);
  } else {
    localStorage.removeItem('authToken');
    authStore.set(null);
  }
}

export function login(token: string) {
  localStorage.setItem('authToken', token);
  authStore.set(token);
}

export function logout() {
  localStorage.removeItem('authToken');
  authStore.set(null);
  goto("/");
}

/**
 * Essaie de se connecter avec email/password
 * Retourne un objet { success: boolean, error?: string }
 */
export async function tryLogin(email: string, password: string) {
  if (!email || !password) {
    return { success: false, error: 'Champs non rempli' };
  }

  try {
    const response = await fetch('http://localhost:8080/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password })
    });

    if (!response.ok) {
      return { success: false, error: 'Erreur lors de la connexion' };
    }

    const result = await response.json();
    const token = result.token;

    login(token);
    return { success: true };
  } catch (err) {
    console.error(err);
    return { success: false, error: 'Erreur backend' };
  }
}


import { writable } from 'svelte/store';
import { jwtDecode } from 'jwt-decode';
import { goto } from '$app/navigation';
// import { connectWebSocket, disconnectWebSocket } from '$lib/websocket';
// import { serverpoolStore } from '$lib/stores/fetchinit';
import { authenticateUser } from '$lib/grpc/authService/authService';

interface JwtPayload {
  exp: number;
  [key: string]: any;
}

// Store pour le token JWT
export const authStore = writable<string | null>(null);

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
  const token = localStorage.getItem('authToken');
  if (token && isTokenValid(token)) {
    authStore.set(token);
    // connectWebSocket(token);
    // serverpoolStore.fetchInitData();
  } else {
    localStorage.removeItem('authToken');
    authStore.set(null);
  }
}

// Fonction pour stocker un token et initialiser websocket + store
export function login(token: string) {
  localStorage.setItem('authToken', token);
  authStore.set(token);
//   connectWebSocket(token);
//   serverpoolStore.fetchInitData();
}

// Fonction pour se déconnecter
export function logout() {
  localStorage.removeItem('authToken');
  authStore.set(null);
//   disconnectWebSocket();
  goto("/");
}

// Fonction pour tenter une connexion avec gRPC-Web
export async function tryLogin(email: string, password: string) {
  if (!email || !password) {
    return { success: false, error: 'Champs non rempli' };
  }

  try {
    // Appel gRPC-Web
    const result = await authenticateUser(email, password);

    if (!result.success || !result.token) {
      return { success: false, error: 'Erreur lors de la connexion' };
    }

    login(result.token);
    return { success: true };
  } catch (err) {
    console.error(err);
    return { success: false, error: 'Erreur backend' };
  }
}

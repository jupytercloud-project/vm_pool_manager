import { writable } from 'svelte/store';

export const githubStore = writable<{ login: string; keys: string[]; session?: string } | null>(null);

export function disconnectGitHub() {
  githubStore.set(null);
  if (typeof window !== 'undefined') window.location.href = '/student';
}

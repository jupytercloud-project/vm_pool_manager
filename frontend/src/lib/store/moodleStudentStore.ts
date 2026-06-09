import { writable } from 'svelte/store';

interface MoodleStudent { email: string; fullname: string }

function create() {
  let initial: MoodleStudent | null = null;
  if (typeof window !== 'undefined') {
    const s = localStorage.getItem('moodleStudent');
    if (s) { try { initial = JSON.parse(s); } catch { /* ignore */ } }
  }
  const store = writable<MoodleStudent | null>(initial);
  store.subscribe((v) => {
    if (typeof window === 'undefined') return;
    if (v) localStorage.setItem('moodleStudent', JSON.stringify(v));
    else localStorage.removeItem('moodleStudent');
  });
  return store;
}

export const moodleStudentStore = create();

export function disconnectMoodleStudent() {
  moodleStudentStore.set(null);
  if (typeof window !== 'undefined') window.location.href = '/student';
}

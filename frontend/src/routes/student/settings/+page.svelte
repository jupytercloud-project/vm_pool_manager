<script lang="ts">
  import { onMount } from 'svelte';
  import { browser } from '$app/environment';
  import { goto } from '$app/navigation';
  import { githubStore, disconnectGitHub } from '$lib/store/githubStore';
  import { moodleStudentStore, disconnectMoodleStudent } from '$lib/store/moodleStudentStore';
  import { darkMode, reduceMotion } from '$lib/store/uiStore';

  let provider = $derived($githubStore ? 'github' : $moodleStudentStore ? 'moodle' : null);
  let displayName = $derived(
    $githubStore?.login ?? $moodleStudentStore?.fullname ?? $moodleStudentStore?.email ?? ''
  );
  let email = $derived($moodleStudentStore?.email ?? '');
  let initial = $derived((displayName || '?').charAt(0).toUpperCase());

  // Ajout de clé SSH (élèves Moodle)
  let sshKey = $state('');
  let addingKey = $state(false);
  let keyMsg = $state('');

  onMount(() => {
    if (!browser) return;
    if (!$githubStore && !$moodleStudentStore) goto('/student');
  });

  async function addKey() {
    if (!sshKey.trim() || !email) return;
    addingKey = true; keyMsg = '';
    try {
      const r = await fetch('/api/moodle/ssh-key', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, ssh_key: sshKey.trim() }),
      });
      const d = await r.json();
      if (!d.success) { keyMsg = 'Erreur : ' + (d.error ?? 'échec'); return; }
      keyMsg = 'Clé SSH enregistrée ✓'; sshKey = '';
    } catch { keyMsg = "Erreur lors de l'enregistrement."; }
    finally { addingKey = false; }
  }

  function disconnect() {
    if ($githubStore) disconnectGitHub();
    else disconnectMoodleStudent();
  }
</script>

<svelte:head><title>Mon compte — CloudPoolManager</title></svelte:head>

<div class="max-w-xl mx-auto py-10 space-y-6 animate-fade-up">

  <!-- Hero identité -->
  <div class="card card-interactive p-6 flex items-center gap-4">
    <div class="w-16 h-16 rounded-full flex items-center justify-center text-2xl font-bold text-white shrink-0
      shadow-md {provider === 'github' ? 'bg-neutral-900' : 'bg-[#f98012]'}">
      {initial}
    </div>
    <div class="min-w-0 flex-1">
      <h1 class="text-xl font-bold text-neutral-900 dark:text-white truncate">{displayName || 'Étudiant'}</h1>
      <p class="text-sm text-neutral-500 flex items-center gap-1.5 mt-0.5">
        {#if provider === 'github'}
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/></svg>
          Connecté via GitHub
        {:else}
          <svg class="w-4 h-4 text-[#f98012]" fill="currentColor" viewBox="0 0 24 24"><path d="M12 3 1 9l4 2.18v6L12 21l7-3.82v-6l2-1.09V17h2V9L12 3z"/></svg>
          Connecté via Moodle
        {/if}
      </p>
    </div>
    <button onclick={disconnect} class="btn btn-secondary text-xs shrink-0">Déconnexion</button>
  </div>

  <!-- Préférences -->
  <div class="card p-5">
    <h2 class="text-sm font-bold text-neutral-800 dark:text-neutral-200 mb-1">Préférences</h2>
    <div class="divide-y divide-black/5 dark:divide-white/5">
      <div class="flex items-center justify-between py-3.5">
        <div class="flex items-center gap-3">
          <svg class="w-5 h-5 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/></svg>
          <div>
            <p class="text-sm font-medium text-neutral-800 dark:text-neutral-200">Apparence</p>
            <p class="text-xs text-neutral-400">{$darkMode ? 'Mode sombre' : 'Mode clair'}</p>
          </div>
        </div>
        <button onclick={() => darkMode.update(v => !v)} role="switch" aria-checked={$darkMode} aria-label="Mode sombre"
          class="relative w-11 h-6 rounded-full transition-colors shrink-0 {$darkMode ? 'bg-primary-600' : 'bg-neutral-300 dark:bg-neutral-600'}">
          <span class="absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow transition-transform {$darkMode ? 'translate-x-5' : ''}"></span>
        </button>
      </div>
      <div class="flex items-center justify-between py-3.5">
        <div class="flex items-center gap-3">
          <svg class="w-5 h-5 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
          <div>
            <p class="text-sm font-medium text-neutral-800 dark:text-neutral-200">Réduire les animations</p>
            <p class="text-xs text-neutral-400">Interface plus sobre</p>
          </div>
        </div>
        <button onclick={() => reduceMotion.update(v => !v)} role="switch" aria-checked={$reduceMotion} aria-label="Réduire les animations"
          class="relative w-11 h-6 rounded-full transition-colors shrink-0 {$reduceMotion ? 'bg-primary-600' : 'bg-neutral-300 dark:bg-neutral-600'}">
          <span class="absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow transition-transform {$reduceMotion ? 'translate-x-5' : ''}"></span>
        </button>
      </div>
    </div>
  </div>

  <!-- Clé SSH (Moodle uniquement) -->
  {#if provider === 'moodle'}
    <div class="card p-5">
      <h2 class="text-sm font-bold text-neutral-800 dark:text-neutral-200 mb-1">Clé SSH (optionnel)</h2>
      <p class="text-xs text-neutral-400 mb-3">Connecté via Moodle, tu accèdes à ta VM par JupyterLab et le terminal web — aucune clé n'est requise. Ajoute-en une seulement si tu veux te connecter en SSH direct.</p>
      <textarea bind:value={sshKey} rows="3" placeholder="ssh-ed25519 AAAA..." class="field font-mono text-xs resize-none"></textarea>
      <div class="flex items-center justify-between mt-2">
        {#if keyMsg}<p class="text-xs {keyMsg.startsWith('Erreur') ? 'text-red-600' : 'text-green-600'}">{keyMsg}</p>{:else}<span></span>{/if}
        <button onclick={addKey} disabled={addingKey || !sshKey.trim()} class="btn btn-primary text-sm">
          {#if addingKey}<span class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>{/if}
          Enregistrer
        </button>
      </div>
    </div>
  {/if}

  <a href="/student" class="btn btn-secondary text-sm w-full">← Retour à mes cours</a>
</div>

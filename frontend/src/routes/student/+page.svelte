<script lang="ts">
  import { returnPoolsWithKey, attribVMinPool } from "$lib/grpc/attribVMService/attribVMService";
  import { githubStore, disconnectGitHub } from '$lib/store/githubStore';
  import { moodleStudentStore, disconnectMoodleStudent } from '$lib/store/moodleStudentStore';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';

  import { onMount } from 'svelte';
  import { get } from 'svelte/store';

  let sshkey = $state("");
  let availablePools: { pool_id: string; user_id: string }[] = $state([]);
  let selectedPool: { pool_id: string; user_id: string } | null = $state(null);
  let vmIp = $state("");
  let vmUser = $state("");
  let vmAppPort = $state(0);
  let guacUrl = $state("");
  let loading = $state(false);
  let errorMsg = $state("");
  let assignError = $state("");
  let noCoursFound = $state(false);
  let copied = $state(false);
  let appReady = $state(false);
  let probing = $state(false);
  let probeInterval: ReturnType<typeof setInterval> | null = null;

  let githubLoading = $state(false);

  // Moodle login state
  let moodleConfigured = $state(false);
  let moodleEmail = $state("");
  let showMoodleForm = $state(false);
  let moodleUser = $state("");
  let moodlePass = $state("");
  let moodleLoading = $state(false);
  // Ajout optionnel d'une clé SSH (pour les élèves Moodle qui veulent du SSH direct)
  let showAddKey = $state(false);
  let sshKeyInput = $state("");
  let addingKey = $state(false);
  let addKeyMsg = $state("");

  let githubLogin = $derived($githubStore?.login ?? null);
  let githubKeys = $derived($githubStore?.keys ?? []);

  onMount(async () => {
    try {
      const sr = await fetch('/api/moodle/status');
      if (sr.ok) moodleConfigured = !!(await sr.json()).configured;
    } catch { /* ignore */ }

    // Réhydrate une session Moodle persistée (rester connecté entre les pages).
    const ms = get(moodleStudentStore);
    if (ms?.email) {
      moodleEmail = ms.email;
      await refreshMoodlePools();
    }

    const params = new URLSearchParams(window.location.search);
    const sessionId = params.get('github_session');
    if (sessionId) {
      githubLoading = true;
      try {
        const res = await fetch(`/api/github/session?id=${encodeURIComponent(sessionId)}`);
        if (res.ok) {
          const data = await res.json();
          githubStore.set({ login: data.login, keys: data.keys ?? [] });
          if ((data.keys ?? []).length === 1) sshkey = data.keys[0];
        }
      } catch { /* ignore */ } finally { githubLoading = false; }
      window.history.replaceState({}, '', '/');
    }
  });

  function startProbing(ip: string, port: number) {
    appReady = false;
    probing = true;
    probeInterval = setInterval(async () => {
      try {
        const res = await fetch(`/api/app-status?ip=${encodeURIComponent(ip)}&port=${port}`);
        const data = await res.json();
        if (data.ready) {
          appReady = true;
          probing = false;
          if (probeInterval) { clearInterval(probeInterval); probeInterval = null; }
        }
      } catch { /* keep trying */ }
    }, 3000);
  }

  let submitting = $state(false);
  let submitStatus = $state("");

  // Confirmation modal state
  let confirmState = $state({
    show: false,
    title: '',
    message: '',
    onConfirm: () => {}
  });

  async function executeSubmit() {
    if (!selectedPool || !vmIp) return;
    submitting = true;
    submitStatus = "";
    try {
      const res = await fetch(`/api/nbgrader/submit?pool_id=${encodeURIComponent(selectedPool.pool_id)}&user_id=${encodeURIComponent(selectedPool.user_id)}&student_ip=${encodeURIComponent(vmIp)}`, {
        method: "POST"
      });
      if (!res.ok) throw new Error("Erreur serveur: " + await res.text());
      submitStatus = "Travaux soumis avec succès !";
    } catch (e: any) {
      submitStatus = "Erreur: " + e.message;
    } finally {
      submitting = false;
    }
  }

  function submitWork() {
    confirmState = {
      show: true,
      title: 'Soumettre',
      message: 'Êtes-vous sûr de vouloir soumettre vos travaux ? Cette action enregistrera une copie en lecture seule de vos fichiers actuels pour l\'évaluation.',
      onConfirm: executeSubmit
    };
  }

  function fallbackCopy(text: string) {
    const el = document.createElement('textarea');
    el.value = text;
    el.style.position = 'fixed';
    el.style.opacity = '0';
    document.body.appendChild(el);
    el.select();
    document.execCommand('copy');
    document.body.removeChild(el);
  }

  function copyCmd() {
    const text = `ssh ${vmUser}@${vmIp}`;
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text).catch(() => fallbackCopy(text));
    } else {
      fallbackCopy(text);
    }
    copied = true;
    setTimeout(() => copied = false, 2000);
  }

  async function handleSSHKey() {
    if (!sshkey.trim()) return;
    loading = true; errorMsg = ""; noCoursFound = false; availablePools = []; selectedPool = null; vmIp = "";
    try {
      availablePools = await returnPoolsWithKey(sshkey);
    } catch { /* ignore stream-close errors when results were already collected */ }
    finally { loading = false; }
    if (availablePools.length === 0) noCoursFound = true;
  }

  async function refreshMoodlePools() {
    if (!moodleEmail) return;
    noCoursFound = false;
    try {
      const pr = await fetch(`/api/moodle/my-pools?email=${encodeURIComponent(moodleEmail)}`);
      const pd = await pr.json().catch(() => ({ pools: [] }));
      availablePools = pd.pools ?? [];
      if (!availablePools.length) noCoursFound = true;
    } catch { /* ignore */ }
  }

  async function loginMoodle() {
    if (!moodleUser.trim() || !moodlePass) return;
    moodleLoading = true; errorMsg = ""; noCoursFound = false;
    availablePools = []; selectedPool = null; vmIp = "";
    try {
      const r = await fetch('/api/moodle/login', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: moodleUser.trim(), password: moodlePass }),
      });
      if (!r.ok) { errorMsg = "Identifiants Moodle invalides."; return; }
      const data = await r.json();
      moodleEmail = data.email ?? "";
      moodlePass = "";
      moodleStudentStore.set({ email: moodleEmail, fullname: data.fullname ?? "" });
      await refreshMoodlePools();
    } catch { errorMsg = "Erreur de connexion Moodle."; }
    finally { moodleLoading = false; }
  }

  function disconnectMoodle() {
    moodleEmail = ""; showMoodleForm = false; moodleUser = ""; moodlePass = "";
    availablePools = []; noCoursFound = false;
    showAddKey = false; sshKeyInput = ""; addKeyMsg = "";
    moodleStudentStore.set(null);
  }

  async function addMoodleSSHKey() {
    if (!sshKeyInput.trim() || !moodleEmail) return;
    addingKey = true; addKeyMsg = "";
    try {
      const r = await fetch('/api/moodle/ssh-key', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: moodleEmail, ssh_key: sshKeyInput.trim() }),
      });
      const d = await r.json();
      if (!d.success) { addKeyMsg = "Erreur : " + (d.error ?? "échec"); return; }
      addKeyMsg = "Clé SSH enregistrée.";
      sshKeyInput = ""; showAddKey = false;
    } catch { addKeyMsg = "Erreur lors de l'enregistrement."; }
    finally { addingKey = false; }
  }

  function computeUsername(poolId: string): string {
    let name = ("student_" + poolId).split("@")[0].toLowerCase();
    name = name.replace(/[^a-z0-9_.-]/g, "");
    if (name.length > 32) name = name.substring(0, 32);
    return name;
  }

  async function assignVM(pool: { pool_id: string; user_id: string }) {
    selectedPool = pool; loading = true; errorMsg = ""; assignError = ""; vmIp = ""; vmUser = ""; vmAppPort = 0; guacUrl = "";
    appReady = false; probing = false;
    if (probeInterval) { clearInterval(probeInterval); probeInterval = null; }
    try {
      let ip = "", port = 0, user = "";
      if (moodleEmail) {
        // Attribution par identité Moodle, sans clé SSH (accès navigateur + Guacamole).
        const r = await fetch('/api/moodle/attrib-vm', {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ pool_id: pool.pool_id, user_id: pool.user_id, email: moodleEmail }),
        });
        const d = await r.json();
        if (!d.success) { assignError = d.error || "Erreur lors de l'attribution."; return; }
        ip = d.ip; port = d.app_port ?? 0; user = "";
      } else {
        const result = await attribVMinPool(pool.pool_id, pool.user_id, sshkey);
        ip = result.ip; user = result.username || computeUsername(pool.pool_id); port = result.appPort ?? 0;
      }
      vmIp = ip;
      vmUser = user;
      vmAppPort = port;
      fetch(`/api/guac-url?ip=${encodeURIComponent(ip)}`)
        .then(r => r.json())
        .then(data => { if (data.url) guacUrl = data.url; })
        .catch(() => {});
      if (vmAppPort > 0) startProbing(ip, vmAppPort);
    } catch (err: any) {
      assignError = err?.message || "Erreur lors de l'attribution de la VM.";
    } finally { loading = false; }
  }
</script>

<svelte:head>
  <title>CloudPoolManager — Portail Étudiant</title>
</svelte:head>

<div class="max-w-lg mx-auto py-10 animate-fade-up">

  <ConfirmModal
    bind:show={confirmState.show}
    title={confirmState.title}
    message={confirmState.message}
    onConfirm={confirmState.onConfirm}
  />

  {#if !vmIp}
    <div class="mb-8">
      <h1 class="text-3xl font-bold text-primary-800 mb-2" style="font-family: 'Source Sans 3', sans-serif; letter-spacing: -0.01em;">
        Portail étudiant
      </h1>
      <p class="text-sm text-neutral-500 leading-relaxed">
        Collez votre clé SSH publique pour accéder à votre machine virtuelle de travaux pratiques.
      </p>
    </div>

    <div class="card p-6 space-y-5">

      {#if moodleEmail}
        <!-- Moodle connected banner -->
        <div class="flex items-center gap-3 px-3 py-2.5 rounded bg-blue-50 dark:bg-[#0a1a2e] border border-blue-200 dark:border-[#1e3a5f]">
          <svg class="w-4 h-4 text-blue-600 shrink-0" fill="currentColor" viewBox="0 0 24 24"><path d="M12 3 1 9l4 2.18v6L12 21l7-3.82v-6l2-1.09V17h2V9L12 3zm6.82 6L12 12.72 5.18 9 12 5.28 18.82 9zM17 15.99l-5 2.73-5-2.73v-3.72L12 15l5-2.73v3.72z"/></svg>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-semibold text-blue-800 dark:text-blue-400">Connecté via Moodle — <span class="font-mono">{moodleEmail}</span></p>
            <p class="text-xs text-blue-600 mt-0.5">Aucune clé SSH requise : accès via JupyterLab et le terminal web.</p>
          </div>
          <button onclick={disconnectMoodle} class="text-blue-500 hover:text-blue-700 text-xs">Déconnecter</button>
        </div>

        <div class="flex items-center gap-2">
          <button onclick={refreshMoodlePools} class="btn btn-secondary text-xs gap-1.5 flex-1">
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
            Rafraîchir mes cours
          </button>
          <button onclick={() => showAddKey = !showAddKey} class="btn btn-secondary text-xs flex-1">
            {showAddKey ? 'Annuler' : 'Ajouter une clé SSH'}
          </button>
        </div>

        {#if showAddKey}
          <div class="space-y-2 p-3 rounded border border-neutral-200 bg-neutral-50">
            <p class="text-xs text-neutral-500">Optionnel — pour vous connecter aussi en SSH direct. Sinon, JupyterLab et le terminal web suffisent.</p>
            <textarea bind:value={sshKeyInput} rows="3" placeholder="ssh-ed25519 AAAA..." class="field font-mono text-xs resize-none"></textarea>
            <button onclick={addMoodleSSHKey} disabled={addingKey || !sshKeyInput.trim()} class="btn btn-primary text-xs w-full">
              {#if addingKey}<span class="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>{/if}
              Enregistrer la clé
            </button>
          </div>
        {/if}
        {#if addKeyMsg}
          <p class="text-xs {addKeyMsg.startsWith('Erreur') ? 'text-red-600' : 'text-green-600'}">{addKeyMsg}</p>
        {/if}
      {:else}
      {#if githubLogin}
        <!-- GitHub connected banner -->
        <div class="flex items-center gap-3 px-3 py-2.5 rounded bg-green-50 dark:bg-[#0a2018] border border-green-200 dark:border-[#14532d]">
          <svg class="w-4 h-4 text-green-600 shrink-0" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/>
          </svg>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-semibold text-green-800 dark:text-green-400">Connecté via GitHub — <span class="font-mono">{githubLogin}</span></p>
            {#if githubKeys.length === 0}
              <p class="text-xs text-green-600 mt-0.5">Aucune clé SSH sur ce compte. Entrez-la manuellement.</p>
            {:else if githubKeys.length === 1}
              <p class="text-xs text-green-600 mt-0.5">Clé SSH récupérée automatiquement.</p>
            {/if}
          </div>
          <button onclick={() => { disconnectGitHub(); sshkey = ''; }} class="text-green-500 hover:text-green-700 text-xs">Déconnecter</button>
        </div>

        {#if githubKeys.length > 1}
          <div>
            <label class="section-label mb-2 block">Choisir une clé SSH</label>
            <div class="space-y-1.5">
              {#each githubKeys as key, i}
                <button
                  onclick={() => sshkey = key}
                  class="w-full text-left px-3 py-2 rounded border text-xs font-mono truncate transition-colors
                    {sshkey === key ? 'border-primary-400 bg-primary-50 text-primary-800' : 'border-neutral-200 hover:border-neutral-300 text-neutral-600'}"
                >
                  Clé {i + 1} — {key.slice(0, 40)}…
                </button>
              {/each}
            </div>
          </div>
        {/if}
      {:else}
        <!-- GitHub login button -->
        <!-- data-sveltekit-reload: /api/github/login is a server endpoint (302 to
             GitHub), not a SvelteKit route — force a full navigation so the SPA
             router doesn't intercept it (which made the button need several clicks). -->
        <a
          href="/api/github/login"
          data-sveltekit-reload
          rel="external"
          class="flex items-center justify-center gap-2.5 w-full py-2.5 rounded-lg font-semibold text-sm
            bg-neutral-900 hover:bg-neutral-700 text-white transition-all"
        >
          <svg class="w-4 h-4 shrink-0" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/>
          </svg>
          Se connecter avec GitHub
        </a>

        <div class="flex items-center gap-3">
          <hr class="flex-1 border-neutral-200">
          <span class="text-xs text-neutral-400">ou</span>
          <hr class="flex-1 border-neutral-200">
        </div>

        {#if moodleConfigured}
          {#if !showMoodleForm}
            <button
              onclick={() => showMoodleForm = true}
              class="flex items-center justify-center gap-2.5 w-full py-2.5 rounded-lg font-semibold text-sm
                bg-[#f98012] hover:bg-[#e06f0a] text-white transition-all"
            >
              <svg class="w-4 h-4 shrink-0" fill="currentColor" viewBox="0 0 24 24"><path d="M12 3 1 9l4 2.18v6L12 21l7-3.82v-6l2-1.09V17h2V9L12 3z"/></svg>
              Se connecter avec Moodle
            </button>
          {:else}
            <div class="space-y-2 p-3 rounded border border-neutral-200 bg-neutral-50">
              <p class="section-label">Connexion Moodle</p>
              <input class="field text-sm" type="text" placeholder="Identifiant Moodle" bind:value={moodleUser} autocomplete="username" />
              <input class="field text-sm" type="password" placeholder="Mot de passe" bind:value={moodlePass} autocomplete="current-password"
                onkeydown={(e) => { if (e.key === 'Enter') loginMoodle(); }} />
              <button onclick={loginMoodle} disabled={moodleLoading || !moodleUser.trim() || !moodlePass} class="btn btn-primary w-full text-sm">
                {#if moodleLoading}
                  <span class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>
                {/if}
                Se connecter
              </button>
            </div>
          {/if}

          <div class="flex items-center gap-3">
            <hr class="flex-1 border-neutral-200">
            <span class="text-xs text-neutral-400">ou clé SSH</span>
            <hr class="flex-1 border-neutral-200">
          </div>
        {/if}
      {/if}

      {#if !githubLogin || githubKeys.length === 0 || githubKeys.length > 1}
        <div>
          <label for="sshkey" class="section-label mb-2 block">Clé publique SSH</label>
          <textarea
            id="sshkey"
            bind:value={sshkey}
            rows="4"
            placeholder="ssh-ed25519 AAAA..."
            class="field font-mono text-sm resize-none"
          ></textarea>
        </div>
      {/if}

      <button
        onclick={handleSSHKey}
        disabled={loading || !sshkey.trim()}
        class="btn btn-primary w-full"
      >
        {#if loading && !selectedPool}
          <span class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>
          Recherche en cours…
        {:else}
          Rechercher mes cours
        {/if}
      </button>

      {#if errorMsg && availablePools.length === 0}
        <div class="px-3 py-2.5 rounded bg-red-50 border border-red-200 text-red-700 text-sm animate-fade-in">{errorMsg}</div>
      {/if}
      {/if}
    </div>

    {#if noCoursFound}
      <div class="mt-6 card p-6 flex flex-col items-center text-center gap-3 animate-fade-in">
        <svg class="w-10 h-10 text-neutral-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
            d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <div>
          <p class="text-sm font-semibold text-neutral-700">Aucun cours lié à cette clé SSH</p>
          <p class="text-xs text-neutral-400 mt-1">Vérifiez que vous avez bien collé votre clé publique, ou contactez votre enseignant.</p>
        </div>
      </div>
    {/if}

    {#if availablePools.length > 0}
      <div class="mt-6">
        <p class="section-label mb-3 block">Cours disponibles</p>
        <div class="card overflow-hidden divide-y divide-neutral-100">
          {#each availablePools as pool}
            <div class="flex items-center justify-between px-5 py-3.5 hover:bg-neutral-50 transition-colors">
              <div>
                <p class="text-sm font-semibold text-neutral-900">{pool.pool_id}</p>
                <p class="text-xs text-neutral-500 mt-0.5">{pool.user_id}</p>
              </div>
              <button onclick={() => assignVM(pool)} disabled={loading} class="btn btn-primary text-xs px-4 py-2">
                {#if loading && selectedPool === pool}
                  <span class="w-3 h-3 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>
                  Attribution…
                {:else}
                  Rejoindre
                {/if}
              </button>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if assignError}
      <div class="mt-4 px-3 py-2.5 rounded bg-red-50 border border-red-200 text-red-700 text-sm animate-fade-in">{assignError}</div>
    {/if}

  {:else}
    <div class="mb-8 animate-fade-in">
      <div class="flex items-center gap-3 mb-2">
        <span class="flex h-3 w-3 relative">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-60"></span>
          <span class="relative inline-flex rounded-full h-3 w-3 bg-green-500"></span>
        </span>
        <h1 class="text-3xl font-bold text-primary-800" style="font-family: 'Source Sans 3', sans-serif;">VM attribuée</h1>
      </div>
      <p class="text-sm text-neutral-500 ml-6">
        {#if (vmAppPort > 0 && !appReady) || (vmAppPort === 0 && !guacUrl)}Démarrage en cours…{:else}Votre environnement est prêt.{/if}
      </p>
    </div>

    <div class="card p-6 space-y-5 animate-fade-in">

      {#if vmAppPort > 0}
        {#if appReady}
          <!-- JupyterLab -->
          <a
            href="http://{vmIp}:{vmAppPort}/lab"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center justify-center gap-2.5 w-full py-3.5 rounded-lg font-semibold text-base
              bg-amber-500 hover:bg-amber-400 text-white transition-all shadow-sm hover:shadow-md"
          >
            <svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"/>
            </svg>
            Ouvrir JupyterLab
          </a>
          <!-- Jupyter Classic — needed for nbgrader "Assignments" tab -->
          <!-- Submit Button -->
          <div class="flex flex-col gap-2">
            <button
              onclick={submitWork}
              disabled={submitting}
              class="flex items-center justify-center gap-2.5 w-full py-2.5 rounded-lg font-semibold text-sm
                bg-white border border-amber-300 text-amber-700 hover:bg-amber-50 transition-all disabled:opacity-50"
            >
              {#if submitting}
                <span class="w-4 h-4 border-2 border-amber-400/40 border-t-amber-700 rounded-full shrink-0"
                  style="animation: spinnerGlow 0.8s linear infinite;"></span>
              {:else}
                <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/>
                </svg>
              {/if}
              Soumettre mes travaux
            </button>
            {#if submitStatus}
              <p class="text-xs text-center {submitStatus.startsWith('Erreur') ? 'text-red-600' : 'text-green-600'}">{submitStatus}</p>
            {/if}
          </div>
        {:else}
          <div class="flex items-center justify-center gap-2.5 w-full py-3.5 rounded-lg font-semibold text-base
            bg-neutral-200 text-neutral-500 cursor-not-allowed select-none">
            <span class="w-4 h-4 border-2 border-neutral-400/40 border-t-neutral-500 rounded-full shrink-0"
              style="animation: spinnerGlow 0.8s linear infinite;"></span>
            Démarrage de l'application…
          </div>
        {/if}
      {/if}

      {#if guacUrl}
        <a
          href={guacUrl}
          target="_blank"
          rel="noopener noreferrer"
          class="flex items-center justify-center gap-2.5 w-full py-3.5 rounded-lg font-semibold text-base
            bg-primary-700 hover:bg-primary-600 text-white transition-all shadow-sm hover:shadow-md"
        >
          <svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
          </svg>
          Ouvrir le terminal web (Guacamole)
        </a>
      {:else if vmAppPort === 0}
        <!-- VM sans app (Ubuntu…) : la connexion Guacamole se prépare -->
        <div class="flex items-center justify-center gap-2.5 w-full py-3.5 rounded-lg font-semibold text-base
          bg-neutral-200 dark:bg-neutral-800 text-neutral-500 dark:text-neutral-400 cursor-not-allowed select-none">
          <span class="w-4 h-4 border-2 border-neutral-400/40 border-t-neutral-500 rounded-full shrink-0"
            style="animation: spinnerGlow 0.8s linear infinite;"></span>
          Préparation de l'accès terminal…
        </div>
      {/if}

      {#if vmUser}
      <hr class="border-neutral-200 dark:border-neutral-700"/>

      <div>
        <p class="section-label mb-2.5 block">Connexion SSH</p>
        <div class="flex items-center gap-2 bg-neutral-900 pl-4 pr-2 py-2 rounded-md font-mono">
          <svg class="w-4 h-4 text-primary-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3"/>
          </svg>
          <code class="text-sm text-green-400 select-all flex-1">ssh {vmUser}@{vmIp}</code>
          <button
            onclick={copyCmd}
            class="shrink-0 flex items-center gap-1.5 px-2.5 py-1.5 rounded text-xs font-semibold transition-all
              {copied ? 'bg-green-600 text-white' : 'bg-neutral-700 hover:bg-neutral-600 text-neutral-300'}"
            title="Copier"
          >
            {#if copied}
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/>
              </svg>
              Copié
            {:else}
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/>
              </svg>
              Copier
            {/if}
          </button>
        </div>
        <p class="text-xs text-neutral-400 mt-2">
          Si la connexion demande un mot de passe :
          <code class="font-mono text-neutral-500">ssh -i ~/.ssh/id_ed25519 {vmUser}@{vmIp}</code>
        </p>
      </div>
      {/if}

      <button
        onclick={() => {
        // Revenir à la liste des cours SANS déconnecter (on garde la session Moodle/SSH).
        vmIp = ""; vmUser = ""; vmAppPort = 0; guacUrl = "";
        selectedPool = null;
        appReady = false; probing = false;
        if (probeInterval) { clearInterval(probeInterval); probeInterval = null; }
        if (moodleEmail) refreshMoodlePools();
        else { availablePools = []; sshkey = ""; }
      }}
        class="btn btn-secondary text-sm"
      >
        ← Retour
      </button>
    </div>
  {/if}

</div>

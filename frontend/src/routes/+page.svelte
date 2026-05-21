<script lang="ts">
  import { returnPoolsWithKey, attribVMinPool } from "$lib/grpc/attribVMService/attribVMService";

  let sshkey = $state("");
  let availablePools: { pool_id: string; user_id: string }[] = $state([]);
  let selectedPool: { pool_id: string; user_id: string } | null = $state(null);
  let vmIp = $state("");
  let vmUser = $state("");
  let loading = $state(false);
  let errorMsg = $state("");
  let copied = $state(false);

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
    loading = true; errorMsg = ""; availablePools = []; selectedPool = null; vmIp = "";
    try {
      availablePools = await returnPoolsWithKey(sshkey);
      if (availablePools.length === 0) errorMsg = "Aucun cours disponible pour cette clé SSH.";
    } catch { errorMsg = "Erreur lors de la récupération des cours disponibles."; }
    finally { loading = false; }
  }

  function computeUsername(poolId: string): string {
    let name = ("student_" + poolId).split("@")[0].toLowerCase();
    name = name.replace(/[^a-z0-9_.-]/g, "");
    if (name.length > 32) name = name.substring(0, 32);
    return name;
  }

  async function assignVM(pool: { pool_id: string; user_id: string }) {
    selectedPool = pool; loading = true; errorMsg = ""; vmIp = ""; vmUser = "";
    try {
      const result = await attribVMinPool(pool.pool_id, pool.user_id, sshkey);
      vmIp = result.ip;
      vmUser = result.username || computeUsername(pool.pool_id);
    } catch (err: any) {
      errorMsg = err?.message || "Erreur lors de l'attribution de la VM.";
    } finally { loading = false; }
  }
</script>

<svelte:head>
  <title>CloudPoolManager — Portail Étudiant</title>
</svelte:head>

<div class="max-w-lg mx-auto py-10 animate-fade-up">

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

      {#if errorMsg}
        <div class="px-3 py-2.5 rounded bg-red-50 border border-red-200 text-red-700 text-sm animate-fade-in">{errorMsg}</div>
      {/if}
    </div>

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

  {:else}
    <div class="mb-8 animate-fade-in">
      <div class="flex items-center gap-3 mb-2">
        <span class="flex h-3 w-3 relative">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-60"></span>
          <span class="relative inline-flex rounded-full h-3 w-3 bg-green-500"></span>
        </span>
        <h1 class="text-3xl font-bold text-primary-800" style="font-family: 'Source Sans 3', sans-serif;">VM attribuée</h1>
      </div>
      <p class="text-sm text-neutral-500 ml-6">Votre environnement est prêt.</p>
    </div>

    <div class="card p-6 space-y-5 animate-fade-in">
      <div>
        <p class="section-label mb-2.5 block">Commande de connexion</p>
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
      </div>

      <p class="text-xs text-neutral-400">
        Si la connexion demande un mot de passe, précisez votre clé privée :
        <code class="font-mono text-neutral-500">ssh -i ~/.ssh/id_ed25519 {vmUser}@{vmIp}</code>
      </p>

      <button
        onclick={() => { vmIp = ""; vmUser = ""; availablePools = []; sshkey = ""; }}
        class="btn btn-secondary text-sm"
      >
        ← Retour
      </button>
    </div>
  {/if}

</div>

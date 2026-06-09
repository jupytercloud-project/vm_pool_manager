<script lang="ts">
  import { authStore, configs, serverPools, flavors, images, networks } from '$lib/store';
  import { logout } from '$lib/store/authStore';
  import { simpleMode, darkMode } from '$lib/store/uiStore';
  import { goto } from '$app/navigation';
  import { create } from '@bufbuild/protobuf';
  import { AddPersonalSSHKeyRequestSchema, type AddPersonalSSHKeyRequest, type AddPersonnalSSHKeyResponse } from '$lib/grpc/frontcontrol_pb';
  import { addSSHPersonalKey } from '$lib/grpc/userUpdateService/userService';

  let sshModal = $state(false);
  let sshKey = $state('');
  let sshSuccess = $state(false);
  let sshError = $state('');

  async function handleSSHKeySubmit() {
    sshError = ''; sshSuccess = false;
    const req: AddPersonalSSHKeyRequest = create(AddPersonalSSHKeyRequestSchema, {
      userId: $authStore?.email ?? '', publicKey: sshKey,
    });
    try {
      const res: AddPersonnalSSHKeyResponse = await addSSHPersonalKey(req);
      if (res.success) { sshSuccess = true; setTimeout(() => { sshModal = false; sshKey = ''; sshSuccess = false; }, 1200); }
      else { sshError = 'Erreur lors de l\'ajout de la clé'; }
    } catch { sshError = 'Erreur de connexion'; }
  }
</script>

<svelte:head><title>Profil — CloudPoolManager</title></svelte:head>

<div class="space-y-7 animate-fade-up">

  <!-- Header -->
  <div class="flex items-start justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800 dark:text-primary-300">Paramètres</h1>
      <p class="text-sm text-neutral-500 mt-1">{$authStore?.email}</p>
    </div>
    <button onclick={() => sshModal = true} class="btn btn-secondary text-sm">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
          d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/>
      </svg>
      Ajouter une clé SSH
    </button>
  </div>

  <!-- Préférences -->
  <div class="card p-5">
    <h2 class="text-sm font-bold text-neutral-800 dark:text-neutral-200 mb-1">Préférences</h2>
    <div class="divide-y divide-black/5 dark:divide-white/5">
      <!-- Apparence -->
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
      {#if $authStore?.role === 'admin'}
        <!-- Mode d'affichage -->
        <div class="flex items-center justify-between py-3.5">
          <div class="flex items-center gap-3">
            <svg class="w-5 h-5 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>
            <div>
              <p class="text-sm font-medium text-neutral-800 dark:text-neutral-200">Mode d'affichage</p>
              <p class="text-xs text-neutral-400">{$simpleMode ? 'Simple (épuré)' : 'Expert (complet)'}</p>
            </div>
          </div>
          <button onclick={() => simpleMode.update(v => !v)} role="switch" aria-checked={$simpleMode} aria-label="Mode simple"
            class="relative w-11 h-6 rounded-full transition-colors shrink-0 {$simpleMode ? 'bg-amber-500' : 'bg-neutral-300 dark:bg-neutral-600'}">
            <span class="absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow transition-transform {$simpleMode ? 'translate-x-5' : ''}"></span>
          </button>
        </div>
      {/if}
      <!-- Compte -->
      <div class="flex items-center justify-between py-3.5">
        <div class="flex items-center gap-3">
          <svg class="w-5 h-5 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
          <div>
            <p class="text-sm font-medium text-neutral-800 dark:text-neutral-200">Compte</p>
            <p class="text-xs text-neutral-400">{$authStore?.email ?? '—'}{#if $authStore?.role === 'admin'} · Admin{/if}</p>
          </div>
        </div>
        <button onclick={logout} class="btn btn-secondary text-xs">Déconnexion</button>
      </div>
    </div>
  </div>

  <!-- Serverpools table -->
  <div class="card overflow-hidden">
    <div class="px-5 py-4 border-b border-neutral-200 bg-neutral-50 flex items-center justify-between">
      <h2 class="text-sm font-bold text-neutral-800">Mes Serverpools</h2>
      <span class="badge badge-info">{$serverPools.length}</span>
    </div>

    {#if $serverPools.length === 0}
      <div class="px-5 py-10 text-center text-sm text-neutral-400">Aucun serverpool</div>
    {:else}
      <div class="overflow-x-auto">
        <table class="data-table">
          <thead>
            <tr>
              <th>Nom</th>
              <th>Image</th>
              <th>Flavor</th>
              <th>Réseau</th>
              <th class="text-center">VMs</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each $serverPools as sp, i}
              <tr class="animate-slide-right" style="animation-delay:{i*0.04}s">
                <td class="font-semibold text-neutral-900">{sp.name}</td>
                <td class="text-neutral-600">{$images.find(img => img.id === sp.image)?.name ?? sp.image}</td>
                <td class="text-neutral-600">{$flavors.find(f => f.id === sp.flavor)?.name ?? sp.flavor}</td>
                <td class="text-neutral-600">{$networks.find(n => n.id === sp.network)?.name ?? sp.network}</td>
                <td class="text-center">
                  <span class="text-xs text-neutral-500 tabular-nums font-medium">{sp.minVm}–{sp.maxVm}</span>
                </td>
                <td class="text-right pr-5">
                  <button onclick={() => goto(`/serverpool/${sp.name}`)} class="btn btn-primary text-xs px-3.5 py-1.5">
                    Inspecter
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <!-- Configs table -->
  <div class="card overflow-hidden">
    <div class="px-5 py-4 border-b border-neutral-200 bg-neutral-50 flex items-center justify-between">
      <h2 class="text-sm font-bold text-neutral-800">Mes Configurations</h2>
      <span class="badge badge-info">{$configs.length}</span>
    </div>

    {#if $configs.length === 0}
      <div class="px-5 py-10 text-center text-sm text-neutral-400">Aucune configuration</div>
    {:else}
      <div class="overflow-x-auto">
        <table class="data-table">
          <thead><tr><th>Nom</th><th>Aperçu</th></tr></thead>
          <tbody>
            {#each $configs as conf, i}
              <tr class="animate-slide-right" style="animation-delay:{i*0.04}s">
                <td class="font-semibold text-neutral-900">{conf.name}</td>
                <td class="text-neutral-500 font-mono text-xs truncate max-w-xs">
                  {conf.data?.slice(0, 60)}{(conf.data?.length ?? 0) > 60 ? '…' : ''}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>

<!-- SSH Modal -->
{#if sshModal}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <div class="modal-box">
      <div class="flex items-center justify-between mb-5">
        <h3 class="text-base font-bold text-neutral-900" style="font-family: 'Source Sans 3', sans-serif;">Ajouter une clé SSH</h3>
        <button onclick={() => sshModal = false} class="text-neutral-400 hover:text-neutral-700 transition-colors p-1 rounded hover:bg-neutral-100">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>

      {#if sshError}
        <div class="mb-4 px-3 py-2.5 rounded bg-red-50 border border-red-200 text-red-700 text-sm animate-fade-in">{sshError}</div>
      {/if}
      {#if sshSuccess}
        <div class="mb-4 px-3 py-2.5 rounded bg-green-50 border border-green-200 text-green-700 text-sm animate-fade-in">Clé ajoutée</div>
      {/if}

      <label class="section-label mb-2 block">Clé publique</label>
      <textarea
        class="field font-mono text-xs resize-none"
        rows="5"
        placeholder="ssh-ed25519 AAAA..."
        bind:value={sshKey}
      ></textarea>

      <button
        onclick={handleSSHKeySubmit}
        disabled={!sshKey.trim()}
        class="btn btn-primary w-full mt-4 text-sm"
      >
        Enregistrer
      </button>
    </div>
  </div>
{/if}

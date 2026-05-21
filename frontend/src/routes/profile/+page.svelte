<script lang="ts">
  import { authStore, configs, serverPools, flavors, images, networks } from '$lib/store';
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
      <h1 class="text-3xl font-bold text-primary-800" style="font-family: 'Source Sans 3', sans-serif;">Profil</h1>
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

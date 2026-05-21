<script lang="ts">
  import { onMount } from 'svelte';
  import { authStore } from '$lib/store';
  import { browser } from '$app/environment';

  interface VMInstance {
    id: string; name: string; ip: string; public_ip: string; az: string;
    status: string; healthy: boolean; activity_status: string;
    registered_at: string; last_seen: string; raw_meta: Record<string, string>;
  }
  interface InventoryPool { pool_id: string; user_id: string; vms: VMInstance[]; }

  let pools: InventoryPool[] = $state([]);
  let loading = $state(true);
  let error = $state('');
  let lastRefresh = $state('');
  let refreshing = $state(false);
  let autoRefresh: ReturnType<typeof setInterval> | null = null;

  async function fetchInventory(silent = false) {
    if (!silent) loading = true; else refreshing = true;
    try {
      const res = await fetch('/api/inventory');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      pools = await res.json();
      lastRefresh = new Date().toLocaleTimeString('fr-FR');
      error = '';
    } catch { error = "Impossible de charger l'inventaire"; }
    finally { loading = false; refreshing = false; }
  }

  onMount(() => {
    if (!browser) return;
    if (!$authStore || $authStore.role !== 'admin') { window.location.href = '/'; return; }
    fetchInventory();
    autoRefresh = setInterval(() => fetchInventory(true), 15000);
    return () => { if (autoRefresh) clearInterval(autoRefresh); };
  });

  function timeSince(dateStr: string): string {
    const diff = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
    if (diff < 60) return `${diff}s`;
    if (diff < 3600) return `${Math.floor(diff/60)}min`;
    if (diff < 86400) return `${Math.floor(diff/3600)}h`;
    return `${Math.floor(diff/86400)}j`;
  }

  const totalVMs = $derived(pools.reduce((a, p) => a + p.vms.length, 0));
  const healthyVMs = $derived(pools.reduce((a, p) => a + p.vms.filter(v => v.healthy).length, 0));
  const readyVMs = $derived(pools.reduce((a, p) => a + p.vms.filter(v => v.status === 'ready').length, 0));
  const activeVMs = $derived(pools.reduce((a, p) => a + p.vms.filter(v => v.activity_status !== 'idle').length, 0));
</script>

<svelte:head><title>Inventaire VM — CloudPoolManager</title></svelte:head>

<div class="space-y-7 animate-fade-up">

  <!-- Header -->
  <div class="flex items-start justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800" style="font-family: 'Source Sans 3', sans-serif;">Inventaire</h1>
      <p class="text-sm text-neutral-500 mt-1">Supervision en temps réel des instances provisionnées</p>
    </div>
    <div class="flex items-center gap-3">
      {#if lastRefresh}
        <span class="text-xs text-neutral-400">Maj {lastRefresh}</span>
      {/if}
      <button
        onclick={() => fetchInventory(true)}
        disabled={refreshing}
        class="btn btn-secondary text-xs px-3.5 py-2 gap-1.5"
      >
        <svg class="w-3.5 h-3.5 {refreshing ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
        </svg>
        Actualiser
      </button>
    </div>
  </div>

  <!-- Stats -->
  {#if !loading && !error}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      {#each [
        { label: 'Pools',       value: pools.length,                   accent: 'stat-accent-indigo',  color: 'text-primary-700' },
        { label: 'VMs total',   value: totalVMs,                       accent: 'stat-accent-violet',  color: 'text-primary-500' },
        { label: 'Santé',       value: `${healthyVMs}/${totalVMs}`,    accent: 'stat-accent-emerald', color: 'text-green-600'   },
        { label: 'Actives SSH', value: activeVMs,                      accent: 'stat-accent-amber',   color: 'text-amber-600'   },
      ] as stat, i}
        <div class="card p-5 {stat.accent} animate-fade-up" style="animation-delay:{i*0.05}s">
          <p class="section-label mb-2">{stat.label}</p>
          <p class="text-2xl font-bold {stat.color}">{stat.value}</p>
        </div>
      {/each}
    </div>
  {/if}

  <!-- Loading -->
  {#if loading}
    <div class="flex flex-col items-center justify-center py-24 gap-4">
      <div class="w-9 h-9 rounded-full border-2 border-neutral-200 border-t-primary-700" style="animation: spinnerGlow 0.7s linear infinite;"></div>
      <p class="text-sm text-neutral-500">Chargement de l'inventaire…</p>
    </div>
  {/if}

  <!-- Error -->
  {#if error}
    <div class="card px-4 py-3 border-red-200 bg-red-50 text-red-700 text-sm animate-fade-in">{error}</div>
  {/if}

  <!-- Pool sections -->
  {#if !loading && !error}
    {#each pools as pool, pi}
      <div class="card overflow-hidden animate-fade-up" style="animation-delay:{pi*0.06}s">
        <!-- Pool header -->
        <div class="flex items-center justify-between px-5 py-3.5 bg-neutral-50 border-b border-neutral-200">
          <div class="flex items-center gap-3">
            <div class="relative flex h-2.5 w-2.5">
              {#if pool.vms.every(v => v.healthy)}
                <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-60"></span>
              {/if}
              <span class="relative inline-flex rounded-full h-2.5 w-2.5 {pool.vms.every(v => v.healthy) ? 'bg-green-500' : 'bg-red-500'}"></span>
            </div>
            <span class="text-sm font-bold text-neutral-900">{pool.pool_id}</span>
            <span class="text-xs text-neutral-500">{pool.user_id}</span>
          </div>
          <span class="text-xs text-neutral-400 tabular-nums">{pool.vms.length} VM{pool.vms.length > 1 ? 's' : ''}</span>
        </div>

        <!-- Table -->
        <div class="overflow-x-auto">
          <table class="data-table">
            <thead>
              <tr>
                <th>Nom</th>
                <th>IP</th>
                <th>Statut</th>
                <th>Santé</th>
                <th>Activité SSH</th>
                <th class="text-right">Dernière activité</th>
              </tr>
            </thead>
            <tbody>
              {#each pool.vms as vm}
                <tr>
                  <td><span class="font-mono text-xs text-neutral-700">{vm.name}</span></td>
                  <td><span class="font-mono text-xs text-neutral-700">{vm.ip}</span></td>
                  <td>
                    <span class="badge {vm.status === 'ready' ? 'badge-ready' : vm.status === 'starting' ? 'badge-starting' : 'badge-error'}">
                      {vm.status}
                    </span>
                  </td>
                  <td>
                    <div class="flex items-center gap-1.5">
                      <span class="w-1.5 h-1.5 rounded-full {vm.healthy ? 'bg-green-500' : 'bg-red-500'}"></span>
                      <span class="text-xs font-medium {vm.healthy ? 'text-green-700' : 'text-red-700'}">{vm.healthy ? 'OK' : 'KO'}</span>
                    </div>
                  </td>
                  <td>
                    {#if vm.activity_status && vm.activity_status !== 'idle'}
                      <span class="badge badge-info gap-1.5">
                        <span class="relative flex h-1.5 w-1.5">
                          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
                          <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-sky-400"></span>
                        </span>
                        SSH actif
                      </span>
                    {:else}
                      <span class="text-xs text-neutral-400">Inactif</span>
                    {/if}
                  </td>
                  <td class="text-right">
                    <span class="text-xs text-neutral-400 tabular-nums">il y a {timeSince(vm.last_seen)}</span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/each}

    {#if pools.length === 0}
      <div class="card flex flex-col items-center justify-center py-24 text-center">
        <svg class="w-10 h-10 text-neutral-300 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
            d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
        <p class="text-neutral-500 text-sm font-medium">Aucune VM provisionnée pour le moment</p>
        <p class="text-neutral-400 text-xs mt-1">Les instances apparaîtront ici une fois démarrées</p>
      </div>
    {/if}
  {/if}
</div>

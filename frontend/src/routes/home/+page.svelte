<script lang="ts">
  import { onMount } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { browser } from '$app/environment';
  import { goto } from '$app/navigation';
  import { apiFetch } from '$lib/api';
  import { authStore } from '$lib/store/authStore';
  import { meStore } from '$lib/store/meStore';
  import { serverPools } from '$lib/store';

  const isAdmin = $derived($meStore?.is_admin ?? ($authStore?.role === 'admin'));
  const isStaff = $derived($meStore?.is_staff ?? ($authStore?.role === 'admin'));
  const role = $derived($meStore?.role ?? $authStore?.role ?? 'student');
  const isChercheur = $derived(role === 'chercheur');

  interface Card { href: string; label: string; icon: string; }
  const ICONS: Record<string, string> = {
    inventory: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01',
    serverpool: 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2',
    grading: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4',
    usage: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
    config: 'M10.5 6h9.75M10.5 6a1.5 1.5 0 11-3 0m3 0a1.5 1.5 0 10-3 0M3.75 6H7.5m3 12h9.75m-9.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-3.75 0H7.5m9-6h3.75m-3.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-9.75 0h9.75',
    image: 'M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z',
    jobs: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
    environments: 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z',
  };

  const cards = $derived((): Card[] => {
    if (isChercheur) {
      return [
        { href: '/environments', label: $_('nav.myEnvironments'), icon: ICONS.environments },
        { href: '/jobs', label: $_('nav.jobs'), icon: ICONS.jobs },
        { href: '/usage', label: $_('nav.costs'), icon: ICONS.usage },
      ];
    }
    if (isStaff) {
      const c: Card[] = [];
      if (isAdmin) c.push({ href: '/inventory', label: $_('nav.inventory'), icon: ICONS.inventory });
      c.push({ href: '/serverpool', label: isAdmin ? $_('nav.serverpools') : $_('nav.myCourses'), icon: ICONS.serverpool });
      c.push({ href: '/grading', label: $_('nav.grading'), icon: ICONS.grading });
      if (isAdmin) {
        c.push({ href: '/usage', label: $_('nav.costs'), icon: ICONS.usage });
        c.push({ href: '/config', label: $_('nav.configs'), icon: ICONS.config });
        c.push({ href: '/propose-image', label: $_('nav.proposeImage'), icon: ICONS.image });
      }
      return c;
    }
    return [{ href: '/student', label: $_('nav.myCourses'), icon: ICONS.serverpool }];
  });

  const subtitle = $derived(
    isAdmin ? $_('home.subAdmin') : isChercheur ? $_('home.subChercheur') : isStaff ? $_('home.subProf') : $_('home.subStudent')
  );

  // Stats admin (depuis l'inventaire).
  let stats = $state<{ pools: number; vms: number; active: number } | null>(null);
  const myPools = $derived($serverPools?.length ?? 0);

  onMount(async () => {
    if (!browser) return;
    // Étudiant (github/moodle, sans compte staff) → portail étudiant.
    if (!$authStore) { goto('/student'); return; }
    if (isAdmin) {
      try {
        const r = await apiFetch('/api/inventory');
        if (r.ok) {
          const pools = await r.json();
          const vms = pools.reduce((n: number, p: any) => n + (p.vms?.length ?? 0), 0);
          const active = pools.reduce((n: number, p: any) => n + (p.vms?.filter((v: any) => v.activity_status === 'active' || v.activity_status === 'connected').length ?? 0), 0);
          stats = { pools: pools.length, vms, active };
        }
      } catch { /* ignore */ }
    }
  });
</script>

<svelte:head><title>{$_('home.title')}</title></svelte:head>

<div class="space-y-7 animate-fade-up">
  <div>
    <h1 class="text-3xl font-bold text-primary-800 dark:text-primary-300">
      {$_('home.hello')}{#if $authStore?.email}, <span class="text-neutral-800 dark:text-neutral-100">{$authStore.email}</span>{/if}
    </h1>
    <p class="text-sm text-neutral-500 mt-1">{subtitle}</p>
  </div>

  {#if isAdmin && stats}
    <div class="grid grid-cols-3 gap-4">
      <div class="card p-5"><p class="section-label">{$_('home.statPools')}</p><p class="text-2xl font-bold tabular-nums mt-1">{stats.pools}</p></div>
      <div class="card p-5"><p class="section-label">{$_('home.statVms')}</p><p class="text-2xl font-bold tabular-nums mt-1">{stats.vms}</p></div>
      <div class="card p-5"><p class="section-label">{$_('home.statActive')}</p><p class="text-2xl font-bold text-green-600 tabular-nums mt-1">{stats.active}</p></div>
    </div>
  {:else if isStaff || isChercheur}
    <div class="grid grid-cols-2 gap-4 max-w-md">
      <div class="card p-5"><p class="section-label">{$_('home.statMyPools')}</p><p class="text-2xl font-bold tabular-nums mt-1">{myPools}</p></div>
    </div>
  {/if}

  <div>
    <h2 class="text-sm font-bold text-neutral-700 dark:text-neutral-300 mb-3">{$_('home.quickActions')}</h2>
    <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
      {#each cards() as c}
        <a href={c.href} class="card card-interactive p-5 flex items-center gap-3 hover:border-primary-300 transition-colors">
          <span class="w-10 h-10 rounded-xl bg-primary-50 dark:bg-primary-900/30 flex items-center justify-center text-primary-700 dark:text-primary-300 shrink-0">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d={c.icon}/></svg>
          </span>
          <span class="text-sm font-semibold text-neutral-800 dark:text-neutral-200">{c.label}</span>
        </a>
      {/each}
    </div>
  </div>
</div>

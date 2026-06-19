<script lang="ts">
  import { onMount } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { apiFetch } from '$lib/api';
  import { browser } from '$app/environment';

  interface Group { key: string; vm_hours: number; vcpu_hours: number; gb_hours: number; cost: number; }
  interface Totals { vm_hours: number; vcpu_hours: number; gb_hours: number; cost: number; }

  let by = $state<'user' | 'pool'>('user');
  let month = $state(browser ? new Date().toISOString().slice(0, 7) : '');
  let currency = $state('€');
  let groups = $state<Group[]>([]);
  let totals = $state<Totals>({ vm_hours: 0, vcpu_hours: 0, gb_hours: 0, cost: 0 });
  let loading = $state(true);

  interface StorageGroup { key: string; vms: number; disk_gb: number; quota_gb: number; over_quota: boolean; }
  let storageGroups = $state<StorageGroup[]>([]);
  let storageQuota = $state(0);

  async function load() {
    loading = true;
    try {
      const res = await apiFetch(`/api/usage?month=${encodeURIComponent(month)}&by=${by}`);
      if (res.ok) {
        const d = await res.json();
        groups = (d.groups ?? []).sort((a: Group, b: Group) => b.cost - a.cost);
        totals = d.totals ?? { vm_hours: 0, vcpu_hours: 0, gb_hours: 0, cost: 0 };
        currency = d.currency ?? '€';
      }
      const sr = await apiFetch(`/api/storage?by=${by}`);
      if (sr.ok) {
        const sd = await sr.json();
        storageGroups = (sd.groups ?? []).sort((a: StorageGroup, b: StorageGroup) => b.disk_gb - a.disk_gb);
        storageQuota = sd.quota_gb ?? 0;
      }
    } catch { /* ignore */ }
    finally { loading = false; }
  }

  const fmt = (n: number) => (n ?? 0).toLocaleString(undefined, { maximumFractionDigits: 1 });
  const money = (n: number) => (n ?? 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });

  onMount(load);
</script>

<svelte:head><title>{$_('usage.pageTitle')}</title></svelte:head>

<div class="space-y-6 animate-fade-up">
  <div class="flex flex-wrap items-end justify-between gap-3">
    <div>
      <h1 class="text-3xl font-bold text-primary-800 dark:text-primary-300">{$_('usage.title')}</h1>
      <p class="text-sm text-neutral-500 mt-1">{$_('usage.subtitle')}</p>
    </div>
    <div class="flex items-center gap-2">
      <input type="month" bind:value={month} onchange={load} class="field text-sm w-auto py-1.5" />
      <div class="inline-flex rounded-lg border border-neutral-200 dark:border-neutral-700 overflow-hidden text-sm">
        <button onclick={() => { by = 'user'; load(); }} class="px-3 py-1.5 {by === 'user' ? 'bg-primary-600 text-white' : 'text-neutral-500'}">{$_('usage.byUser')}</button>
        <button onclick={() => { by = 'pool'; load(); }} class="px-3 py-1.5 {by === 'pool' ? 'bg-primary-600 text-white' : 'text-neutral-500'}">{$_('usage.byPool')}</button>
      </div>
    </div>
  </div>

  <!-- Totaux -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
    <div class="card p-5"><p class="section-label">{$_('usage.cost')}</p><p class="text-2xl font-bold text-primary-700 tabular-nums mt-1">{money(totals.cost)} {currency}</p></div>
    <div class="card p-5"><p class="section-label">{$_('usage.vmHours')}</p><p class="text-2xl font-bold tabular-nums mt-1">{fmt(totals.vm_hours)}</p></div>
    <div class="card p-5"><p class="section-label">{$_('usage.vcpuHours')}</p><p class="text-2xl font-bold tabular-nums mt-1">{fmt(totals.vcpu_hours)}</p></div>
    <div class="card p-5"><p class="section-label">{$_('usage.gbHours')}</p><p class="text-2xl font-bold tabular-nums mt-1">{fmt(totals.gb_hours)}</p></div>
  </div>

  {#if !loading && totals.cost === 0}
    <div class="px-4 py-3 rounded-xl border border-sky-200 bg-sky-50 text-sky-900 text-sm dark:bg-sky-900/20 dark:border-sky-700 dark:text-sky-200">
      {$_('usage.accruing')}
    </div>
  {/if}

  <!-- Détail -->
  <div class="card overflow-hidden">
    {#if loading}
      <p class="text-sm text-neutral-400 p-6 text-center">{$_('usage.loading')}</p>
    {:else if groups.length === 0}
      <p class="text-sm text-neutral-400 p-6 text-center">{$_('usage.empty')}</p>
    {:else}
      <table class="w-full text-sm">
        <thead class="bg-neutral-50 dark:bg-neutral-800/50 text-left text-xs text-neutral-500">
          <tr>
            <th class="px-4 py-2.5 font-semibold">{by === 'user' ? $_('usage.colUser') : $_('usage.colPool')}</th>
            <th class="px-4 py-2.5 font-semibold text-right">{$_('usage.vmHours')}</th>
            <th class="px-4 py-2.5 font-semibold text-right">{$_('usage.vcpuHours')}</th>
            <th class="px-4 py-2.5 font-semibold text-right">{$_('usage.cost')}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-neutral-100 dark:divide-neutral-800">
          {#each groups as g}
            <tr>
              <td class="px-4 py-2.5 font-medium text-neutral-800 dark:text-neutral-200">{g.key}</td>
              <td class="px-4 py-2.5 text-right tabular-nums">{fmt(g.vm_hours)}</td>
              <td class="px-4 py-2.5 text-right tabular-nums">{fmt(g.vcpu_hours)}</td>
              <td class="px-4 py-2.5 text-right tabular-nums font-semibold text-primary-700">{money(g.cost)} {currency}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
  <!-- Stockage alloué + quotas (D2) -->
  <div class="card p-5 space-y-3">
    <div class="flex items-center justify-between">
      <h2 class="text-sm font-bold text-neutral-800 dark:text-neutral-200">{$_('usage.storageTitle')}</h2>
      {#if storageQuota > 0}<span class="text-xs text-neutral-400">{$_('usage.quota')} : {storageQuota} GB</span>{/if}
    </div>
    {#if storageGroups.length === 0}
      <p class="text-sm text-neutral-400 py-2">{$_('usage.empty')}</p>
    {:else}
      <div class="space-y-2.5">
        {#each storageGroups as g}
          {@const pct = storageQuota > 0 ? Math.min(100, Math.round((g.disk_gb / storageQuota) * 100)) : 0}
          <div>
            <div class="flex items-center justify-between text-sm mb-1">
              <span class="font-medium text-neutral-800 dark:text-neutral-200">{g.key} <span class="text-xs text-neutral-400">· {g.vms} VM{g.vms > 1 ? 's' : ''}</span></span>
              <span class="tabular-nums {g.over_quota ? 'text-red-600 font-semibold' : 'text-neutral-500'}">
                {g.disk_gb} GB{#if storageQuota > 0} / {storageQuota} GB{/if}
                {#if g.over_quota} · {$_('usage.overQuota')}{/if}
              </span>
            </div>
            {#if storageQuota > 0}
              <div class="h-1.5 rounded-full bg-neutral-100 dark:bg-neutral-800 overflow-hidden">
                <div class="h-full rounded-full {g.over_quota ? 'bg-red-500' : 'bg-primary-500'}" style="width: {pct}%"></div>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <p class="text-xs text-neutral-400">{$_('usage.disclaimer')} {$_('usage.storageNote')}</p>
</div>

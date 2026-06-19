<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { apiFetch } from '$lib/api';
  import { browser } from '$app/environment';
  import { serverPools } from '$lib/store';

  interface Job {
    id: number; name: string; pool_id: string; status: string; exit_code: number;
    log: string; vm_name: string; auto_stop: boolean; priority: number; created_at: string; finished_at?: string;
  }

  let name = $state('');
  let poolId = $state('');
  let script = $state('#!/usr/bin/env bash\n');
  let autoStop = $state(true);
  let priority = $state(0);
  let submitting = $state(false);
  let submitMsg = $state('');
  let jobs = $state<Job[]>([]);
  let openLog = $state<number | null>(null);
  let timer: ReturnType<typeof setInterval> | null = null;

  async function loadJobs() {
    try {
      const r = await apiFetch('/api/jobs');
      if (r.ok) jobs = (await r.json()).jobs ?? [];
    } catch { /* ignore */ }
  }

  async function submit() {
    if (!poolId || !script.trim() || submitting) return;
    submitting = true; submitMsg = '';
    try {
      const r = await apiFetch('/api/jobs', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), pool_id: poolId, script, priority, auto_stop: autoStop }),
      });
      const d = await r.json();
      if (!r.ok || !d.ok) submitMsg = $_('jobs.submitError') + (d.error ?? '');
      else { submitMsg = ''; name = ''; await loadJobs(); }
    } catch { submitMsg = $_('jobs.submitError'); }
    finally { submitting = false; }
  }

  async function cancel(id: number) {
    try { await apiFetch(`/api/jobs/cancel?id=${id}`, { method: 'POST' }); await loadJobs(); } catch { /* ignore */ }
  }

  function badgeClass(s: string): string {
    switch (s) {
      case 'succeeded': return 'bg-green-100 text-green-700 border-green-200';
      case 'failed': return 'bg-red-100 text-red-700 border-red-200';
      case 'running': return 'bg-sky-100 text-sky-700 border-sky-200';
      case 'canceled': return 'bg-neutral-100 text-neutral-500 border-neutral-200';
      default: return 'bg-amber-100 text-amber-700 border-amber-200'; // queued
    }
  }

  onMount(() => {
    if (!browser) return;
    loadJobs();
    timer = setInterval(loadJobs, 5000);
  });
  onDestroy(() => { if (timer) clearInterval(timer); });
</script>

<svelte:head><title>{$_('jobs.pageTitle')}</title></svelte:head>

<div class="space-y-6 animate-fade-up">
  <div>
    <h1 class="text-3xl font-bold text-primary-800 dark:text-primary-300">{$_('jobs.title')}</h1>
    <p class="text-sm text-neutral-500 mt-1">{$_('jobs.subtitle')}</p>
  </div>

  <!-- Soumission -->
  <div class="card p-5 space-y-3">
    <h2 class="text-sm font-bold text-neutral-800 dark:text-neutral-200">{$_('jobs.newJob')}</h2>
    <div class="flex flex-wrap gap-3">
      <input bind:value={name} placeholder={$_('jobs.namePlaceholder')} class="field text-sm w-48" />
      <select bind:value={poolId} class="field text-sm w-56">
        <option value="" disabled>{$_('jobs.choosePool')}</option>
        {#each $serverPools as p}
          <option value={p.name}>{p.name}</option>
        {/each}
      </select>
      <select bind:value={priority} class="field text-sm w-auto">
        <option value={1}>{$_('jobs.prioHigh')}</option>
        <option value={0}>{$_('jobs.prioNormal')}</option>
        <option value={-1}>{$_('jobs.prioLow')}</option>
      </select>
      <label class="flex items-center gap-2 text-sm text-neutral-600 dark:text-neutral-300">
        <input type="checkbox" bind:checked={autoStop} class="w-4 h-4 accent-primary-700" /> {$_('jobs.autoStop')}
      </label>
    </div>
    <textarea bind:value={script} rows="6" spellcheck="false"
      class="field font-mono text-xs resize-y w-full" placeholder="#!/usr/bin/env bash"></textarea>
    <div class="flex items-center justify-between">
      {#if submitMsg}<p class="text-xs text-red-600">{submitMsg}</p>{:else}<span class="text-xs text-neutral-400">{$_('jobs.hint')}</span>{/if}
      <button onclick={submit} disabled={!poolId || !script.trim() || submitting} class="btn btn-primary text-sm">{$_('jobs.run')}</button>
    </div>
  </div>

  <!-- Liste -->
  <div class="card overflow-hidden">
    <div class="px-5 py-3 border-b border-neutral-200 dark:border-neutral-700 text-sm font-bold text-neutral-800 dark:text-neutral-200">{$_('jobs.history')}</div>
    {#if jobs.length === 0}
      <p class="text-sm text-neutral-400 p-6 text-center">{$_('jobs.empty')}</p>
    {:else}
      <div class="divide-y divide-neutral-100 dark:divide-neutral-800">
        {#each jobs as j}
          <div class="px-5 py-3">
            <div class="flex items-center gap-3">
              <span class="text-[10px] font-semibold px-2 py-0.5 rounded border {badgeClass(j.status)}">{$_('jobs.status_' + j.status)}</span>
              {#if j.priority > 0}<span class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-orange-100 text-orange-700 border border-orange-200" title={$_('jobs.prioHigh')}>↑</span>{:else if j.priority < 0}<span class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-neutral-100 text-neutral-500 border border-neutral-200" title={$_('jobs.prioLow')}>↓</span>{/if}
              <span class="font-medium text-neutral-800 dark:text-neutral-200">{j.name}</span>
              <span class="text-xs text-neutral-400">{j.pool_id}{#if j.vm_name} · {j.vm_name}{/if}{#if j.status === 'failed' || j.status === 'succeeded'} · exit {j.exit_code}{/if}</span>
              <div class="flex-1"></div>
              {#if j.status === 'queued'}
                <button onclick={() => cancel(j.id)} class="text-xs text-red-600 hover:underline">{$_('jobs.cancel')}</button>
              {/if}
              {#if j.log}
                <button onclick={() => (openLog = openLog === j.id ? null : j.id)} class="text-xs text-primary-600 hover:underline">{openLog === j.id ? $_('jobs.hideLog') : $_('jobs.showLog')}</button>
              {/if}
            </div>
            {#if openLog === j.id && j.log}
              <pre class="mt-2 p-3 rounded-lg bg-neutral-900 text-neutral-100 text-xs overflow-x-auto whitespace-pre-wrap max-h-80">{j.log}</pre>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
  <p class="text-xs text-neutral-400">{$_('jobs.note')}</p>
</div>

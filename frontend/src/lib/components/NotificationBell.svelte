<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '$lib/api';
  import { _ } from 'svelte-i18n';

  interface AuditLog { id: number; created_at: string; actor: string; role: string; method: string; path: string; ip: string; }

  let logs = $state<AuditLog[]>([]);
  let open = $state(false);
  let lastSeen = $state(0);

  const unread = $derived(logs.filter(l => l.id > lastSeen).length);

  function label(l: AuditLog): string {
    const p = l.path;
    if (p === '/api/vm/action') return $_('notif.vmAction');
    if (p === '/api/vm/rebuild') return $_('notif.vmRebuild');
    if (p === '/api/vm/resize') return $_('notif.vmResize');
    if (p === '/api/admin/users/role') return $_('notif.roleChange');
    if (p === '/api/admin/announcement') return $_('notif.announcement');
    if (p === '/api/pool/meta') return $_('notif.poolMeta');
    if (p === '/api/pool/presets') return $_('notif.preset');
    if (p === '/api/pool/broadcast-file') return $_('notif.broadcast');
    if (p === '/api/jobs') return $_('notif.jobSubmit');
    if (p === '/api/jobs/cancel') return $_('notif.jobCancel');
    if (p === '/api/xcours/import') return $_('notif.importX');
    if (p === '/api/moodle/import') return $_('notif.importMoodle');
    if (p === '/api/moodle/attrib-vm') return $_('notif.attribVm');
    if (p === '/api/moodle/push-grades') return $_('notif.pushGrades');
    if (p === '/api/moodle/ssh-key') return $_('notif.sshKey');
    if (p.startsWith('/jobs/')) return $_('notif.jobDone') + ' (' + (p.split('/')[2] || '') + ')';
    if (p.startsWith('/api/nbgrader/')) return $_('notif.nbgrader') + ' (' + (p.split('/').pop() || '') + ')';
    if (l.method === 'DELETE') return $_('notif.deletion');
    if (l.method === 'POST' || l.method === 'PUT' || l.method === 'PATCH') return $_('notif.genericChange');
    return l.method + ' ' + p;
  }
  function timeAgo(s: string): string {
    const d = Math.floor((Date.now() - new Date(s).getTime()) / 1000);
    if (d < 60) return `${d}s`;
    if (d < 3600) return `${Math.floor(d / 60)}min`;
    if (d < 86400) return `${Math.floor(d / 3600)}h`;
    return `${Math.floor(d / 86400)}j`;
  }

  async function load() {
    try {
      const r = await apiFetch('/api/admin/audit?limit=20');
      if (r.ok) logs = (await r.json()).logs ?? [];
    } catch { /* ignore */ }
  }
  function toggle() {
    open = !open;
    if (open && logs.length) {
      lastSeen = Math.max(...logs.map(l => l.id));
      try { localStorage.setItem('auditLastSeen', String(lastSeen)); } catch { /* ignore */ }
    }
  }

  onMount(() => {
    try { lastSeen = Number(localStorage.getItem('auditLastSeen') || '0'); } catch { /* ignore */ }
    load();
    const id = setInterval(load, 30000);
    return () => clearInterval(id);
  });
</script>

<div class="relative">
  <button onclick={toggle} title={$_('notif.title')} aria-label={$_('notif.title')}
    class="relative p-2 rounded-full text-neutral-500 dark:text-neutral-400 hover:text-primary-700 dark:hover:text-primary-300 hover:bg-black/5 dark:hover:bg-white/5 transition-colors">
    <svg class="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/>
    </svg>
    {#if unread > 0}
      <span class="absolute -top-0.5 -right-0.5 bg-red-500 text-white text-[10px] font-bold rounded-full min-w-[16px] h-4 px-1 flex items-center justify-center">{unread > 9 ? '9+' : unread}</span>
    {/if}
  </button>

  {#if open}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="fixed inset-0 z-40" onclick={() => (open = false)}></div>
    <div class="absolute right-0 mt-2 w-80 max-h-96 overflow-y-auto rounded-xl border border-neutral-200 dark:border-neutral-700 bg-white dark:bg-neutral-800 shadow-xl z-50 p-2">
      <div class="px-2 py-1.5 text-xs font-semibold text-neutral-500 dark:text-neutral-400">{$_('notif.recent')}</div>
      {#if logs.length === 0}
        <p class="text-sm text-neutral-400 px-2 py-4 text-center">{$_('notif.empty')}</p>
      {:else}
        {#each logs as l}
          <div class="px-2 py-2 rounded-lg {l.id > lastSeen ? 'bg-primary-50/60 dark:bg-primary-900/20' : ''}">
            <p class="text-sm text-neutral-800 dark:text-neutral-200">{label(l)}</p>
            <p class="text-xs text-neutral-400 truncate">{l.actor} · {timeAgo(l.created_at)}</p>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<script lang="ts">
  import { onMount } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { apiFetch } from '$lib/api';
  import { authStore, flavors } from '$lib/store';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import { simpleMode, refreshInterval } from '$lib/store/uiStore';
  import { displayName } from '$lib/displayName';
  import { browser } from '$app/environment';

  interface VMInstance {
    id: string; name: string; ip: string; public_ip: string; az: string;
    status: string; healthy: boolean; activity_status: string;
    registered_at: string; last_seen: string; raw_meta: Record<string, string>;
    power_state?: string;    // état Nova live : ACTIVE | SHUTOFF | SUSPENDED…
    guac_url?: string;
    grafana_url?: string;
    student?: string;        // étudiant attribué (par IP)
    is_instructor?: boolean; // VM de l'enseignant
  }

  // Un seul badge d'état clair, basé sur l'état Nova réel.
  function powerBadge(ps?: string): { label: string; cls: string } {
    switch (ps) {
      case 'ACTIVE': return { label: $_('inventory.powerActive'), cls: 'bg-green-100 text-green-700 border-green-200' };
      case 'SHUTOFF': return { label: $_('inventory.powerShutoff'), cls: 'bg-neutral-100 text-neutral-500 border-neutral-200' };
      case 'SUSPENDED':
      case 'PAUSED': return { label: $_('inventory.powerSuspended'), cls: 'bg-amber-100 text-amber-700 border-amber-200' };
      case 'REBOOT':
      case 'HARD_REBOOT': return { label: $_('inventory.powerRebooting'), cls: 'bg-sky-100 text-sky-700 border-sky-200' };
      case 'BUILD': return { label: $_('inventory.powerBuilding'), cls: 'bg-sky-100 text-sky-700 border-sky-200' };
      case 'ERROR': return { label: $_('inventory.powerError'), cls: 'bg-red-100 text-red-700 border-red-200' };
      default: return { label: ps || '—', cls: 'bg-neutral-100 text-neutral-500 border-neutral-200' };
    }
  }
  interface InventoryPool { pool_id: string; user_id: string; vms: VMInstance[]; linked_course?: string; label?: string; tags?: string; compute?: boolean; }
  const tagList = (t?: string) => (t || '').split(',').map(s => s.trim()).filter(Boolean);

  let pools: InventoryPool[] = $state([]);
  let loading = $state(true);
  let error = $state('');
  let lastRefresh = $state('');
  let refreshing = $state(false);

  async function fetchInventory(silent = false) {
    if (!silent) loading = true; else refreshing = true;
    try {
      const res = await apiFetch('/api/inventory');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      pools = await res.json();
      lastRefresh = new Date().toLocaleTimeString('fr-FR');
      error = '';
    } catch { error = $_('inventory.errLoadInventory'); }
    finally { loading = false; refreshing = false; }
  }

  // Actions de cycle de vie d'une VM (Phase 0 : /api/vm/action).
  let actingId = $state<string | null>(null);
  // Boutons d'action VM harmonisés : carrés de taille fixe, couleurs cohérentes.
  const ABTN = 'inline-flex items-center justify-center w-9 h-9 rounded-lg border text-base transition-colors disabled:opacity-40 border-neutral-200 dark:border-neutral-700 text-neutral-600 dark:text-neutral-300 hover:border-primary-300 hover:text-primary-700';
  const ABTN_DANGER = 'inline-flex items-center justify-center w-9 h-9 rounded-lg border text-base transition-colors disabled:opacity-40 border-neutral-200 dark:border-neutral-700 text-red-500 hover:border-red-300 hover:text-red-600';
  // Message d'action séparé de `error` (qui, lui, masque tout l'inventaire).
  let vmMsg = $state<{ type: 'ok' | 'err'; text: string } | null>(null);
  let vmMsgTimer: ReturnType<typeof setTimeout> | null = null;
  function showVmMsg(type: 'ok' | 'err', text: string) {
    vmMsg = { type, text };
    if (vmMsgTimer) clearTimeout(vmMsgTimer);
    vmMsgTimer = setTimeout(() => { vmMsg = null; }, 7000);
  }

  const ACTION_OK = (): Record<string, string> => ({
    start: $_('inventory.actionOkStart'), stop: $_('inventory.actionOkStop'), reboot: $_('inventory.actionOkReboot'),
    suspend: $_('inventory.actionOkSuspend'), resume: $_('inventory.actionOkResume'),
  });
  // Traduit les conflits OpenStack (409 task_state/vm_state) en messages clairs.
  function friendlyVMError(raw: string, action: string): string {
    const s = (raw || '').toLowerCase();
    if (action === 'start' && s.includes('vm_state active')) return $_('inventory.errAlreadyStarted');
    if (action === 'stop' && (s.includes('vm_state stopped') || s.includes('shutoff'))) return $_('inventory.errAlreadyStopped');
    if (s.includes('task_state') || s.includes('reboot') || s.includes('powering') || s.includes('409') || s.includes('conflict')) {
      return $_('inventory.errActionInProgress');
    }
    return $_('inventory.errActionImpossibleState');
  }

  // Confirmation avant les actions disruptives (arrêter / redémarrer).
  let confirmState = $state<{ show: boolean; title: string; message: string; confirmText: string; danger: boolean; onConfirm: () => void }>(
    { show: false, title: '', message: '', confirmText: $_('inventory.confirm'), danger: false, onConfirm: () => {} }
  );
  function requestVmAction(vm: VMInstance, action: string) {
    if (action === 'stop' || action === 'reboot') {
      const verbe = action === 'stop' ? $_('inventory.verbStop') : $_('inventory.verbReboot');
      confirmState = {
        show: true,
        title: (action === 'stop' ? $_('inventory.actionStop') : $_('inventory.actionReboot')) + $_('inventory.theMachineSuffix'),
        message: $_('inventory.confirmActionMsgPrefix') + verbe + $_('inventory.confirmActionMsgMid') + vm.name + $_('inventory.confirmActionMsgSuffix'),
        confirmText: action === 'stop' ? $_('inventory.actionStop') : $_('inventory.actionReboot'),
        danger: true,
        onConfirm: () => vmAction(vm, action),
      };
    } else {
      vmAction(vm, action);
    }
  }

  // Réinitialisation (rebuild) — destructif.
  function requestVmRebuild(vm: VMInstance) {
    confirmState = {
      show: true,
      title: $_('inventory.rebuildTitle'),
      message: $_('inventory.rebuildMsgPrefix') + vm.name + $_('inventory.rebuildMsgSuffix'),
      confirmText: $_('inventory.actionReset'),
      danger: true,
      onConfirm: () => vmRebuild(vm),
    };
  }
  async function vmRebuild(vm: VMInstance) {
    if (actingId) return;
    actingId = vm.id;
    try {
      const res = await apiFetch('/api/vm/rebuild', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: vm.id }),
      });
      if (!res.ok) {
        const e = await res.json().catch(() => ({}));
        showVmMsg('err', $_('inventory.rebuildFailedPrefix') + (e.error || `HTTP ${res.status}`));
        actingId = null;
      } else {
        showVmMsg('ok', $_('inventory.rebuildLaunched'));
        setTimeout(() => { fetchInventory(true); actingId = null; }, 2500);
      }
    } catch {
      showVmMsg('err', $_('inventory.rebuildUnreachable'));
      actingId = null;
    }
  }

  // Resize (changement de flavor) — via un petit modal de sélection.
  let resizeState = $state<{ show: boolean; vm: VMInstance | null; flavor: string }>(
    { show: false, vm: null, flavor: '' });
  const sortedFlavors = $derived(
    [...$flavors].sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' })));

  function requestVmResize(vm: VMInstance) {
    resizeState = { show: true, vm, flavor: '' };
  }
  async function vmResize() {
    const vm = resizeState.vm;
    const flavor = resizeState.flavor;
    if (!vm || !flavor || actingId) return;
    actingId = vm.id;
    resizeState = { show: false, vm: null, flavor: '' };
    try {
      const res = await apiFetch('/api/vm/resize', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: vm.id, flavor_ref: flavor }),
      });
      if (!res.ok) {
        const e = await res.json().catch(() => ({}));
        showVmMsg('err', $_('inventory.resizeFailedPrefix') + (e.error || `HTTP ${res.status}`));
        actingId = null;
      } else {
        showVmMsg('ok', $_('inventory.resizeLaunched'));
        setTimeout(() => { fetchInventory(true); actingId = null; }, 2500);
      }
    } catch {
      showVmMsg('err', $_('inventory.resizeUnreachable'));
      actingId = null;
    }
  }

  async function vmAction(vm: VMInstance, action: string) {
    if (actingId) return;
    actingId = vm.id;
    try {
      const res = await apiFetch('/api/vm/action', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: vm.id, action }),
      });
      if (!res.ok) {
        const e = await res.json().catch(() => ({}));
        showVmMsg('err', friendlyVMError(e.error || '', action));
        actingId = null;
      } else {
        showVmMsg('ok', ACTION_OK()[action] || $_('inventory.actionDone'));
        // On garde la VM verrouillée le temps de la transition, puis on rafraîchit.
        setTimeout(() => { fetchInventory(true); actingId = null; }, 2500);
      }
    } catch {
      showVmMsg('err', $_('inventory.actionUnreachable'));
      actingId = null;
    }
  }


  // Édition du libellé / des étiquettes d'un pool.
  let editingPool = $state<string | null>(null);
  let editLabel = $state('');
  let editTags = $state('');
  function startEditPool(p: InventoryPool) {
    editingPool = p.pool_id + ':' + p.user_id;
    editLabel = p.label || '';
    editTags = p.tags || '';
  }
  async function savePoolMeta(p: InventoryPool) {
    try {
      const r = await apiFetch('/api/pool/meta', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pool_id: p.pool_id, user_id: p.user_id, label: editLabel, tags: editTags }),
      });
      if (r.ok) { editingPool = null; showVmMsg('ok', $_('inventory.poolUpdated')); fetchInventory(true); }
      else showVmMsg('err', $_('inventory.poolUpdateFailed'));
    } catch { showVmMsg('err', $_('inventory.updateFailed')); }
  }

  onMount(() => {
    if (!browser) return;
    if (!$authStore || $authStore.role !== 'admin') { window.location.href = '/'; return; }
    fetchInventory();
  });

  // Auto-refresh : intervalle configurable (Paramètres). Se recrée si l'intervalle change.
  $effect(() => {
    if (!browser || !$authStore || $authStore.role !== 'admin') return;
    const ms = Math.max(3, $refreshInterval || 15) * 1000;
    const id = setInterval(() => fetchInventory(true), ms);
    return () => clearInterval(id);
  });

  function timeSince(dateStr: string): string {
    const diff = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
    if (diff < 60) return `${diff}s`;
    if (diff < 3600) return `${Math.floor(diff/60)}min`;
    if (diff < 86400) return `${Math.floor(diff/3600)}h`;
    return `${Math.floor(diff/86400)}j`;
  }

  let poolSearch = $state('');
  const filteredPools = $derived(
    poolSearch.trim()
      ? pools.filter(p => {
          const q = poolSearch.trim().toLowerCase();
          return (p.pool_id + ' ' + p.user_id + ' ' + (p.linked_course || '') + ' ' + (p.label || '') + ' ' + (p.tags || '')).toLowerCase().includes(q)
            || p.vms.some(v => (v.student || '').toLowerCase().includes(q));
        })
      : pools
  );

  const totalVMs = $derived(pools.reduce((a, p) => a + p.vms.length, 0));
  const healthyVMs = $derived(pools.reduce((a, p) => a + p.vms.filter(v => v.healthy).length, 0));
  const readyVMs = $derived(pools.reduce((a, p) => a + p.vms.filter(v => v.status === 'ready').length, 0));
  const activeVMs = $derived(pools.reduce((a, p) => a + p.vms.filter(v => v.activity_status !== 'idle').length, 0));
</script>

<svelte:head><title>{$_('inventory.pageTitle')}</title></svelte:head>

{#snippet actionButtons(vm: VMInstance)}
  {#if vm.id}
    {#if vm.power_state === 'SHUTOFF'}
      <button onclick={() => requestVmAction(vm,'start')} disabled={actingId===vm.id} title={$_('inventory.actionStart')} class={ABTN} aria-label={$_('inventory.actionStart')}>▶</button>
    {:else if vm.power_state === 'SUSPENDED' || vm.power_state === 'PAUSED'}
      <button onclick={() => requestVmAction(vm,'resume')} disabled={actingId===vm.id} title={$_('inventory.actionResume')} class={ABTN} aria-label={$_('inventory.actionResume')}>▶</button>
    {:else if vm.power_state === 'ACTIVE'}
      <button onclick={() => requestVmAction(vm,'stop')} disabled={actingId===vm.id} title={$_('inventory.actionStop')} class={ABTN} aria-label={$_('inventory.actionStop')}>⏹</button>
      <button onclick={() => requestVmAction(vm,'reboot')} disabled={actingId===vm.id} title={$_('inventory.actionReboot')} class={ABTN} aria-label={$_('inventory.actionReboot')}>↻</button>
    {:else}
      <button onclick={() => requestVmAction(vm,'start')} disabled={actingId===vm.id} title={$_('inventory.actionStart')} class={ABTN} aria-label={$_('inventory.actionStart')}>▶</button>
      <button onclick={() => requestVmAction(vm,'stop')} disabled={actingId===vm.id} title={$_('inventory.actionStop')} class={ABTN} aria-label={$_('inventory.actionStop')}>⏹</button>
    {/if}
    <button onclick={() => requestVmResize(vm)} disabled={actingId===vm.id} title={$_('inventory.resizeTitle')} class={ABTN} aria-label={$_('inventory.resize')}>⤢</button>
    <button onclick={() => requestVmRebuild(vm)} disabled={actingId===vm.id} title={$_('inventory.resetTitle')} class={ABTN_DANGER} aria-label={$_('inventory.actionReset')}>⟲</button>
  {/if}
{/snippet}

<ConfirmModal bind:show={confirmState.show} title={confirmState.title} message={confirmState.message}
  confirmText={confirmState.confirmText} danger={confirmState.danger} onConfirm={confirmState.onConfirm} />

{#if resizeState.show && resizeState.vm}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[60] flex items-center justify-center px-4">
    <div class="fixed inset-0 bg-neutral-900/40 backdrop-blur-sm" onclick={() => (resizeState = { show: false, vm: null, flavor: '' })}></div>
    <div class="relative w-full max-w-sm bg-white dark:bg-neutral-800 rounded-2xl shadow-2xl p-6">
      <h3 class="text-base font-bold text-neutral-900 dark:text-white mb-1">{$_('inventory.resizeTitle')}</h3>
      <p class="text-sm text-neutral-500 mb-4">{resizeState.vm.name}</p>
      <label class="section-label block mb-1" for="resize-flavor">{$_('inventory.resizeChooseFlavor')}</label>
      <select id="resize-flavor" bind:value={resizeState.flavor} class="field text-sm w-full mb-5">
        <option value="" disabled>—</option>
        {#each sortedFlavors as f}
          <option value={f.id}>{f.name}</option>
        {/each}
      </select>
      <div class="flex justify-end gap-2">
        <button onclick={() => (resizeState = { show: false, vm: null, flavor: '' })} class="btn btn-secondary text-sm">{$_('inventory.cancel')}</button>
        <button onclick={vmResize} disabled={!resizeState.flavor} class="btn btn-primary text-sm">{$_('inventory.resizeApply')}</button>
      </div>
    </div>
  </div>
{/if}

{#if vmMsg}
  <div class="fixed top-6 right-6 z-50 max-w-sm px-5 py-4 rounded-xl shadow-2xl text-sm font-medium flex items-start gap-3 animate-fade-in
    {vmMsg.type === 'ok' ? 'bg-green-600 text-white' : 'bg-amber-500 text-white'}">
    <svg class="w-5 h-5 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      {#if vmMsg.type === 'ok'}
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
      {:else}
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
      {/if}
    </svg>
    <span class="flex-1">{vmMsg.text}</span>
    <button onclick={() => (vmMsg = null)} class="opacity-80 hover:opacity-100 shrink-0" aria-label={$_('inventory.close')}>✕</button>
  </div>
{/if}

{#if $simpleMode}
<div class="space-y-6 animate-fade-up">
  <div class="flex items-start justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800">{$_('inventory.myStudents')}</h1>
      <p class="text-sm text-neutral-500 mt-1">{$_('inventory.myStudentsSubtitle')}</p>
    </div>
    <button onclick={() => fetchInventory(true)} disabled={refreshing} class="btn btn-secondary text-xs px-3.5 py-2">
      <svg class="w-3.5 h-3.5 {refreshing ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
      </svg>
      {$_('inventory.refresh')}
    </button>
  </div>

  {#if loading}
    <div class="flex justify-center py-20"><div class="w-8 h-8 rounded-full border-2 border-neutral-200 border-t-primary-700" style="animation: spinnerGlow 0.7s linear infinite;"></div></div>
  {:else if error}
    <div class="card px-4 py-3 border-red-200 bg-red-50 text-red-700 text-sm">{error}</div>
  {:else if pools.length === 0}
    <div class="card flex flex-col items-center justify-center py-20 text-center">
      <p class="text-neutral-500 text-sm">{$_('inventory.noActiveCourse')}</p>
    </div>
  {:else}
    {#if pools.length > 1}
      <input class="field text-sm mb-3" type="text" placeholder={$_('inventory.filterCourses')} bind:value={poolSearch} />
    {/if}
    <div class="space-y-4">
      {#each filteredPools as pool, pi}
        {@const activeVms = pool.vms.filter(v => v.activity_status !== 'idle')}
        {@const connectedStudents = pool.vms.filter(v => v.activity_status !== 'idle' && v.student)}
        {@const readyVms = pool.vms.filter(v => v.status === 'ready' && !v.is_instructor)}
        <div class="card overflow-hidden animate-fade-up" style="animation-delay:{pi*0.06}s">
          <div class="flex items-center justify-between px-5 py-4 border-b border-neutral-100">
            <div>
              <div class="flex items-center gap-2 flex-wrap">
                <h2 class="text-sm font-bold text-neutral-900">{pool.label || pool.pool_id}</h2>
                {#if pool.linked_course}
                  <span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-primary-50 text-primary-700 border border-primary-200">🎓 {pool.linked_course}</span>
                {/if}
                {#if pool.compute}
                  <span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-violet-50 text-violet-700 border border-violet-200">⚙ {$_('inventory.computeBadge')}</span>
                {/if}
                {#each tagList(pool.tags) as tag}
                  <span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-neutral-100 text-neutral-600 border border-neutral-200">{tag}</span>
                {/each}
              </div>
              <p class="text-xs text-neutral-400 mt-0.5">
                <span class="{connectedStudents.length > 0 ? 'text-green-600 font-semibold' : 'text-neutral-400'}">
                  {connectedStudents.length} {connectedStudents.length > 1 ? $_('inventory.studentsConnected') : $_('inventory.studentConnected')}
                </span>
                · {readyVms.length} {readyVms.length > 1 ? $_('inventory.machinesAvailable') : $_('inventory.machineAvailable')}
              </p>
            </div>
            <div class="flex items-center gap-1.5">
              {#if activeVms.length > 0}
                <span class="animate-ping absolute inline-flex h-2 w-2 rounded-full bg-green-400 opacity-60"></span>
                <span class="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
                <span class="text-xs text-green-600 font-semibold">{$_('inventory.inProgress')}</span>
              {:else}
                <span class="inline-flex rounded-full h-2 w-2 bg-neutral-300"></span>
                <span class="text-xs text-neutral-400">{$_('inventory.waiting')}</span>
              {/if}
            </div>
          </div>
          <div class="divide-y divide-neutral-50">
            {#each pool.vms as vm}
              {@const connected = vm.activity_status !== 'idle'}
              {@const label = vm.student ? displayName(vm.student) : connected ? $_('inventory.personalConnectionInstructor') : vm.is_instructor ? $_('inventory.instructorVmReserved') : vm.status === 'ready' ? $_('inventory.freeMachine') : $_('inventory.startingUp')}
              <div class="flex items-center justify-between gap-3 px-5 py-3 transition-colors {connected ? 'bg-green-50/70 dark:bg-green-900/10' : 'hover:bg-neutral-50 dark:hover:bg-white/[0.03]'}">
                <div class="flex items-center gap-3 min-w-0">
                  <!-- Avatar : initiale de l'étudiant, ou icône ; vert vif si connecté -->
                  <div class="relative w-9 h-9 rounded-full flex items-center justify-center text-sm font-bold shrink-0 transition-colors
                    {connected ? 'bg-green-500 text-white shadow-sm' : vm.is_instructor ? 'bg-primary-100 text-primary-600 dark:bg-primary-900/40 dark:text-primary-300' : 'bg-neutral-100 text-neutral-400 dark:bg-neutral-800'}">
                    {#if vm.student}
                      {vm.student.charAt(0).toUpperCase()}
                    {:else if connected || vm.is_instructor}
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
                    {:else}
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>
                    {/if}
                    {#if connected}
                      <span class="absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full bg-green-500 ring-2 ring-white dark:ring-[#13151f]"></span>
                    {/if}
                  </div>
                  <div class="min-w-0">
                    <p class="text-sm font-semibold truncate {connected ? 'text-neutral-900 dark:text-white' : 'text-neutral-500 dark:text-neutral-400'}">{label}</p>
                    <p class="text-[11px] text-neutral-400 font-mono truncate">{vm.name}</p>
                  </div>
                </div>
                <div class="flex items-center gap-3 shrink-0">
                  {#if connected}
                    <span class="badge badge-ready">● {$_('inventory.online')}</span>
                  {:else if vm.student}
                    <span class="text-xs text-neutral-400">{$_('inventory.offline')}</span>
                  {:else if vm.is_instructor}
                    <span class="text-xs text-neutral-400">{$_('inventory.reserved')}</span>
                  {:else if vm.status === 'ready'}
                    <span class="text-xs text-neutral-400">{$_('inventory.waiting')}</span>
                  {:else}
                    <span class="text-xs text-amber-600">{$_('inventory.startingUp')}</span>
                  {/if}
                  {#if vm.guac_url}
                    <a href={vm.guac_url} target="_blank" rel="noopener" class="inline-flex items-center gap-1.5 h-9 px-3 rounded-lg border border-neutral-200 dark:border-neutral-700 text-xs font-medium text-neutral-700 dark:text-neutral-300 hover:border-primary-300 hover:text-primary-700 transition-colors">⊳ {$_('inventory.terminal')}</a>
                  {/if}
                  {#if vm.grafana_url}
                    <a href={vm.grafana_url} target="_blank" rel="noopener" title={$_('inventory.graphs')} class="inline-flex items-center gap-1.5 h-9 px-3 rounded-lg border border-neutral-200 dark:border-neutral-700 text-xs font-medium text-neutral-700 dark:text-neutral-300 hover:border-primary-300 hover:text-primary-700 transition-colors">{$_('inventory.graphs')}</a>
                  {/if}
                  {@render actionButtons(vm)}
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
{:else}
<div class="space-y-7 animate-fade-up">

  <!-- Header -->
  <div class="flex items-start justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800">{$_('inventory.title')}</h1>
      <p class="text-sm text-neutral-500 mt-1">{$_('inventory.subtitle')}</p>
    </div>
    <div class="flex items-center gap-3">
      {#if lastRefresh}
        <span class="text-xs text-neutral-400">{$_('inventory.updatedAt')} {lastRefresh}</span>
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
        {$_('inventory.refresh')}
      </button>
    </div>
  </div>

  <!-- Stats -->
  {#if !loading && !error}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      {#each [
        { label: $_('inventory.statPools'),      value: pools.length,                   accent: 'stat-accent-indigo',  color: 'text-primary-700' },
        { label: $_('inventory.statTotalVms'),   value: totalVMs,                       accent: 'stat-accent-violet',  color: 'text-primary-500' },
        { label: $_('inventory.statHealth'),     value: `${healthyVMs}/${totalVMs}`,    accent: 'stat-accent-emerald', color: 'text-green-600'   },
        { label: $_('inventory.statActiveSsh'),  value: activeVMs,                      accent: 'stat-accent-amber',   color: 'text-amber-600'   },
      ] as stat, i}
        <div class="card card-interactive p-5 animate-fade-up" style="animation-delay:{i*0.05}s">
          <p class="section-label mb-2">{stat.label}</p>
          <p class="text-3xl font-bold {stat.color} tabular-nums tracking-tight">{stat.value}</p>
        </div>
      {/each}
    </div>
  {/if}

  <!-- Loading -->
  {#if loading}
    <div class="flex flex-col items-center justify-center py-24 gap-4">
      <div class="w-9 h-9 rounded-full border-2 border-neutral-200 border-t-primary-700" style="animation: spinnerGlow 0.7s linear infinite;"></div>
      <p class="text-sm text-neutral-500">{$_('inventory.loadingInventory')}</p>
    </div>
  {/if}

  <!-- Error -->
  {#if error}
    <div class="card px-4 py-3 border-red-200 bg-red-50 text-red-700 text-sm animate-fade-in">{error}</div>
  {/if}

  <!-- Pool sections -->
  {#if !loading && !error}
    {#if pools.length > 1}
      <input class="field text-sm mb-4" type="text" placeholder={$_('inventory.filterPools')} bind:value={poolSearch} />
    {/if}
    {#each filteredPools as pool, pi}
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
            <span class="text-sm font-bold text-neutral-900">{pool.label || pool.pool_id}</span>
            {#if pool.label}<span class="text-xs text-neutral-400 font-mono">{pool.pool_id}</span>{/if}
            <span class="text-xs text-neutral-500">{pool.user_id}</span>
            {#if pool.linked_course}
              <span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-primary-50 text-primary-700 border border-primary-200">🎓 {pool.linked_course}</span>
            {/if}
            {#if pool.compute}
              <span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-violet-50 text-violet-700 border border-violet-200">⚙ {$_('inventory.computeBadge')}</span>
            {/if}
            {#each tagList(pool.tags) as tag}
              <span class="text-[10px] font-medium px-1.5 py-0.5 rounded bg-neutral-100 text-neutral-600 border border-neutral-200">{tag}</span>
            {/each}
            <button onclick={() => startEditPool(pool)} title={$_('inventory.renameTag')} aria-label={$_('inventory.editPool')} class="text-neutral-400 hover:text-primary-700 text-xs">✎</button>
          </div>
          <span class="text-xs text-neutral-400 tabular-nums">{pool.vms.length} VM{pool.vms.length > 1 ? 's' : ''}</span>
        </div>

        {#if editingPool === pool.pool_id + ':' + pool.user_id}
          <div class="px-5 py-3 bg-neutral-50/60 border-b border-neutral-200 flex flex-wrap items-end gap-2">
            <div class="flex-1 min-w-[160px]">
              <label class="block text-[11px] text-neutral-500 mb-1">{$_('inventory.displayName')}</label>
              <input class="field text-sm" type="text" placeholder={pool.pool_id} bind:value={editLabel} />
            </div>
            <div class="flex-1 min-w-[160px]">
              <label class="block text-[11px] text-neutral-500 mb-1">{$_('inventory.tagsLabel')}</label>
              <input class="field text-sm" type="text" placeholder={$_('inventory.tagsPlaceholder')} bind:value={editTags} />
            </div>
            <button onclick={() => savePoolMeta(pool)} class="btn btn-primary text-sm">{$_('inventory.save')}</button>
            <button onclick={() => (editingPool = null)} class="btn btn-secondary text-sm">{$_('inventory.cancel')}</button>
          </div>
        {/if}

        <!-- Table -->
        <div class="overflow-x-auto">
          <table class="data-table">
            <thead>
              <tr>
                <th>{$_('inventory.colName')}</th>
                <th>{$_('inventory.colIp')}</th>
                <th>{$_('inventory.colStatus')}</th>
                <th>{$_('inventory.colHealth')}</th>
                <th>{$_('inventory.colActivity')}</th>
                <th>{$_('inventory.terminal')}</th>
                <th class="text-right">{$_('inventory.colLastActivity')}</th>
              </tr>
            </thead>
            <tbody>
              {#each pool.vms as vm}
                {@const connected = vm.activity_status !== 'idle'}
                <tr class="transition-colors {connected && vm.student ? 'bg-green-50/60 dark:bg-green-900/10' : ''}">
                  <td>
                    <div class="flex items-center gap-2.5">
                      <div class="w-7 h-7 rounded-full flex items-center justify-center text-[11px] font-bold shrink-0 transition-colors
                        {connected && vm.student ? 'bg-green-500 text-white shadow-sm' : vm.is_instructor ? 'bg-primary-100 text-primary-600 dark:bg-primary-900/40 dark:text-primary-300' : 'bg-neutral-100 text-neutral-400 dark:bg-neutral-800'}">
                        {#if vm.student}
                          {vm.student.charAt(0).toUpperCase()}
                        {:else if vm.is_instructor || connected}
                          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
                        {:else}
                          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>
                        {/if}
                      </div>
                      <div class="flex flex-col gap-0.5 min-w-0">
                        {#if vm.student}
                          <span class="text-xs font-semibold truncate {connected ? 'text-green-700 dark:text-green-400' : 'text-neutral-700 dark:text-neutral-300'}">{displayName(vm.student)}</span>
                        {:else if vm.is_instructor}
                          <span class="text-xs font-semibold text-primary-600 dark:text-primary-400">{connected ? $_('inventory.personalConnection') : $_('inventory.instructorVm')}</span>
                        {:else if connected}
                          <span class="text-xs font-semibold text-primary-600 dark:text-primary-400">{$_('inventory.personalConnection')}</span>
                        {:else}
                          <span class="text-xs text-neutral-400">{$_('inventory.freeMachine')}</span>
                        {/if}
                        <span class="font-mono text-[10px] text-neutral-400 truncate">{vm.name}</span>
                      </div>
                    </div>
                  </td>
                  <td><span class="font-mono text-xs text-neutral-700">{vm.ip}</span></td>
                  <td>
                    {#if vm.power_state}
                      {@const b = powerBadge(vm.power_state)}
                      <span class="text-xs font-semibold px-2 py-0.5 rounded-full border {b.cls}">{b.label}</span>
                    {:else}
                      <span class="badge {vm.status === 'ready' ? 'badge-ready' : vm.status === 'starting' ? 'badge-starting' : 'badge-error'}">{vm.status}</span>
                    {/if}
                  </td>
                  <td>
                    <div class="flex items-center gap-1.5">
                      <span class="w-1.5 h-1.5 rounded-full {vm.healthy ? 'bg-green-500' : 'bg-red-500'}"></span>
                      <span class="text-xs font-medium {vm.healthy ? 'text-green-700' : 'text-red-700'}">{vm.healthy ? $_('inventory.healthOk') : $_('inventory.healthKo')}</span>
                    </div>
                  </td>
                  <td>
                    {#if vm.activity_status && vm.activity_status !== 'idle'}
                      <span class="badge badge-info gap-1.5">
                        <span class="relative flex h-1.5 w-1.5">
                          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
                          <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-sky-400"></span>
                        </span>
                        {$_('inventory.onJupyter')}
                      </span>
                    {:else}
                      <span class="text-xs text-neutral-400">{$_('inventory.inactive')}</span>
                    {/if}
                  </td>
                  <td>
                    {#if vm.guac_url}
                      <a href={vm.guac_url} target="_blank" rel="noopener"
                         class="btn btn-secondary text-xs px-2 py-1 flex items-center gap-1.5 w-fit">
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                            d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
                        </svg>
                        {$_('inventory.terminal')}
                      </a>
                    {:else}
                      <span class="text-xs text-neutral-400">—</span>
                    {/if}
                    {#if vm.grafana_url}
                      <a href={vm.grafana_url} target="_blank" rel="noopener" class="btn btn-secondary text-xs px-2 py-1 mt-1 w-fit">{$_('inventory.graphs')}</a>
                    {/if}
                    <div class="flex gap-1 mt-1">{@render actionButtons(vm)}</div>
                  </td>
                  <td class="text-right">
                    <span class="text-xs text-neutral-400 tabular-nums">{$_('inventory.ago')} {timeSince(vm.last_seen)}</span>
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
        <p class="text-neutral-500 text-sm font-medium">{$_('inventory.noVmProvisioned')}</p>
        <p class="text-neutral-400 text-xs mt-1">{$_('inventory.instancesWillAppear')}</p>
      </div>
    {/if}
  {/if}
</div>
{/if}

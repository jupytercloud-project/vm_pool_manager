<script lang="ts">
import {
  rebuildServer, RebuildServerRequestSchema, CreatePoolRequestSchema,
  DeletePoolRequestSchema, deletePool, createPool, addServer, addSSHKeys,
} from '$lib/index';
import { apiFetch } from '$lib/api';
import type { ServerPool, Server, CreatePoolRequest, DeletePoolRequest, RebuildServerRequest, Image } from '$lib/type';
import { authStore, serverPools, servers, configs, images, flavors, networks } from '$lib/store';
import { simpleMode } from '$lib/store/uiStore';
import { displayName } from '$lib/displayName';
import { onMount } from 'svelte';
import { page } from '$app/state';
import { _ } from 'svelte-i18n';

// Inventory data for simple mode (more reliable than gRPC servers store)
interface InvPool { pool_id: string; user_id: string; label?: string; tags?: string; compute?: boolean; linked_course?: string; vms: { status: string; activity_status: string }[]; }
let inventoryPools: InvPool[] = $state([]);
// Nom d'affichage d'un pool (label défini dans l'inventaire, sinon l'identifiant).
const poolLabel = (name: string, user: string) =>
  inventoryPools.find(p => p.pool_id === name && p.user_id === user)?.label || name;
const poolMeta = (name: string, user: string): InvPool | undefined =>
  inventoryPools.find(p => p.pool_id === name && p.user_id === user);
const poolTags = (m?: InvPool) => (m?.tags || '').split(',').map(t => t.trim()).filter(Boolean);
async function loadInventory() {
  try {
    const res = await apiFetch('/api/inventory');
    if (res.ok) inventoryPools = await res.json();
  } catch { /* ignore */ }
}
import { create } from '@bufbuild/protobuf';
import { ListSSHPublicKeysRequestSchema, type DeletePoolResponse, type RebuildServerResponse } from '$lib/grpc/frontcontrol_pb';
import { create as createProto } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import CreateServerPoolModal from '$lib/components/CreateServerPoolModal.svelte';
import AddSSHKeys from '$lib/components/AddSSHKeys.svelte';

const token = $derived($authStore?.token ?? null);
let selectedsp: string = $state('');
let createspModal = $state(false);
let ListStudentModalOpen = $state(false);
let createError = $state('');
let createSuccess = $state(false);

let selectedNetwork = $state('');
let selectedFlavor = $state('');
let selectedConfigFile = $state('');
let scheduleDay = $state('');
let scheduleTime = $state('');
let scheduleWindowHours: number | undefined = $state(undefined);
let offDays = $state({ monday:false, tuesday:false, wednesday:false, thursday:false, friday:false, saturday:true, sunday:true });
let selectedGroupImage: string | null = $state(null);
let selectedImage: string | null = $state(null);
let appPort = $state(0);
let computeMode = $state(false);
let minVm = $state(1);
let maxVm = $state(5);

// Progression des étudiants d'un pool (A1).
interface ProgressRow { name: string; email?: string; has_vm: boolean; ip?: string; power_state?: string; activity?: string; healthy: boolean; last_active?: string; }
let progressOpen = $state(false);
let progressLoading = $state(false);
let progressRows = $state<ProgressRow[]>([]);
let progressStats = $state<{ enrolled: number; launched: number; active: number }>({ enrolled: 0, launched: 0, active: 0 });

async function loadProgress(sp: ServerPool) {
  progressLoading = true;
  try {
    const res = await apiFetch(`/api/pool/progress?pool_id=${encodeURIComponent(sp.name)}&user_id=${encodeURIComponent(sp.userId)}`);
    if (res.ok) {
      const d = await res.json();
      progressRows = d.rows ?? [];
      progressStats = { enrolled: d.enrolled ?? 0, launched: d.launched ?? 0, active: d.active ?? 0 };
    }
  } catch { /* ignore */ }
  finally { progressLoading = false; }
}
function toggleProgress(sp: ServerPool) {
  progressOpen = !progressOpen;
  if (progressOpen) loadProgress(sp);
}
function progressActivityLabel(row: ProgressRow): string {
  if (!row.has_vm) return $_('progress.notLaunched');
  if (row.activity === 'active') return $_('progress.onNotebook');
  if (row.activity === 'connected') return $_('progress.connected');
  if (row.power_state === 'SUSPENDED' || row.power_state === 'PAUSED') return $_('progress.suspended');
  if (row.power_state === 'SHUTOFF') return $_('progress.off');
  return $_('progress.idle');
}
function progressRelative(iso?: string): string {
  if (!iso) return '—';
  const d = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (d < 60) return `${d}s`;
  if (d < 3600) return `${Math.floor(d / 60)}min`;
  if (d < 86400) return `${Math.floor(d / 3600)}h`;
  return `${Math.floor(d / 86400)}j`;
}

// Diffusion d'un fichier à toutes les VMs d'un pool (A2).
let broadcastFile: File | null = $state(null);
let broadcastSubdir = $state('');
let broadcastBusy = $state(false);
let broadcastMsg = $state('');
let broadcastErr = $state(false);

async function handleBroadcastFile(sp: ServerPool) {
  if (!broadcastFile || broadcastBusy) return;
  broadcastBusy = true; broadcastMsg = ''; broadcastErr = false;
  try {
    const buf = await broadcastFile.arrayBuffer();
    // ArrayBuffer → base64 (par paquets pour éviter le dépassement de pile).
    let bin = '';
    const bytes = new Uint8Array(buf);
    for (let i = 0; i < bytes.length; i += 0x8000) {
      bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
    }
    const content_b64 = btoa(bin);
    const res = await apiFetch('/api/pool/broadcast-file', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        pool_id: sp.name, user_id: sp.userId,
        filename: broadcastFile.name, subdir: broadcastSubdir.trim(), content_b64,
      }),
    });
    const d = await res.json();
    if (!res.ok || !d.ok) {
      broadcastErr = true;
      broadcastMsg = $_('serverpool.broadcastError') + (d.error ?? `HTTP ${res.status}`);
    } else {
      broadcastErr = d.failed > 0;
      broadcastMsg = $_('serverpool.broadcastDone')
        .replace('{ok}', String(d.succeeded)).replace('{total}', String(d.total));
      broadcastFile = null;
    }
  } catch {
    broadcastErr = true;
    broadcastMsg = $_('serverpool.broadcastUnreachable');
  } finally { broadcastBusy = false; }
}

onMount(() => {
  if (!token) window.location.href = '/';
  selectedsp = page.params.id || '';
  loadInventory();
});

let selectedPool = $derived($serverPools.find(p => p.name === selectedsp));
let selMeta = $derived(selectedPool ? poolMeta(selectedPool.name, selectedPool.userId) : undefined);
let sortedFlavors = $derived([...$flavors].sort((a, b) => a.name.localeCompare(b.name, undefined, {numeric:true, sensitivity:'base'})));

async function handleDeleteServerpool(sp: ServerPool) {
  if (!confirm($_('serverpool.confirmDeletePool') + ' ' + sp.name + ' ?')) return;
  const req: DeletePoolRequest = create(DeletePoolRequestSchema, { user: $authStore?.email, poolId: sp.name });
  try {
    const res: DeletePoolResponse = await deletePool(req);
    if (res.success) {
      selectedsp = '';
      const { loadServerPools } = await import('$lib/store/serverpoolStore');
      await loadServerPools($authStore?.email ?? '');
    }
  } catch(e) { console.error(e); }
}

async function handleCreateServer(sp: ServerPool) {
  if (!confirm($_('serverpool.confirmAddServer') + ' ' + sp.name + ' ?')) return;
  const req: CreatePoolRequest = create(CreatePoolRequestSchema, {
    user: $authStore?.email, name: sp.name, image: sp.image, flavor: sp.flavor,
    network: sp.network, minVm: String(sp.minVm), maxVm: String(sp.maxVm), config: sp.config,
  });
  try { await addServer(req); } catch(e) { console.error(e); }
}

// Clone (template) : crée un nouveau pool en réutilisant tous les paramètres d'un pool
// existant (image, flavor, réseau, config, off_days, port…), avec un nouveau nom.
async function clonePool(sp: ServerPool) {
  const suggested = sp.name + '-copie';
  const newName = typeof window !== 'undefined'
    ? window.prompt($_('serverpool.clonePrompt'), suggested)
    : null;
  if (!newName || !newName.trim()) return;
  const req: CreatePoolRequest = create(CreatePoolRequestSchema, {
    user: $authStore?.email ?? '', name: newName.trim(), image: sp.image, flavor: sp.flavor,
    network: sp.network, minVm: String(sp.minVm), maxVm: String(sp.maxVm), config: sp.config,
    metadata: sp.metadata ?? {}, appPort: sp.appPort ?? 0,
  });
  try {
    const res = await createPool(req);
    if (res.success) {
      const { loadServerPools } = await import('$lib/store/serverpoolStore');
      await loadServerPools($authStore?.email ?? '');
      selectedsp = newName.trim();
    }
  } catch (e) { console.error(e); }
}

export function getUniqueFirstAlphaBlocks(imgs: Image[]): string[] {
  const prefixes = imgs.map(img => { const m = img.name.match(/^[A-Za-z]+/); return m ? m[0] : null; }).filter((x): x is string => x !== null);
  return Array.from(new Set(prefixes));
}
export function filterImagesByPrefix(imgs: Image[], prefix: string): Image[] {
  return imgs.filter(img => img.name.startsWith(prefix));
}

type CreateServerPoolForm = { name:string; image:string; flavor:string; networks:string; minVm:number; maxVm:number; config:string; };

async function handleCreateServerpool(event: Event) {
  event.preventDefault();
  const form = event.target as HTMLFormElement;
  const fd = new FormData(form);
  const data: CreateServerPoolForm = {
    name: fd.get('namesp') as string, image: selectedImage ?? '',
    flavor: selectedFlavor, networks: selectedNetwork,
    minVm: Number(fd.get('min_vm')), maxVm: Number(fd.get('max_vm')), config: selectedConfigFile,
  };
  if (!data.name?.trim()) { createError = $_('serverpool.errorNameRequired'); return; }
  if (!data.image || !data.flavor || !data.networks) { createError = $_('serverpool.errorImageFlavorNetworkRequired'); return; }

  const enabledOffDays = Object.entries(offDays).filter(([,v]) => v).map(([k]) => k);
  const hasSchedule = Boolean(scheduleDay && scheduleTime);
  if ((scheduleDay && !scheduleTime) || (!scheduleDay && scheduleTime)) {
    createError = $_('serverpool.errorScheduleDayTime'); return;
  }
  const metadata: Record<string, string> = {};
  if (enabledOffDays.length > 0) metadata.off_days = enabledOffDays.join(',');
  if (computeMode) metadata.compute = 'true';
  const req: CreatePoolRequest = create(CreatePoolRequestSchema, {
    user: $authStore?.email ?? '', name: data.name, image: data.image,
    flavor: data.flavor, network: data.networks, minVm: String(data.minVm), maxVm: String(data.maxVm),
    config: data.config ?? '', metadata,
    timeWindow: 0, appPort: appPort > 0 ? appPort : 0,
  });
  if (hasSchedule) {
    const startDate = computeNextSchedule(Number(scheduleDay), scheduleTime);
    req.startTime = createProto(TimestampSchema, { seconds: BigInt(Math.floor(startDate.getTime()/1000)), nanos: (startDate.getTime()%1000)*1_000_000 });
    if (scheduleWindowHours != null && scheduleWindowHours > 0) req.timeWindow = scheduleWindowHours;
  }
  try {
    createError = '';
    const res = await createPool(req);
    if (res.success) {
      createSuccess = true;
      const { loadServerPools } = await import('$lib/store/serverpoolStore');
      await loadServerPools($authStore?.email ?? '');
      setTimeout(() => { createspModal = false; createSuccess = false; }, 1200);
    } else { createError = $_('serverpool.errorCreationFailed'); }
  } catch { createError = $_('serverpool.errorCannotCreate'); }
}

function computeNextSchedule(dayOfWeek: number, time: string): Date {
  const [hours, minutes] = time.split(':').map(Number);
  const now = new Date();
  const target = new Date(now);
  target.setHours(hours, minutes, 0, 0);
  let delta = dayOfWeek - now.getDay();
  if (delta < 0 || (delta === 0 && target < now)) delta += 7;
  target.setDate(now.getDate() + delta);
  return target;
}
</script>

{#if $simpleMode}
<div class="space-y-6 animate-fade-up">

  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800">{$_('serverpool.myCoursesTitle')}</h1>
      <p class="text-sm text-neutral-500 mt-1">{$_('serverpool.myCoursesSubtitle')}</p>
    </div>
    <button onclick={() => createspModal = true} class="btn btn-primary">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
      </svg>
      {$_('serverpool.createCourse')}
    </button>
  </div>

  {#if $serverPools.length === 0}
    <div class="card flex flex-col items-center justify-center py-20 text-center">
      <svg class="w-12 h-12 text-neutral-300 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"/>
      </svg>
      <p class="text-neutral-600 font-medium">{$_('serverpool.noCourse')}</p>
      <p class="text-neutral-400 text-sm mt-1">{$_('serverpool.noCourseHint')}</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each $serverPools as sp}
        {@const invPool = inventoryPools.find(p => p.pool_id === sp.name && p.user_id === sp.userId)}
        {@const spVMs = invPool?.vms ?? []}
        {@const activeCount = spVMs.filter(v => v.activity_status !== 'idle').length}
        {@const readyCount = spVMs.filter(v => v.status === 'ready').length}
        <div class="card card-interactive p-5 space-y-4 hover:border-primary-200">
          <div class="flex items-start justify-between">
            <div>
              <h2 class="text-base font-bold text-neutral-900">{invPool?.label || sp.name}</h2>
              <p class="text-xs text-neutral-400 mt-0.5">
                {#if activeCount > 0}
                  <span class="text-green-600 font-semibold">{activeCount} {activeCount > 1 ? $_('serverpool.studentsConnected') : $_('serverpool.studentConnected')}</span>
                  {#if readyCount > activeCount} · {readyCount - activeCount} {$_('serverpool.waiting')}{/if}
                {:else if readyCount > 0}
                  {readyCount} {readyCount > 1 ? $_('serverpool.machinesReady') : $_('serverpool.machineReady')}
                {:else}
                  {$_('serverpool.noMachineStarted')}
                {/if}
              </p>
            </div>
            <span class="badge {activeCount > 0 ? 'badge-ready' : readyCount > 0 ? 'badge-starting' : 'badge-info'}">
              {activeCount > 0 ? $_('serverpool.statusRunning') : readyCount > 0 ? $_('serverpool.statusReady') : $_('serverpool.statusStopped')}
            </span>
          </div>

          <div class="flex items-center gap-1.5">
            {#each {length: Math.min(sp.maxVm, 12)} as _, i}
              <div class="h-2 flex-1 rounded-full {i < activeCount ? 'bg-green-400' : i < readyCount ? 'bg-primary-200' : 'bg-neutral-200'}"></div>
            {/each}
          </div>
          <p class="text-xs text-neutral-400 -mt-2">{activeCount} / {sp.maxVm} {$_('serverpool.placesUsed')}</p>

          <div class="flex gap-2 pt-1">
            <button
              onclick={() => { selectedsp = sp.name; ListStudentModalOpen = true; }}
              class="btn btn-primary text-xs flex-1"
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"/>
              </svg>
              {$_('serverpool.students')}
            </button>
            <button
              onclick={() => handleCreateServer(sp)}
              class="btn btn-success text-xs"
              title={$_('serverpool.startMachineTitle')}
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
              </svg>
              {$_('serverpool.start')}
            </button>
            <button
              onclick={() => clonePool(sp)}
              class="btn btn-secondary text-xs px-2.5"
              title={$_('serverpool.cloneTitle')}
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2"/>
              </svg>
            </button>
            <button
              onclick={() => handleDeleteServerpool(sp)}
              class="btn btn-danger text-xs px-2.5"
              title={$_('serverpool.deleteCourseTitle')}
            >
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
{:else}
<div class="space-y-6 animate-fade-up">

  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800">{$_('serverpool.serverpoolsTitle')}</h1>
      <p class="text-sm text-neutral-500 mt-1">{$_('serverpool.serverpoolsSubtitle')}</p>
    </div>
    <button onclick={() => createspModal = true} class="btn btn-primary">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
      </svg>
      {$_('serverpool.newServerpool')}
    </button>
  </div>

  <div class="flex gap-6">
    <!-- Sidebar pool list -->
    <div class="w-56 shrink-0 space-y-1">
      {#each $serverPools as sp}
        <button
          onclick={() => selectedsp = sp.name}
          class="w-full text-left px-3.5 py-2.5 rounded text-sm font-medium transition-all duration-150
            {selectedsp === sp.name
              ? 'bg-primary-50 text-primary-700 border border-primary-200'
              : 'text-neutral-600 hover:text-primary-700 hover:bg-primary-50 border border-transparent'}"
        >
          <div class="flex items-center gap-2.5">
            <span class="w-1.5 h-1.5 rounded-full {selectedsp === sp.name ? 'bg-primary-600' : 'bg-neutral-300'}"></span>
            {poolLabel(sp.name, sp.userId)}
          </div>
        </button>
      {/each}

      {#if $serverPools.length === 0}
        <p class="text-xs text-neutral-400 px-3 py-2">{$_('serverpool.noServerpool')}</p>
      {/if}
    </div>

    <!-- Detail panel -->
    <div class="flex-1 min-w-0">
      {#if selectedPool}
        <div class="card p-6 space-y-6 animate-fade-in">

          <!-- Pool name + range -->
          <div class="flex items-start justify-between">
            <div class="min-w-0">
              <h2 class="text-xl font-bold text-neutral-900">{poolLabel(selectedPool.name, selectedPool.userId)}</h2>
              <p class="text-xs text-neutral-400 mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
                {#if poolLabel(selectedPool.name, selectedPool.userId) !== selectedPool.name}
                  <span class="font-mono">{selectedPool.name}</span>
                  <span class="text-neutral-300">·</span>
                {/if}
                <span>{selectedPool.userId}</span>
              </p>
              {#if selMeta?.linked_course || selMeta?.compute || poolTags(selMeta).length}
                <div class="flex flex-wrap items-center gap-1.5 mt-2">
                  {#if selMeta?.linked_course}
                    <span class="text-[11px] font-medium px-2 py-0.5 rounded bg-primary-50 text-primary-700 border border-primary-200">🎓 {selMeta.linked_course}</span>
                  {/if}
                  {#if selMeta?.compute}
                    <span class="text-[11px] font-medium px-2 py-0.5 rounded bg-violet-50 text-violet-700 border border-violet-200">⚙ {$_('inventory.computeBadge')}</span>
                  {/if}
                  {#each poolTags(selMeta) as tag}
                    <span class="text-[11px] font-medium px-2 py-0.5 rounded bg-neutral-100 text-neutral-600 border border-neutral-200">{tag}</span>
                  {/each}
                </div>
              {/if}
            </div>
            <div class="card-elevated px-4 py-2.5 text-center">
              <p class="section-label mb-1">{$_('serverpool.vmTarget')}</p>
              <p class="text-xl font-bold text-primary-700 tabular-nums">{selectedPool.minVm} – {selectedPool.maxVm}</p>
            </div>
          </div>

          <hr class="divider"/>

          <!-- Properties -->
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {#each [
              { label: 'Flavor', icon: 'M13 10V3L4 14h7v7l9-11h-7z', value: $flavors.find(f => f.id === selectedPool?.flavor)?.name ?? selectedPool?.flavor },
              { label: 'Image', icon: 'M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7', value: $images.find(i => i.id === selectedPool?.image)?.name ?? selectedPool?.image },
              { label: $_('serverpool.network'), icon: 'M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9', value: $networks.find(n => n.id === selectedPool?.network)?.name ?? selectedPool?.network },
            ] as prop}
              <div class="card-elevated px-4 py-3 hover:border-primary-200 transition-colors">
                <div class="flex items-center gap-2 mb-2">
                  <svg class="w-3.5 h-3.5 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={prop.icon}/>
                  </svg>
                  <p class="section-label">{prop.label}</p>
                </div>
                <p class="text-sm font-semibold text-neutral-800 truncate">{prop.value}</p>
              </div>
            {/each}
          </div>

          <hr class="divider"/>

          <!-- Actions -->
          <div class="flex flex-wrap gap-3">
            <button onclick={() => handleCreateServer(selectedPool)} class="btn btn-success text-sm">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
              </svg>
              {$_('serverpool.addServer')}
            </button>
            <button onclick={() => ListStudentModalOpen = true} class="btn btn-secondary text-sm">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"/>
              </svg>
              {$_('serverpool.students')}
            </button>
            <button onclick={() => clonePool(selectedPool)} class="btn btn-secondary text-sm">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2"/>
              </svg>
              {$_('serverpool.clone')}
            </button>
            <div class="flex-1"></div>
            <button onclick={() => handleDeleteServerpool(selectedPool)} class="btn btn-danger text-sm">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
              {$_('serverpool.delete')}
            </button>
          </div>

          <hr class="divider"/>

          <!-- Pousser un fichier à toutes les VMs du pool (A2) -->
          <div>
            <div class="flex items-center gap-2 mb-2">
              <svg class="w-4 h-4 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/>
              </svg>
              <p class="section-label">{$_('serverpool.broadcastTitle')}</p>
            </div>
            <p class="text-xs text-neutral-400 mb-3">{$_('serverpool.broadcastHint')}</p>
            <div class="space-y-2.5">
              <div class="flex flex-wrap items-center gap-3">
                <label class="btn btn-secondary text-sm cursor-pointer inline-flex items-center gap-2">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"/></svg>
                  {$_('serverpool.broadcastChoose')}
                  <input type="file" class="hidden" onchange={(e) => { broadcastFile = (e.currentTarget.files?.[0] ?? null); broadcastMsg = ''; }} />
                </label>
                <span class="text-sm truncate max-w-[18rem] {broadcastFile ? 'text-neutral-700 dark:text-neutral-200 font-medium' : 'text-neutral-400'}">
                  {broadcastFile?.name ?? $_('serverpool.broadcastNoFile')}
                </span>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <input type="text" bind:value={broadcastSubdir} placeholder={$_('serverpool.broadcastSubdir')}
                  class="field text-sm w-48 py-1.5" />
                <button onclick={() => handleBroadcastFile(selectedPool)}
                  disabled={!broadcastFile || broadcastBusy} class="btn btn-primary text-sm inline-flex items-center gap-2">
                  {#if broadcastBusy}<span class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full inline-block" style="animation: spinnerGlow 0.6s linear infinite;"></span>{/if}
                  {$_('serverpool.broadcastSend')}
                </button>
              </div>
            </div>
            {#if broadcastMsg}
              <p class="text-xs mt-2 {broadcastErr ? 'text-red-600' : 'text-green-600'}">{broadcastMsg}</p>
            {/if}
          </div>

          <hr class="divider"/>

          <!-- Progression des étudiants en direct (A1) -->
          <div>
            <div class="flex items-center justify-between mb-2">
              <div class="flex items-center gap-2">
                <svg class="w-4 h-4 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/>
                </svg>
                <p class="section-label">{$_('progress.title')}</p>
              </div>
              <button onclick={() => toggleProgress(selectedPool)} class="btn btn-secondary text-xs">
                {progressOpen ? $_('progress.hide') : $_('progress.show')}
              </button>
            </div>

            {#if progressOpen}
              <div class="flex flex-wrap gap-4 mb-3 text-sm">
                <span class="text-neutral-500">{$_('progress.enrolled')} : <b class="text-neutral-800 dark:text-neutral-200">{progressStats.enrolled}</b></span>
                <span class="text-neutral-500">{$_('progress.launched')} : <b class="text-primary-700">{progressStats.launched}</b></span>
                <span class="text-neutral-500">{$_('progress.activeNow')} : <b class="text-green-600">{progressStats.active}</b></span>
                <button onclick={() => loadProgress(selectedPool)} disabled={progressLoading} class="btn btn-secondary text-xs">{$_('progress.refresh')}</button>
              </div>
              {#if progressLoading && progressRows.length === 0}
                <p class="text-sm text-neutral-400 py-3">{$_('progress.loading')}</p>
              {:else if progressRows.length === 0}
                <p class="text-sm text-neutral-400 py-3">{$_('progress.empty')}</p>
              {:else}
                <div class="overflow-hidden rounded-xl border border-neutral-200 dark:border-neutral-700">
                  <table class="w-full text-sm">
                    <thead class="bg-neutral-50 dark:bg-neutral-800/50 text-left text-xs text-neutral-500">
                      <tr>
                        <th class="px-3 py-2 font-semibold">{$_('progress.colStudent')}</th>
                        <th class="px-3 py-2 font-semibold">{$_('progress.colState')}</th>
                        <th class="px-3 py-2 font-semibold text-right">{$_('progress.colLastActivity')}</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-neutral-100 dark:divide-neutral-800">
                      {#each progressRows as row}
                        <tr>
                          <td class="px-3 py-2">
                            <span class="font-medium text-neutral-800 dark:text-neutral-200">{displayName(row.name)}</span>
                            {#if row.email}<span class="text-xs text-neutral-400 ml-1">{row.email}</span>{/if}
                          </td>
                          <td class="px-3 py-2">
                            <span class="inline-flex items-center gap-1.5">
                              <span class="w-2 h-2 rounded-full {row.activity === 'active' ? 'bg-green-500' : row.activity === 'connected' ? 'bg-sky-500' : row.has_vm ? 'bg-neutral-300' : 'bg-neutral-200'}"></span>
                              {progressActivityLabel(row)}
                            </span>
                          </td>
                          <td class="px-3 py-2 text-right text-xs text-neutral-400">{row.has_vm ? progressRelative(row.last_active) : '—'}</td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            {/if}
          </div>
        </div>

      {:else}
        <div class="card flex flex-col items-center justify-center py-24 text-center">
          <svg class="w-12 h-12 text-neutral-300 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
              d="M5 12h14M12 5l7 7-7 7"/>
          </svg>
          <p class="text-neutral-600 font-medium">{$_('serverpool.noPoolSelected')}</p>
          <p class="text-neutral-400 text-sm mt-1 max-w-xs">{$_('serverpool.noPoolSelectedHint')}</p>
        </div>
      {/if}
    </div>
  </div>
</div>
{/if}

{#if createspModal}
  <CreateServerPoolModal
    bind:open={createspModal}
    images={$images} flavors={sortedFlavors} networks={$networks} configs={$configs}
    bind:selectedGroupImage bind:selectedImage bind:selectedFlavor bind:selectedNetwork
    bind:selectedConfigFile bind:scheduleDay bind:scheduleTime bind:scheduleWindowHours bind:offDays
    bind:appPort bind:computeMode bind:minVm bind:maxVm {createError} {createSuccess}
    {handleCreateServerpool} {getUniqueFirstAlphaBlocks} {filterImagesByPrefix}
  />
{/if}

{#if ListStudentModalOpen && selectedPool}
  <AddSSHKeys bind:open={ListStudentModalOpen} poolname={selectedPool.name} />
{/if}

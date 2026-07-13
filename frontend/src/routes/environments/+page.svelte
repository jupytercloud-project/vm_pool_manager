<script lang="ts">
import {
  CreatePoolRequestSchema, DeletePoolRequestSchema,
  deletePool, createPool, addServer,
} from '$lib/index';
import { apiFetch } from '$lib/api';
import type { ServerPool, Server, CreatePoolRequest, DeletePoolRequest, Image } from '$lib/type';
import { authStore, serverPools, servers, configs, images, flavors, networks } from '$lib/store';
import { openProxySession, openInNewTab } from '$lib/proxy';
import { onMount } from 'svelte';
import { _ } from 'svelte-i18n';
import { create } from '@bufbuild/protobuf';
import { create as createProto } from '@bufbuild/protobuf';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import CreateServerPoolModal from '$lib/components/CreateServerPoolModal.svelte';

const token = $derived($authStore?.token ?? null);
const email = $derived($authStore?.email ?? '');

// ---- Création d'environnement (réutilise le modal, défauts « calcul ») ----
let createspModal = $state(false);
let createError = $state('');
let createSuccess = $state(false);
let selectedNetwork = $state('');
let selectedFlavor = $state('');
let selectedConfigFile = $state('');
let scheduleDay = $state('');
let scheduleTime = $state('');
let scheduleWindowHours: number | undefined = $state(undefined);
let offDays = $state({ monday:false, tuesday:false, wednesday:false, thursday:false, friday:false, saturday:false, sunday:false });
let selectedGroupImage: string | null = $state(null);
let selectedImage: string | null = $state(null);
let appPort = $state(0);
let computeMode = $state(true); // par défaut : environnement de calcul (SSH/terminal, sans Jupyter)
let minVm = $state(1);
let maxVm = $state(2);

let sortedFlavors = $derived([...$flavors].sort((a, b) => a.name.localeCompare(b.name, undefined, {numeric:true, sensitivity:'base'})));

// ---- Helpers d'affichage ----
const flavorOf = (id: string) => $flavors.find(f => f.id === id);
const imageOf = (id: string) => $images.find(i => i.id === id);
function flavorLabel(id: string): string {
  const f = flavorOf(id);
  if (!f) return id;
  return `${f.name} · ${f.vcpus} vCPU · ${Math.round((f.ram ?? 0)/1024)} Go`;
}
// GPU : heuristique sur les extra_specs OpenStack (pci_passthrough / vgpu / resources:VGPU / gpu).
function flavorHasGPU(id: string): boolean {
  const f = flavorOf(id);
  const es = (f?.extraSpecs ?? '').toLowerCase();
  return /gpu|pci_passthrough|vgpu|nvidia/.test(es);
}

// Rattache une machine à un environnement : metadata.serverpool_id sinon préfixe de nom.
function poolOfServer(s: Server): string {
  const meta = (s.metadata ?? {}) as Record<string,string>;
  if (meta.serverpool_id) return meta.serverpool_id;
  const hit = $serverPools.find(p => s.name?.startsWith(p.name + '-') || s.name === p.name);
  return hit?.name ?? '';
}
const serversOfPool = (poolName: string) => $servers.filter(s => poolOfServer(s) === poolName);

function statusBadge(status: string): string {
  const s = (status || '').toLowerCase();
  if (s.includes('active') || s === 'ready' || s.includes('running')) return 'badge-ready';
  if (s.includes('build') || s.includes('start')) return 'badge-starting';
  if (s.includes('shutoff') || s.includes('stop') || s.includes('suspend')) return 'badge-info';
  return 'badge-info';
}

// ---- Actions machines ----
let busyServer = $state('');       // id de la VM en cours d'action
let actionErr = $state('');
async function reloadServers() {
  const { loadServers } = await import('$lib/store/serverpoolStore');
  await loadServers(email);
}
async function vmAction(s: Server, action: 'start'|'stop'|'suspend'|'resume'|'reboot') {
  busyServer = s.id; actionErr = '';
  try {
    const res = await apiFetch('/api/vm/action', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server_id: s.id, action }),
    });
    if (!res.ok) { actionErr = `${s.name}: ${(await res.text()) || res.status}`; }
    else { await reloadServers(); }
  } catch (e: any) { actionErr = `${s.name}: ${e?.message || 'action'}`; }
  finally { busyServer = ''; }
}

// Terminal navigateur (Guacamole) sur SA VM.
let termErr = $state('');
async function openTerminal(s: Server) {
  termErr = '';
  const ip = s.ipAddress || s.addressedIp;
  if (!ip) { termErr = `${s.name}: ${$_('environments.noIp')}`; return; }
  try {
    const res = await apiFetch(`/api/guac-url?ip=${encodeURIComponent(ip)}`);
    const d = await res.json().catch(() => ({}));
    if (res.ok && d.url) openInNewTab(d.url);
    else termErr = `${s.name}: ${$_('environments.terminalUnavailable')}`;
  } catch { termErr = `${s.name}: ${$_('environments.terminalUnavailable')}`; }
}

// VS Code / Jupyter (environnements non-calcul) sur SA VM.
let openingApp = $state('');
async function openApp(kind: 'vscode'|'jupyter', poolName: string) {
  openingApp = poolName + kind; termErr = '';
  try {
    const { url } = await openProxySession(kind, poolName, email, 'self', 'write');
    openInNewTab(url);
  } catch (e: any) { termErr = e?.message || kind; }
  finally { openingApp = ''; }
}

let copied = $state('');
function copySsh(ip: string) {
  const cmd = `ssh vmuser@${ip}`;
  navigator.clipboard?.writeText(cmd);
  copied = ip; setTimeout(() => { if (copied === ip) copied = ''; }, 1500);
}

// ---- Environnements (pools) ----
async function handleAddServer(sp: ServerPool) {
  const req: CreatePoolRequest = create(CreatePoolRequestSchema, {
    user: email, name: sp.name, image: sp.image, flavor: sp.flavor,
    network: sp.network, minVm: String(sp.minVm), maxVm: String(sp.maxVm), config: sp.config,
  });
  try { await addServer(req); setTimeout(reloadServers, 1500); } catch(e) { console.error(e); }
}
async function handleDelete(sp: ServerPool) {
  if (!confirm($_('environments.confirmDelete') + ' ' + sp.name + ' ?')) return;
  const req: DeletePoolRequest = create(DeletePoolRequestSchema, { user: email, poolId: sp.name });
  try {
    const res = await deletePool(req);
    if (res.success) {
      const { loadServerPools, loadServers } = await import('$lib/store/serverpoolStore');
      await loadServerPools(email); await loadServers(email);
    }
  } catch(e) { console.error(e); }
}

export function getUniqueFirstAlphaBlocks(imgs: Image[]): string[] {
  const prefixes = imgs.map(img => { const m = img.name.match(/^[A-Za-z]+/); return m ? m[0] : null; }).filter((x): x is string => x !== null);
  return Array.from(new Set(prefixes));
}
export function filterImagesByPrefix(imgs: Image[], prefix: string): Image[] {
  return imgs.filter(img => img.name.startsWith(prefix));
}

async function handleCreateServerpool(event: Event) {
  event.preventDefault();
  const form = event.target as HTMLFormElement;
  const fd = new FormData(form);
  const name = (fd.get('namesp') as string || '').trim();
  if (!name) { createError = $_('serverpool.errorNameRequired'); return; }
  if (!selectedImage || !selectedFlavor || !selectedNetwork) { createError = $_('serverpool.errorImageFlavorNetworkRequired'); return; }
  const enabledOffDays = Object.entries(offDays).filter(([,v]) => v).map(([k]) => k);
  const metadata: Record<string, string> = {};
  if (enabledOffDays.length > 0) metadata.off_days = enabledOffDays.join(',');
  if (computeMode) metadata.compute = 'true';
  const req: CreatePoolRequest = create(CreatePoolRequestSchema, {
    user: email, name, image: selectedImage, flavor: selectedFlavor, network: selectedNetwork,
    minVm: String(minVm), maxVm: String(maxVm), config: selectedConfigFile ?? '',
    metadata, timeWindow: 0, appPort: appPort > 0 ? appPort : 0,
  });
  const hasSchedule = Boolean(scheduleDay && scheduleTime);
  if (hasSchedule) {
    const [h, m] = scheduleTime.split(':').map(Number);
    const now = new Date(); const t = new Date(now); t.setHours(h, m, 0, 0);
    let delta = Number(scheduleDay) - now.getDay(); if (delta < 0 || (delta === 0 && t < now)) delta += 7;
    t.setDate(now.getDate() + delta);
    req.startTime = createProto(TimestampSchema, { seconds: BigInt(Math.floor(t.getTime()/1000)), nanos: 0 });
    if (scheduleWindowHours != null && scheduleWindowHours > 0) req.timeWindow = scheduleWindowHours;
  }
  try {
    createError = '';
    const res = await createPool(req);
    if (res.success) {
      createSuccess = true;
      const { loadServerPools } = await import('$lib/store/serverpoolStore');
      await loadServerPools(email);
      setTimeout(() => { createspModal = false; createSuccess = false; }, 1200);
    } else { createError = $_('serverpool.errorCreationFailed'); }
  } catch { createError = $_('serverpool.errorCannotCreate'); }
}

onMount(async () => {
  if (!token) { window.location.href = '/'; return; }
  if (email) {
    const { loadAll } = await import('$lib/store/serverpoolStore');
    loadAll(email);
  }
});
</script>

<div class="space-y-8 animate-fade-up">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800">{$_('environments.title')}</h1>
      <p class="text-sm text-neutral-500 mt-1">{$_('environments.subtitle')}</p>
    </div>
    <button onclick={() => createspModal = true} class="btn btn-primary">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
      {$_('environments.newEnv')}
    </button>
  </div>

  {#if actionErr}<p class="text-sm text-red-600">{actionErr}</p>{/if}
  {#if termErr}<p class="text-sm text-red-600">{termErr}</p>{/if}

  {#if $serverPools.length === 0}
    <div class="card flex flex-col items-center justify-center py-20 text-center">
      <svg class="w-12 h-12 text-neutral-300 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z"/></svg>
      <p class="text-neutral-600 font-medium">{$_('environments.empty')}</p>
      <p class="text-neutral-400 text-sm mt-1 max-w-md">{$_('environments.emptyHint')}</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
      {#each $serverPools as sp}
        {@const vms = serversOfPool(sp.name)}
        {@const isCompute = (sp.metadata?.compute === 'true')}
        <div class="card p-5 space-y-4">
          <!-- Env header -->
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="text-base font-bold text-neutral-900 truncate">{sp.name}</h2>
              <div class="flex flex-wrap items-center gap-1.5 mt-1.5">
                <span class="text-[11px] font-medium px-2 py-0.5 rounded {isCompute ? 'bg-violet-50 text-violet-700 border border-violet-200' : 'bg-primary-50 text-primary-700 border border-primary-200'}">
                  {isCompute ? $_('environments.computeBadge') : $_('environments.jupyterBadge')}
                </span>
                {#if flavorHasGPU(sp.flavor)}
                  <span class="text-[11px] font-semibold px-2 py-0.5 rounded bg-emerald-50 text-emerald-700 border border-emerald-200">GPU</span>
                {/if}
                <span class="text-[11px] text-neutral-500 px-2 py-0.5 rounded bg-neutral-100 border border-neutral-200">{flavorLabel(sp.flavor)}</span>
              </div>
              <p class="text-xs text-neutral-400 mt-1.5 truncate">{imageOf(sp.image)?.name ?? sp.image}</p>
            </div>
            <div class="text-center shrink-0">
              <p class="section-label mb-0.5">{$_('environments.machines')}</p>
              <p class="text-lg font-bold text-primary-700 tabular-nums">{vms.length}<span class="text-neutral-300 text-sm"> / {sp.maxVm}</span></p>
            </div>
          </div>

          <!-- Machines de l'environnement -->
          {#if vms.length > 0}
            <div class="space-y-2">
              {#each vms as s}
                {@const ip = s.ipAddress || s.addressedIp}
                {@const off = (s.status||'').toLowerCase().match(/shutoff|stop|suspend/)}
                <div class="rounded-xl border border-neutral-200 dark:border-neutral-700 p-3 flex flex-wrap items-center gap-x-3 gap-y-2">
                  <span class="badge {statusBadge(s.status)} shrink-0">{s.status || '—'}</span>
                  <span class="font-mono text-xs text-neutral-700 dark:text-neutral-300 truncate">{s.name}</span>
                  {#if ip}
                    <button onclick={() => copySsh(ip)} title="ssh vmuser@{ip}"
                      class="font-mono text-xs text-primary-700 hover:underline inline-flex items-center gap-1">
                      {ip}
                      {#if copied === ip}<span class="text-green-600 text-[10px]">✓ {$_('environments.copied')}</span>
                      {:else}<svg class="w-3 h-3 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>{/if}
                    </button>
                  {/if}
                  <div class="flex-1"></div>
                  <!-- Power -->
                  {#if off}
                    <button onclick={() => vmAction(s, 'start')} disabled={busyServer === s.id} class="btn btn-success text-xs px-2.5 py-1" title={$_('environments.start')}>▶ {$_('environments.start')}</button>
                  {:else}
                    <button onclick={() => vmAction(s, 'stop')} disabled={busyServer === s.id} class="btn btn-secondary text-xs px-2.5 py-1" title={$_('environments.stop')}>⏸ {$_('environments.stop')}</button>
                  {/if}
                  <!-- Accès -->
                  <button onclick={() => openTerminal(s)} class="btn btn-secondary text-xs px-2.5 py-1" title={$_('environments.terminal')}>
                    <svg class="w-3.5 h-3.5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>
                    {$_('environments.terminal')}
                  </button>
                  {#if !isCompute}
                    <button onclick={() => openApp('jupyter', sp.name)} disabled={openingApp === sp.name+'jupyter'} class="btn btn-secondary text-xs px-2.5 py-1">Jupyter</button>
                    <button onclick={() => openApp('vscode', sp.name)} disabled={openingApp === sp.name+'vscode'} class="btn btn-secondary text-xs px-2.5 py-1">VS Code</button>
                  {/if}
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-xs text-neutral-400 py-2">{$_('environments.noMachine')}</p>
          {/if}

          <!-- Env actions -->
          <div class="flex flex-wrap gap-2 pt-1">
            <button onclick={() => handleAddServer(sp)} class="btn btn-success text-xs">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
              {$_('environments.addMachine')}
            </button>
            <a href="/jobs" class="btn btn-secondary text-xs">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
              {$_('environments.runJob')}
            </a>
            <div class="flex-1"></div>
            <button onclick={() => handleDelete(sp)} class="btn btn-danger text-xs px-2.5"
              title={$_('environments.confirmDelete')} aria-label={$_('environments.confirmDelete')}>
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

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

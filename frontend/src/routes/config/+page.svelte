<script lang="ts">
  import { createConfig, updateConfig, deleteConfig } from '$lib/index';
  import { authStore, configs } from '$lib/store';
  import { onMount } from 'svelte';
  import type { Config } from '$lib/type';

  let config_name = '';
  let editorVisible = false;
  let text = '';
  let newconfigname = '';
  const token = $derived($authStore?.token ?? null);

  onMount(() => { if (!token) window.location.href = '/'; });

  function selectConfig(cfg: Config) {
    config_name = cfg.name;
    text = cfg.data || '';
    newconfigname = cfg.name;
    editorVisible = true;
  }

  function newConfig() {
    config_name = '';
    text = '';
    newconfigname = '';
    editorVisible = true;
  }

  async function handleCreate() {
    await createConfig($authStore?.email ?? '', newconfigname, text);
    config_name = newconfigname;
  }
  async function handleUpdate() { await updateConfig($authStore?.email ?? '', newconfigname, text); }
  async function handleDelete() {
    await deleteConfig($authStore?.email ?? '', newconfigname);
    config_name = ''; text = ''; newconfigname = ''; editorVisible = false;
  }

  const isNew = $derived(config_name !== newconfigname || config_name === '');
</script>

<svelte:head><title>Configurations — CloudPoolManager</title></svelte:head>

<div class="space-y-6 animate-fade-up">

  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold text-primary-800" style="font-family: 'Source Sans 3', sans-serif;">Configurations</h1>
      <p class="text-sm text-neutral-500 mt-1">Scripts cloud-init pour l'initialisation des VMs</p>
    </div>
    <button onclick={newConfig} class="btn btn-primary">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
      </svg>
      Nouvelle configuration
    </button>
  </div>

  <div class="flex gap-6">
    <!-- Sidebar -->
    <div class="w-56 shrink-0 space-y-1">
      {#each $configs as cfg}
        <button
          onclick={() => selectConfig(cfg)}
          class="w-full text-left px-3.5 py-2.5 rounded text-sm font-medium transition-all duration-150
            {config_name === cfg.name && editorVisible
              ? 'bg-primary-50 text-primary-700 border border-primary-200'
              : 'text-neutral-600 hover:text-primary-700 hover:bg-primary-50 border border-transparent'}"
        >
          <div class="flex items-center gap-2.5">
            <svg class="w-3.5 h-3.5 {config_name === cfg.name ? 'text-primary-600' : 'text-neutral-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
            </svg>
            <span class="truncate">{cfg.name}</span>
          </div>
        </button>
      {/each}
      {#if $configs.length === 0}
        <p class="text-xs text-neutral-400 px-3 py-2">Aucune configuration</p>
      {/if}
    </div>

    <!-- Editor -->
    <div class="flex-1 min-w-0">
      {#if editorVisible}
        <div class="card p-6 space-y-5 animate-fade-in">
          <div>
            <label class="section-label mb-2 block">Nom de la configuration</label>
            <input
              class="field"
              type="text"
              placeholder="ex: setup_python_env"
              bind:value={newconfigname}
            />
          </div>

          <div>
            <label class="section-label mb-2 block">Script bash (cloud-init)</label>
            <textarea
              class="field font-mono text-xs resize-none leading-relaxed"
              placeholder="#!/bin/bash&#10;apt-get update..."
              rows={20}
              bind:value={text}
            ></textarea>
          </div>

          <hr class="divider"/>

          <div class="flex items-center gap-3">
            {#if isNew}
              <button
                onclick={handleCreate}
                disabled={!newconfigname.trim()}
                class="btn btn-primary text-sm"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"/>
                </svg>
                Enregistrer
              </button>
            {:else}
              <button onclick={handleUpdate} class="btn btn-success text-sm">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                </svg>
                Mettre à jour
              </button>
            {/if}
            <div class="flex-1"></div>
            {#if !isNew}
              <button onclick={handleDelete} class="btn btn-danger text-sm">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                </svg>
                Supprimer
              </button>
            {/if}
          </div>
        </div>
      {:else}
        <div class="card flex flex-col items-center justify-center py-24 text-center">
          <svg class="w-12 h-12 text-neutral-300 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
          </svg>
          <p class="text-neutral-600 text-sm font-medium">Aucune configuration sélectionnée</p>
          <p class="text-neutral-400 text-xs mt-1 max-w-xs">Sélectionnez un script dans la liste ou créez-en un nouveau</p>
        </div>
      {/if}
    </div>
  </div>
</div>

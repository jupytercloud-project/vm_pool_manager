<script lang="ts">
  import '../app.css';
  import favicon from '$lib/assets/favicon.svg';
  import logoX from '$lib/assets/logo_polytechnique_crop.png';
  import {
    loadAll, logout, resetAll, subscribeUserUpdate,
  } from '$lib/index'
  import { authStore, startOIDCLogin } from '$lib/store/authStore';
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { browser } from '$app/environment';
  import { page } from '$app/state';

  let { children } = $props();
  let userStreamController: AbortController | null = null;

  let previousEmail: string | null = null;

  authStore.subscribe(async (auth) => {
    if (!browser) return;
    if (userStreamController) { userStreamController.abort(); userStreamController = null; }
    if (auth?.email) {
      userStreamController = new AbortController();
      subscribeUserUpdate(auth.email, userStreamController.signal);
      if (auth.email !== previousEmail) {
        previousEmail = auth.email;
        await loadAll(auth.email);
      }
    } else {
      previousEmail = null;
      resetAll();
    }
  });

  onMount(async () => {
    if (!browser) return;
    const token = get(authStore);
    if (!token) resetAll();
  });

  let mobileOpen = $state(false);

  const navLinks = $derived(() => {
    const auth = $authStore;
    const links = [{ href: '/', label: 'Accueil' }];
    if (auth?.role === 'admin') {
      links.push({ href: '/inventory', label: 'Inventaire' });
      links.push({ href: '/serverpool', label: 'Serverpools' });
      links.push({ href: '/config', label: 'Configurations' });
    }
    if (auth) links.push({ href: '/profile', label: 'Profil' });
    return links;
  });

  function isActive(href: string): boolean {
    if (!browser) return false;
    const p = page.url?.pathname ?? '';
    return href === '/' ? p === '/' : p.startsWith(href);
  }
</script>

<svelte:head>
  <link rel="icon" href={favicon} />
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous">
  <link href="https://fonts.googleapis.com/css2?family=Source+Sans+3:ital,wght@0,300;0,400;0,600;0,700;1,400&display=swap" rel="stylesheet">
</svelte:head>

<div class="min-h-screen flex flex-col" style="background: #f8f9fa; font-family: 'Source Sans 3', 'Source Sans Pro', system-ui, sans-serif; color: #212529;">

  <!-- Barre bleue Polytechnique -->
  <div class="nav-stripe"></div>

  <!-- Nav -->
  <nav class="glass-nav sticky top-0 z-30 w-full">
    <div class="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">

      <!-- Brand -->
      <a href="/" class="flex items-center gap-4 shrink-0">
        <img src={logoX} class="h-10 w-auto" alt="École Polytechnique" />
        <div class="hidden sm:block w-px h-7 bg-neutral-300"></div>
        <div class="hidden sm:flex flex-col leading-tight">
          <span class="text-[10px] font-700 text-neutral-500 tracking-widest uppercase" style="letter-spacing: 0.12em;">Infrastructure</span>
          <span class="text-sm font-semibold text-primary-700 tracking-tight">CloudPoolManager</span>
        </div>
        <span class="sm:hidden text-sm font-semibold text-primary-700">CloudPoolManager</span>
      </a>

      <!-- Desktop links -->
      <div class="hidden md:flex items-center gap-1">
        {#each navLinks() as link}
          <a
            href={link.href}
            class="px-4 py-2 text-sm font-600 transition-all duration-150 relative rounded
              {isActive(link.href)
                ? 'text-primary-700 bg-primary-50'
                : 'text-neutral-600 hover:text-primary-700 hover:bg-primary-50'}"
            style="font-weight: {isActive(link.href) ? '700' : '600'};"
          >
            {link.label}
            {#if isActive(link.href)}
              <span class="absolute bottom-0 left-2 right-2 h-0.5 bg-primary-700 rounded-full"></span>
            {/if}
          </a>
        {/each}
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-2.5">
        {#if $authStore}
          {#if $authStore.role === 'admin'}
            <span class="badge badge-admin hidden sm:inline-flex">Admin</span>
          {/if}
          <button onclick={logout} class="btn btn-secondary text-xs px-3.5 py-2">Déconnexion</button>
        {:else}
          <button onclick={startOIDCLogin} class="btn btn-primary text-xs px-3.5 py-2">Se connecter</button>
        {/if}

        <!-- Hamburger -->
        <button
          onclick={() => mobileOpen = !mobileOpen}
          class="md:hidden p-1.5 rounded text-neutral-500 hover:text-primary-700 hover:bg-primary-50 transition-colors"
          aria-label="Menu">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            {#if mobileOpen}
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            {:else}
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/>
            {/if}
          </svg>
        </button>
      </div>
    </div>

    <!-- Mobile menu -->
    {#if mobileOpen}
      <div class="md:hidden border-t border-neutral-200 py-2 px-4 space-y-0.5 animate-fade-in bg-white">
        {#each navLinks() as link}
          <a
            href={link.href}
            onclick={() => mobileOpen = false}
            class="block px-4 py-2.5 text-sm font-semibold transition-colors rounded
              {isActive(link.href)
                ? 'text-primary-700 bg-primary-50 border-l-2 border-primary-700'
                : 'text-neutral-600 hover:text-primary-700 hover:bg-primary-50 border-l-2 border-transparent'}"
          >{link.label}</a>
        {/each}
      </div>
    {/if}
  </nav>

  <!-- Main -->
  <main class="flex-1 max-w-7xl w-full mx-auto px-6 pt-8 pb-16">
    {@render children?.()}
  </main>

  <footer class="border-t border-neutral-200 bg-white">
    <div class="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
      <span class="text-xs text-neutral-400 tracking-wide">CloudPoolManager — IDCS Infrastructure</span>
      <span class="text-xs text-neutral-400 tracking-wide">École Polytechnique · Institut Polytechnique de Paris</span>
    </div>
  </footer>
</div>


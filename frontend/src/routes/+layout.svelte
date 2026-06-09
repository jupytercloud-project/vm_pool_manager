<script lang="ts">
  import '../app.css';
  import favicon from '$lib/assets/favicon.svg';
  import logoX from '$lib/assets/logo_polytechnique_crop.png';
  import {
    loadAll, logout, resetAll, subscribeUserUpdate,
  } from '$lib/index'
  import { authStore, startOIDCLogin } from '$lib/store/authStore';
  import { githubStore, disconnectGitHub } from '$lib/store/githubStore';
  import { moodleStudentStore, disconnectMoodleStudent } from '$lib/store/moodleStudentStore';
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { goto } from '$app/navigation';
  import { browser } from '$app/environment';
  import { page } from '$app/state';
  import { simpleMode, darkMode } from '$lib/store/uiStore';

  let { children } = $props();
  let userStreamController: AbortController | null = null;

  let previousEmail: string | null = null;

  const LOGIN_ROUTE = '/';
  const PUBLIC_ROUTES = ['/', '/auth/callback', '/student'];

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
      const path = page.url?.pathname ?? '';
      if (!PUBLIC_ROUTES.some(r => path.startsWith(r))) {
        goto(LOGIN_ROUTE);
      }
    }
  });

  onMount(async () => {
    if (!browser) return;
    // Force canonical URL — redirect to 10.202.3.109 if accessed via another IP (e.g. Colima 169.254.x.x)
    const h = window.location.hostname;
    if (h !== '10.202.3.109' && h !== 'localhost' && h !== '127.0.0.1' && !h.startsWith('192.168.')) {
      window.location.href = 'https://10.202.3.109' + window.location.pathname;
      return;
    }
    try {
      const mr = await fetch('/api/moodle/status');
      if (mr.ok) { const md = await mr.json(); if (md.configured) moodleUrl = md.url ?? ''; }
    } catch { /* ignore */ }
    const token = get(authStore);
    if (!token) {
      const path = page.url?.pathname ?? '';
      if (!PUBLIC_ROUTES.some(r => path.startsWith(r))) {
        goto(LOGIN_ROUTE);
      }
    }
  });

  $effect(() => {
    if (!browser) return;
    if ($darkMode) {
      document.documentElement.classList.add('dark');
      document.documentElement.style.setProperty('--page-bg', '#0f1117');
      document.documentElement.style.setProperty('--page-color', '#e9ecef');
    } else {
      document.documentElement.classList.remove('dark');
      document.documentElement.style.setProperty('--page-bg', '#f8f9fa');
      document.documentElement.style.setProperty('--page-color', '#212529');
    }
  });

  let mobileOpen = $state(false);
  let moodleUrl = $state('');

  const navLinks = $derived(() => {
    const auth = $authStore;
    const simple = $simpleMode;
    const links: { href: string; label: string; secondary?: boolean }[] = [];
    if (auth?.role === 'admin') {
      // '/serverpool' is the admin home — no separate "Accueil" tab (it pointed
      // to the same page). In simple mode it's labelled "Mes cours".
      links.push({ href: '/inventory', label: simple ? 'Mes étudiants' : 'Inventaire' });
      links.push({ href: '/serverpool', label: simple ? 'Mes cours' : 'Serverpools' });
      links.push({ href: '/grading', label: 'Notation' });
      // Secondaires : regroupées dans un menu "Plus" pour désencombrer la barre.
      if (!simple) links.push({ href: '/config', label: 'Configurations', secondary: true });
      links.push({ href: '/propose-image', label: 'Proposer une image', secondary: true });
    } else if (auth) {
      links.push({ href: '/student', label: 'Mes cours' });
    }
    if (auth) links.push({ href: '/profile', label: 'Profil' });
    return links;
  });
  const primaryNav = $derived(navLinks().filter(l => !l.secondary));
  const secondaryNav = $derived(navLinks().filter(l => l.secondary));
  let moreOpen = $state(false);

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

<div class="min-h-screen flex flex-col" style="background: var(--page-bg, #f8f9fa); font-family: 'Source Sans 3', 'Source Sans Pro', system-ui, sans-serif; color: var(--page-color, #212529);">

{#if page.url?.pathname !== '/'}
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
        {#each primaryNav as link}
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

        {#if secondaryNav.length || (moodleUrl && $authStore?.role === 'admin')}
          <div class="relative">
            <button
              onclick={() => moreOpen = !moreOpen}
              onblur={() => setTimeout(() => moreOpen = false, 150)}
              class="px-4 py-2 text-sm font-600 rounded text-neutral-600 hover:text-primary-700 hover:bg-primary-50 transition-all inline-flex items-center gap-1"
            >
              Plus
              <svg class="w-3.5 h-3.5 transition-transform {moreOpen ? 'rotate-180' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
            </button>
            {#if moreOpen}
              <div class="absolute right-0 mt-1 w-52 py-1.5 rounded-lg border border-neutral-200 dark:border-neutral-700 bg-white dark:bg-neutral-900 shadow-lg z-50">
                {#each secondaryNav as link}
                  <a href={link.href} class="block px-4 py-2 text-sm text-neutral-700 dark:text-neutral-300 hover:bg-primary-50 dark:hover:bg-neutral-800 {isActive(link.href) ? 'text-primary-700 font-semibold' : ''}">
                    {link.label}
                  </a>
                {/each}
                {#if moodleUrl && $authStore?.role === 'admin'}
                  <a href={moodleUrl} target="_blank" rel="noopener noreferrer" class="block px-4 py-2 text-sm text-neutral-700 dark:text-neutral-300 hover:bg-[#f98012]/10 inline-flex items-center gap-1">
                    Ouvrir Moodle ↗
                  </a>
                {/if}
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-2.5">
        <!-- Dark mode toggle -->
        <button
          onclick={() => darkMode.update(v => !v)}
          class="p-1.5 rounded text-neutral-500 hover:text-primary-700 hover:bg-primary-50 dark:text-neutral-400 dark:hover:text-primary-300 dark:hover:bg-neutral-800 transition-colors"
          title={$darkMode ? 'Mode clair' : 'Mode sombre'}
        >
          {#if $darkMode}
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364-6.364l-.707.707M6.343 17.657l-.707.707M17.657 17.657l-.707-.707M6.343 6.343l-.707-.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/>
            </svg>
          {:else}
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/>
            </svg>
          {/if}
        </button>

        {#if $authStore?.role === 'admin'}
          <!-- Simple/Expert mode toggle -->
          <button
            onclick={() => simpleMode.update(v => !v)}
            class="hidden sm:flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-semibold transition-all border
              {$simpleMode
                ? 'bg-amber-50 text-amber-700 border-amber-200 hover:bg-amber-100 dark:bg-amber-900/30 dark:text-amber-300 dark:border-amber-700'
                : 'bg-neutral-50 text-neutral-600 border-neutral-200 hover:bg-neutral-100 dark:bg-neutral-800 dark:text-neutral-300 dark:border-neutral-600'}"
            title={$simpleMode ? 'Passer en mode expert' : 'Passer en mode simple'}
          >
            {#if $simpleMode}
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"/>
              </svg>
              Mode simple
            {:else}
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
              </svg>
              Mode expert
            {/if}
          </button>
        {/if}

        {#if $authStore}
          {#if $authStore.role === 'admin'}
            <span class="badge badge-admin hidden sm:inline-flex">Admin</span>
          {/if}
          <button onclick={logout} class="btn btn-secondary text-xs px-3.5 py-2">Déconnexion</button>
        {:else if $githubStore}
          <span class="hidden sm:flex items-center gap-1.5 text-xs text-neutral-500">
            <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/></svg>
            <span class="font-mono font-semibold text-neutral-700">{$githubStore.login}</span>
          </span>
          <button onclick={disconnectGitHub} class="btn btn-secondary text-xs px-3.5 py-2">Déconnexion</button>
        {:else if $moodleStudentStore}
          <span class="hidden sm:flex items-center gap-1.5 text-xs text-neutral-500">
            <svg class="w-3.5 h-3.5 text-[#f98012]" fill="currentColor" viewBox="0 0 24 24"><path d="M12 3 1 9l4 2.18v6L12 21l7-3.82v-6l2-1.09V17h2V9L12 3z"/></svg>
            <span class="font-mono font-semibold text-neutral-700">{$moodleStudentStore.email}</span>
          </span>
          <button onclick={disconnectMoodleStudent} class="btn btn-secondary text-xs px-3.5 py-2">Déconnexion</button>
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
      <div class="md:hidden border-t border-neutral-200 dark:border-neutral-800 py-2 px-4 space-y-0.5 animate-fade-in bg-white dark:bg-[#13151f]">
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

  <footer class="border-t border-neutral-200 dark:border-neutral-800 bg-white dark:bg-[#13151f]">
    <div class="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
      <span class="text-xs text-neutral-400 tracking-wide">CloudPoolManager — IDCS Infrastructure</span>
      <span class="text-xs text-neutral-400 tracking-wide">École Polytechnique · Institut Polytechnique de Paris</span>
    </div>
  </footer>

{:else}
  <!-- Login page — plein écran sans nav -->
  {@render children?.()}
{/if}

</div>


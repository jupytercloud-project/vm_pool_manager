<script lang="ts">
  import '../app.css';
  import { apiFetch } from '$lib/api';
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
  import { simpleMode, darkMode, reduceMotion, language } from '$lib/store/uiStore';
  import { meStore, loadMe, resetMe } from '$lib/store/meStore';
  import NotificationBell from '$lib/components/NotificationBell.svelte';
  import AnnouncementButton from '$lib/components/AnnouncementButton.svelte';
  import CommandPalette from '$lib/components/CommandPalette.svelte';
  import '$lib/i18n';
  import { locale, _ } from 'svelte-i18n';

  let { children } = $props();
  let userStreamController: AbortController | null = null;

  // Annonce globale (bandeau affiché à tous).
  let announcement = $state<{ message: string; active: boolean } | null>(null);
  let announceDismissed = $state(false);

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
        loadMe();
        await loadAll(auth.email);
      }
    } else {
      previousEmail = null;
      resetMe();
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
      const mr = await apiFetch('/api/moodle/status');
      if (mr.ok) { const md = await mr.json(); if (md.configured) moodleUrl = md.url ?? ''; }
    } catch { /* ignore */ }
    try {
      const ar = await apiFetch('/api/announcement');
      if (ar.ok) announcement = await ar.json();
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
      document.documentElement.style.setProperty('--page-bg', '#fbfbfd');
      document.documentElement.style.setProperty('--page-color', '#1d1d1f');
    }
  });

  $effect(() => {
    if (!browser) return;
    document.documentElement.classList.toggle('reduce-motion', $reduceMotion);
  });

  // Synchronise la langue de l'interface (store persistant → svelte-i18n + <html lang>).
  $effect(() => {
    locale.set($language);
    if (browser) document.documentElement.lang = $language;
  });

  let mobileOpen = $state(false);
  let moodleUrl = $state('');

  // Icônes (path d, style heroicons outline) par route.
  const ICONS: Record<string, string> = {
    '/inventory': 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01',
    '/serverpool': 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-12-2h.01M7 16h.01',
    '/grading': 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4',
    '/config': 'M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4',
    '/propose-image': 'M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z',
    '/student': 'M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z',
    '/profile': 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z',
    '/usage': 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
    '/jobs': 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
  };

  const navLinks = $derived(() => {
    const auth = $authStore;
    const me = $meStore;
    const simple = $simpleMode;
    // Rôle effectif : /api/me (rôles fins en base) avec repli sur le JWT le temps du chargement.
    const isAdmin = me?.is_admin ?? (auth?.role === 'admin');
    const isStaff = me?.is_staff ?? (auth?.role === 'admin');
    const role = me?.role ?? auth?.role;
    const links: { href: string; label: string; secondary?: boolean }[] = [];
    if (isStaff) {
      // Équipe pédagogique (admin / prof / ta).
      if (isAdmin) links.push({ href: '/inventory', label: simple ? $_('nav.myStudents') : $_('nav.inventory') });
      links.push({ href: '/serverpool', label: simple ? $_('nav.myCourses') : $_('nav.serverpools') });
      links.push({ href: '/grading', label: $_('nav.grading') });
      if (!simple && isAdmin) links.push({ href: '/config', label: $_('nav.configs'), secondary: true });
      if (isAdmin) links.push({ href: '/usage', label: $_('nav.costs'), secondary: true });
      links.push({ href: '/jobs', label: $_('nav.jobs'), secondary: true });
      if (isAdmin) links.push({ href: '/propose-image', label: $_('nav.proposeImage'), secondary: true });
    } else if (role === 'chercheur') {
      // Chercheur : gère ses propres environnements de calcul.
      links.push({ href: '/serverpool', label: $_('nav.myEnvironments') });
      links.push({ href: '/jobs', label: $_('nav.jobs') });
    } else if (auth) {
      links.push({ href: '/student', label: $_('nav.myCourses') });
    }
    // 'Profil' a été déplacé dans la page Paramètres (accessible via l'icône ⚙).
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

<div class="min-h-screen flex flex-col" style="background: var(--page-bg, #fbfbfd); font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Inter', system-ui, sans-serif; color: var(--page-color, #1d1d1f);">

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
      <div class="hidden md:flex items-center gap-0.5 bg-neutral-100/70 dark:bg-neutral-800/50 rounded-full p-1 border border-neutral-200/60 dark:border-neutral-700/60">
        {#each primaryNav as link}
          <a
            href={link.href}
            class="px-3.5 py-1.5 text-sm rounded-full transition-all duration-150 inline-flex items-center gap-2
              {isActive(link.href)
                ? 'bg-white dark:bg-neutral-900 text-primary-700 dark:text-primary-300 shadow-sm font-semibold'
                : 'text-neutral-500 dark:text-neutral-400 hover:text-primary-700 dark:hover:text-primary-300'}"
          >
            {#if ICONS[link.href]}
              <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d={ICONS[link.href]}/></svg>
            {/if}
            {link.label}
          </a>
        {/each}

        {#if secondaryNav.length || (moodleUrl && $authStore?.role === 'admin')}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="relative" onmouseleave={() => moreOpen = false}>
            <button
              onclick={() => moreOpen = !moreOpen}
              class="px-3.5 py-1.5 text-sm rounded-full transition-all inline-flex items-center gap-1.5
                {moreOpen || secondaryNav.some(l => isActive(l.href))
                  ? 'bg-white dark:bg-neutral-900 text-primary-700 dark:text-primary-300 shadow-sm font-semibold'
                  : 'text-neutral-500 dark:text-neutral-400 hover:text-primary-700 dark:hover:text-primary-300'}"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M4 6h16M4 12h16M4 18h16"/></svg>
              {$_('nav.more')}
            </button>
            {#if moreOpen}
              <!-- top-full + pt-2 = pont invisible : pas de trou entre le bouton et le menu,
                   pour que onmouseleave ne se déclenche qu'en sortant vraiment de la zone. -->
              <div class="absolute right-0 top-full pt-2 z-50 origin-top-right">
                <div class="glass-menu w-60 p-1.5 rounded-2xl animate-fade-in">
                  {#each secondaryNav as link}
                    <a href={link.href} onclick={() => moreOpen = false} class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm transition-colors {isActive(link.href) ? 'bg-primary-50/80 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300 font-semibold' : 'text-neutral-700 dark:text-neutral-300 hover:bg-black/5 dark:hover:bg-white/5'}">
                      {#if ICONS[link.href]}
                        <svg class="w-4 h-4 shrink-0 text-neutral-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d={ICONS[link.href]}/></svg>
                      {/if}
                      {link.label}
                    </a>
                  {/each}
                  {#if moodleUrl && $authStore?.role === 'admin'}
                    <div class="my-1 border-t border-black/5 dark:border-white/10"></div>
                    <a href={moodleUrl} target="_blank" rel="noopener noreferrer" onclick={() => moreOpen = false} class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm text-neutral-700 dark:text-neutral-300 hover:bg-[#f98012]/10">
                      <svg class="w-4 h-4 shrink-0 text-[#f98012]" fill="currentColor" viewBox="0 0 24 24"><path d="M12 3 1 9l4 2.18v6L12 21l7-3.82v-6l2-1.09V17h2V9L12 3z"/></svg>
                      {$_('nav.openMoodle')}
                      <span class="ml-auto text-[10px] text-neutral-400">↗</span>
                    </a>
                  {/if}
                </div>
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-2">
        <CommandPalette />
        <button onclick={() => window.dispatchEvent(new CustomEvent('open-command-palette'))}
          title={$_('common.search') + ' (⌘K)'} aria-label={$_('common.search')}
          class="hidden sm:flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs text-neutral-400 border border-neutral-200 dark:border-neutral-700 hover:text-primary-700 hover:border-primary-300 transition-colors">
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-4.35-4.35M11 18a7 7 0 100-14 7 7 0 000 14z"/></svg>
          <span class="font-mono">⌘K</span>
        </button>
        {#if ($meStore?.is_admin ?? ($authStore?.role === 'admin'))}
          <AnnouncementButton />
          <NotificationBell />
        {/if}
        <!-- Paramètres : lien direct vers la page dédiée (admin: Paramètres ; étudiant: Mon compte) -->
        {#if $authStore || $githubStore || $moodleStudentStore}
          {@const settingsHref = $authStore ? '/profile' : '/student/settings'}
          <a
            href={settingsHref} title={$_('common.settings')} aria-label={$_('common.settings')}
            class="p-2 rounded-full transition-colors {isActive(settingsHref) ? 'text-primary-700 dark:text-primary-300 bg-black/5 dark:bg-white/10' : 'text-neutral-500 dark:text-neutral-400 hover:text-primary-700 dark:hover:text-primary-300 hover:bg-black/5 dark:hover:bg-white/5'}"
          >
            <svg class="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M10.5 6h9.75M10.5 6a1.5 1.5 0 11-3 0m3 0a1.5 1.5 0 10-3 0M3.75 6H7.5m3 12h9.75m-9.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-3.75 0H7.5m9-6h3.75m-3.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-9.75 0h9.75"/></svg>
          </a>
        {/if}

        {#if $authStore}
          <button onclick={logout} class="btn btn-secondary text-xs px-3.5 py-2">{$_('common.logout')}</button>
        {:else if $githubStore}
          <span class="hidden sm:flex items-center gap-1.5 text-xs text-neutral-500">
            <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/></svg>
            <span class="font-mono font-semibold text-neutral-700">{$githubStore.login}</span>
          </span>
          <button onclick={disconnectGitHub} class="btn btn-secondary text-xs px-3.5 py-2">{$_('common.logout')}</button>
        {:else if $moodleStudentStore}
          <span class="hidden sm:flex items-center gap-1.5 text-xs text-neutral-500">
            <svg class="w-3.5 h-3.5 text-[#f98012]" fill="currentColor" viewBox="0 0 24 24"><path d="M12 3 1 9l4 2.18v6L12 21l7-3.82v-6l2-1.09V17h2V9L12 3z"/></svg>
            <span class="font-mono font-semibold text-neutral-700">{$moodleStudentStore.email}</span>
          </span>
          <button onclick={disconnectMoodleStudent} class="btn btn-secondary text-xs px-3.5 py-2">{$_('common.logout')}</button>
        {:else}
          <button onclick={startOIDCLogin} class="btn btn-primary text-xs px-3.5 py-2">{$_('common.login')}</button>
        {/if}

        <!-- Hamburger -->
        <button
          onclick={() => mobileOpen = !mobileOpen}
          class="md:hidden p-1.5 rounded text-neutral-500 hover:text-primary-700 hover:bg-primary-50 transition-colors"
          aria-label={$_('common.menu')}>
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
            class="block px-4 py-2.5 text-sm font-semibold transition-colors rounded-xl
              {isActive(link.href)
                ? 'text-primary-700 bg-primary-50 dark:bg-primary-900/30 dark:text-primary-300'
                : 'text-neutral-600 dark:text-neutral-300 hover:text-primary-700 hover:bg-primary-50 dark:hover:bg-white/5'}"
          >{link.label}</a>
        {/each}
      </div>
    {/if}
  </nav>

  <!-- Main -->
  <main class="flex-1 max-w-7xl w-full mx-auto px-6 pt-8 pb-16">
    {#if announcement?.active && announcement.message && !announceDismissed}
      <div class="mb-6 px-4 py-3 rounded-xl border border-amber-300 bg-amber-50 text-amber-900 text-sm flex items-start gap-3 dark:bg-amber-900/20 dark:border-amber-700 dark:text-amber-200">
        <svg class="w-5 h-5 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z"/></svg>
        <span class="flex-1 font-medium">{announcement.message}</span>
        <button onclick={() => (announceDismissed = true)} class="opacity-60 hover:opacity-100 shrink-0" aria-label={$_('common.close')}>✕</button>
      </div>
    {/if}
    {@render children?.()}
  </main>

  <footer class="border-t border-neutral-200 dark:border-neutral-800 bg-white dark:bg-[#13151f]">
    <div class="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
      <span class="text-xs text-neutral-400 tracking-wide">{$_('footer.infra')}</span>
      <span class="text-xs text-neutral-400 tracking-wide">{$_('footer.school')}</span>
    </div>
  </footer>

{:else}
  <!-- Login page — plein écran sans nav -->
  {@render children?.()}
{/if}

</div>


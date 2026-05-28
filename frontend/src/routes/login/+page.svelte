<script lang="ts">
  import { goto } from '$app/navigation';
  import { authStore, startOIDCLogin } from '$lib/store/authStore';
  import logoX from '$lib/assets/logo_polytechnique_crop.png';
  import { browser } from '$app/environment';

  // If already logged in, redirect away
  if (browser && $authStore) {
    goto($authStore.role === 'admin' ? '/serverpool' : '/');
  }
</script>

<svelte:head><title>Connexion — CloudPoolManager</title></svelte:head>

<div class="min-h-screen flex flex-col" style="background: #f8f9fa;">

  <!-- Barre bleue Polytechnique -->
  <div style="height:4px; background:#003865; flex-shrink:0;"></div>

  <!-- Header institutionnel -->
  <header class="bg-white border-b border-neutral-200" style="box-shadow: 0 1px 3px rgba(0,0,0,0.06);">
    <div class="max-w-7xl mx-auto px-6 h-16 flex items-center gap-4">
      <img src={logoX} class="h-10 w-auto" alt="École Polytechnique" />
      <div class="w-px h-7 bg-neutral-300"></div>
      <div class="flex flex-col leading-tight">
        <span class="text-[10px] font-700 text-neutral-500 tracking-widest uppercase" style="letter-spacing:0.12em;">Infrastructure</span>
        <span class="text-sm font-semibold text-primary-700 tracking-tight">CloudPoolManager</span>
      </div>
    </div>
  </header>

  <!-- Corps centré -->
  <main class="flex-1 flex items-center justify-center px-4 py-12">
    <div class="w-full max-w-md">

      <!-- Carte principale -->
      <div class="bg-white border border-neutral-200 rounded-lg overflow-hidden animate-fade-up"
           style="box-shadow: 0 4px 24px rgba(0,56,101,0.10); border-top: 3px solid #003865;">

        <!-- En-tête de carte -->
        <div class="px-8 pt-8 pb-6 text-center border-b border-neutral-100">
          <div class="w-14 h-14 rounded-full bg-primary-50 border border-primary-100 flex items-center justify-center mx-auto mb-4">
            <svg class="w-7 h-7 text-primary-700" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8"
                d="M5 12h14M12 5l7 7-7 7"/>
            </svg>
          </div>
          <h1 class="text-xl font-bold text-neutral-900" style="font-family:'Source Sans 3',sans-serif;">
            Accès à la plateforme
          </h1>
          <p class="text-sm text-neutral-500 mt-1">
            Connectez-vous avec vos identifiants Polytechnique
          </p>
        </div>

        <!-- Corps de carte -->
        <div class="px-8 py-7 space-y-4">

          <!-- Bouton OIDC principal -->
          <button
            onclick={startOIDCLogin}
            class="w-full flex items-center gap-3 px-5 py-3.5 rounded font-semibold text-sm transition-all
              bg-primary-700 hover:bg-primary-600 text-white"
            style="box-shadow: 0 2px 8px rgba(0,56,101,0.20);"
          >
            <svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/>
            </svg>
            <span class="flex-1 text-left">Se connecter avec SSO Polytechnique</span>
            <svg class="w-4 h-4 shrink-0 opacity-70" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
            </svg>
          </button>

          <!-- Séparateur -->
          <div class="flex items-center gap-3">
            <div class="flex-1 h-px bg-neutral-200"></div>
            <span class="text-xs text-neutral-400 font-medium">ou</span>
            <div class="flex-1 h-px bg-neutral-200"></div>
          </div>

          <!-- GitHub login -->
          <a
            href="/api/github/login"
            class="w-full flex items-center gap-3 px-5 py-3 rounded font-semibold text-sm transition-all
              bg-neutral-900 hover:bg-neutral-700 text-white"
          >
            <svg class="w-5 h-5 shrink-0" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/>
            </svg>
            <span class="flex-1 text-left">Se connecter avec GitHub</span>
            <span class="text-xs text-white/50 font-normal">portail étudiant</span>
          </a>

          <!-- Portail étudiant sans compte -->
          <a
            href="/"
            class="w-full flex items-center gap-3 px-5 py-3 rounded font-semibold text-sm transition-all
              bg-white border border-neutral-300 text-neutral-700 hover:bg-neutral-50 hover:border-neutral-400"
          >
            <svg class="w-5 h-5 shrink-0 text-neutral-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
            </svg>
            <span class="flex-1 text-left">Accéder au portail étudiant</span>
            <span class="text-xs text-neutral-400 font-normal">clé SSH uniquement</span>
          </a>

        </div>

        <!-- Pied de carte -->
        <div class="px-8 py-4 bg-neutral-50 border-t border-neutral-100">
          <p class="text-xs text-neutral-400 text-center leading-relaxed">
            Accès réservé aux membres de l'École Polytechnique.<br>
            En cas de problème, contactez l'équipe IDCS.
          </p>
        </div>
      </div>

      <!-- Infos comptes de test -->
      <div class="mt-5 p-4 bg-white border border-neutral-200 rounded text-xs text-neutral-500 space-y-1 animate-fade-up" style="animation-delay:0.08s;">
        <p class="font-semibold text-neutral-600 mb-2">Comptes de développement</p>
        <div class="grid grid-cols-2 gap-x-4 gap-y-1 font-mono">
          <span class="text-neutral-400">Admin</span>
          <span>admin / admin123</span>
          <span class="text-neutral-400">Étudiant</span>
          <span>student / student123</span>
        </div>
      </div>

    </div>
  </main>

  <!-- Footer -->
  <footer class="border-t border-neutral-200 bg-white">
    <div class="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
      <span class="text-xs text-neutral-400">CloudPoolManager — IDCS Infrastructure</span>
      <span class="text-xs text-neutral-400">École Polytechnique · Institut Polytechnique de Paris</span>
    </div>
  </footer>

</div>

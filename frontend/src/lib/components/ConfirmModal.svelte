<script lang="ts">
  import { fade, scale } from 'svelte/transition';

  let {
    show = $bindable(false),
    title = 'Confirmation',
    message = 'Êtes-vous sûr ?',
    confirmText = 'Confirmer',
    cancelText = 'Annuler',
    danger = false,
    onConfirm
  }: {
    show: boolean;
    title?: string;
    message: string;
    confirmText?: string;
    cancelText?: string;
    danger?: boolean;
    onConfirm: () => void;
  } = $props();

  function handleConfirm() {
    show = false;
    onConfirm();
  }

  // Monte le modal directement sur <body> : sinon, rendu dans un conteneur animé
  // (transform: translate via animate-fade-*), un position:fixed devient relatif à
  // cet ancêtre transformé → backdrop décalé et modal non centré.
  function portal(node: HTMLElement) {
    if (typeof document !== 'undefined') document.body.appendChild(node);
    return { destroy() { node.parentNode?.removeChild(node); } };
  }
</script>

{#if show}
  <div use:portal class="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-0" transition:fade={{ duration: 150 }}>
    <!-- Backdrop -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="fixed inset-0 bg-neutral-900/50 backdrop-blur-sm" onclick={() => show = false}></div>

    <!-- Modal -->
    <div
      class="relative bg-white dark:bg-neutral-800 rounded-xl shadow-2xl w-full max-w-sm overflow-hidden"
      transition:scale={{ duration: 150, start: 0.95 }}
    >
      <div class="p-5">
        <div class="flex items-start gap-4">
          <div class="shrink-0 mt-0.5">
            {#if danger}
              <div class="w-10 h-10 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center text-red-600 dark:text-red-400">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/></svg>
              </div>
            {:else}
              <div class="w-10 h-10 rounded-full bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center text-amber-600 dark:text-amber-400">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
              </div>
            {/if}
          </div>
          <div>
            <h3 class="text-lg font-bold text-neutral-900 dark:text-white">{title}</h3>
            <p class="text-sm text-neutral-500 dark:text-neutral-400 mt-1 leading-relaxed">{message}</p>
          </div>
        </div>
      </div>
      <div class="px-5 py-4 bg-neutral-50 dark:bg-neutral-900/50 border-t border-neutral-100 dark:border-neutral-800 flex items-center justify-end gap-2">
        <button onclick={() => show = false} class="btn btn-secondary text-sm px-4 py-2">
          {cancelText}
        </button>
        <button onclick={handleConfirm} class="btn {danger ? 'bg-red-600 hover:bg-red-700 text-white' : 'btn-primary'} text-sm px-4 py-2">
          {confirmText}
        </button>
      </div>
    </div>
  </div>
{/if}

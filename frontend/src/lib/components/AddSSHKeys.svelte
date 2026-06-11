<script lang="ts">
  import {
    ListStudentsRequestSchema, type ListStudentsRequest, type ListStudentsResponse,
    AddStudentRequestSchema, type AddStudentRequest, type AddStudentResponse,
    DeleteStudentRequestSchema, type DeleteStudentRequest, type DeleteStudentResponse,
  } from '$lib/grpc/frontcontrol_pb';
  import { apiFetch } from '$lib/api';
  import { addStudents, listStudents, deleteStudent } from '$lib/index';
  import { create } from '@bufbuild/protobuf';
  import { authStore } from '$lib/store';

  let {
    open = $bindable(),
    poolname,
  }: { open: boolean; poolname: string } = $props();

  type AddMode = 'form' | 'raw' | 'github' | 'moodle';

  let addModal = $state(false);
  let loading = $state(false);
  let error: string | null = $state(null);
  let rawMode = $state(false);
  let addMode = $state<AddMode>('form');
  let rawInput = $state('');

  // GitHub mode state
  interface GitHubStudent { login: string; keys: string[] }
  let githubStudents = $state<GitHubStudent[]>([]);
  let githubLoading = $state(false);
  let githubSelected = $state<Set<string>>(new Set());
  let githubFirstNames: Record<string, string> = $state({});
  let githubLastNames: Record<string, string> = $state({});
  let githubKeyChoice: Record<string, number> = $state({});

  async function loadGitHubStudents() {
    githubLoading = true;
    try {
      const res = await apiFetch('/api/github/students');
      if (res.ok) {
        const data: GitHubStudent[] = await res.json() ?? [];
        const seen = new Map<string, GitHubStudent>();
        for (const s of data) {
          if (!s.keys || s.keys.length === 0) continue;
          if (!seen.has(s.login)) seen.set(s.login, s);
        }
        githubStudents = Array.from(seen.values());
      }
    } catch { /* ignore */ } finally { githubLoading = false; }
  }

  function toggleGitHubStudent(login: string) {
    const s = new Set(githubSelected);
    if (s.has(login)) s.delete(login); else s.add(login);
    githubSelected = s;
  }

  async function handleAddGitHub() {
    const toAdd = githubStudents.filter(s => githubSelected.has(s.login));
    if (!toAdd.length) { error = 'Sélectionnez au moins un étudiant.'; return; }
    const students = toAdd.map(s => {
      const keyIdx = githubKeyChoice[s.login] ?? 0;
      return { name: `${(githubFirstNames[s.login] ?? '').trim()}.${(githubLastNames[s.login] ?? '').trim()}`.toLowerCase(), sshKey: s.keys[keyIdx] ?? '' };
    }).filter(s => s.name !== '.' && s.sshKey);
    if (!students.length) { error = 'Renseignez prénom et nom pour les étudiants sélectionnés.'; return; }
    const req: AddStudentRequest = create(AddStudentRequestSchema, { user: $authStore?.email, poolname, students });
    try {
      loading = true; error = null;
      await addStudents(req);
      await handleListStudents();
      githubSelected = new Set();
      addModal = false;
    } catch { error = "Erreur lors de l'ajout."; } finally { loading = false; }
  }

  // Moodle mode state
  interface MoodleCourse { id: number; shortname: string; fullname: string }
  interface MoodleStudent { moodle_id: number; email: string; fullname: string; is_teacher: boolean }
  let moodleConfigured = $state(false);
  let moodleCourses = $state<MoodleCourse[]>([]);
  let moodleCourseId = $state<number | null>(null);
  let moodleStudents = $state<MoodleStudent[]>([]);
  let moodleSelected = $state<Set<string>>(new Set());
  let moodleLoading = $state(false);
  let modes = $derived<[AddMode, string][]>(
    moodleConfigured
      ? [['form', 'Formulaire'], ['raw', 'Import texte'], ['github', 'GitHub'], ['moodle', 'Moodle']]
      : [['form', 'Formulaire'], ['raw', 'Import texte'], ['github', 'GitHub']]
  );

  async function checkMoodle() {
    try {
      const r = await apiFetch('/api/moodle/status');
      if (r.ok) moodleConfigured = !!(await r.json()).configured;
    } catch { /* ignore */ }
  }
  async function loadMoodleCourses() {
    moodleLoading = true;
    try {
      const r = await apiFetch('/api/moodle/courses');
      if (r.ok) moodleCourses = (await r.json()).courses ?? [];
    } catch { /* ignore */ } finally { moodleLoading = false; }
  }
  async function loadMoodleEnrolments(courseId: number) {
    moodleLoading = true; moodleStudents = []; moodleSelected = new Set();
    try {
      const r = await apiFetch(`/api/moodle/enrolments?course_id=${courseId}`);
      if (r.ok) {
        const list: MoodleStudent[] = ((await r.json()).students ?? []).filter((s: MoodleStudent) => !s.is_teacher);
        moodleStudents = list;
        moodleSelected = new Set(list.map(s => s.email));
      }
    } catch { /* ignore */ } finally { moodleLoading = false; }
  }
  function toggleMoodleStudent(email: string) {
    const s = new Set(moodleSelected);
    if (s.has(email)) s.delete(email); else s.add(email);
    moodleSelected = s;
  }
  async function handleImportMoodle() {
    if (moodleCourseId == null) { error = 'Choisissez un cours.'; return; }
    const emails = Array.from(moodleSelected);
    if (!emails.length) { error = 'Sélectionnez au moins un étudiant.'; return; }
    try {
      loading = true; error = null;
      const r = await apiFetch('/api/moodle/import', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pool_id: poolname, user_id: $authStore?.email, course_id: moodleCourseId, emails }),
      });
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error ?? 'erreur');
      await handleListStudents();
      moodleSelected = new Set();
      addModal = false;
    } catch { error = "Erreur lors de l'import depuis Moodle."; } finally { loading = false; }
  }

  interface User { name: string; sshKey: string; ip: string; }
  interface NewStudent { firstName: string; lastName: string; sshKey: string; }

  let users: User[] = $state([]);
  let newStudents: NewStudent[] = $state([{ firstName: '', lastName: '', sshKey: '' }]);

  function addRow() { newStudents = [...newStudents, { firstName: '', lastName: '', sshKey: '' }]; }
  function removeRow(i: number) { newStudents = newStudents.filter((_, idx) => idx !== i); }
  function buildLogin(s: NewStudent): string { return `${s.firstName.trim()}.${s.lastName.trim()}`.toLowerCase(); }

  async function handleListStudents() {
    const req: ListStudentsRequest = create(ListStudentsRequestSchema, { user: $authStore?.email, poolname });
    try {
      loading = true; error = null;
      const res: ListStudentsResponse = await listStudents(req);
      users = res.students.map(s => ({ name: s.name, sshKey: s.sshKey, ip: s.ip }));
    } catch { error = 'Erreur lors du chargement des étudiants.'; }
    finally { loading = false; }
  }

  async function handleDeleteStudent(name: string) {
    if (!confirm(`Supprimer l'étudiant ${name} ?`)) return;
    const req: DeleteStudentRequest = create(DeleteStudentRequestSchema, {
      user: $authStore?.email, poolname, studentName: name,
    });
    try {
      loading = true; error = null;
      await deleteStudent(req);
      await handleListStudents();
    } catch { error = "Erreur lors de la suppression."; }
    finally { loading = false; }
  }

  async function handleAdd() {
    const valid = newStudents.filter(s => s.firstName.trim() && s.lastName.trim() && s.sshKey.trim());
    if (!valid.length) { error = 'Aucun étudiant valide à ajouter.'; return; }
    const req: AddStudentRequest = create(AddStudentRequestSchema, {
      user: $authStore?.email, poolname,
      students: valid.map(s => ({ name: buildLogin(s), sshKey: s.sshKey })),
    });
    try {
      loading = true; error = null;
      await addStudents(req);
      await handleListStudents();
      newStudents = [{ firstName: '', lastName: '', sshKey: '' }];
      addModal = false;
    } catch { error = "Erreur lors de l'ajout."; }
    finally { loading = false; }
  }

  async function handleAddRaw() {
    const lines = rawInput.split('\n').map(l => l.trim()).filter(Boolean);
    const parsed: NewStudent[] = [];
    for (const line of lines) {
      const sep = line.indexOf(';');
      if (sep === -1) continue;
      const loginRaw = line.slice(0, sep).trim();
      const sshKey = line.slice(sep + 1).trim();
      if (!loginRaw || !sshKey) continue;
      const dotIdx = loginRaw.indexOf('.');
      const firstName = dotIdx !== -1 ? loginRaw.slice(0, dotIdx) : loginRaw;
      const lastName = dotIdx !== -1 ? loginRaw.slice(dotIdx + 1) : '';
      parsed.push({ firstName, lastName, sshKey });
    }
    if (!parsed.length) { error = 'Aucun étudiant valide (format: prenom.nom;cle_ssh)'; return; }
    newStudents = parsed;
    await handleAdd();
    rawInput = '';
    newStudents = [{ firstName: '', lastName: '', sshKey: '' }];
    addModal = false;
  }

  $effect(() => { if (open) { handleListStudents(); checkMoodle(); } });
  $effect(() => { if (addModal) { loadGitHubStudents(); } });
  $effect(() => { if (addModal && addMode === 'moodle' && moodleCourses.length === 0) loadMoodleCourses(); });
  $effect(() => { rawMode = addMode === 'raw'; if (rawMode) newStudents = [{ firstName: '', lastName: '', sshKey: '' }]; });
</script>

<!-- Main modal -->
{#if open}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <div class="modal-box" style="max-width:520px;">
      <div class="flex items-center justify-between mb-5">
        <div>
          <h3 class="text-base font-bold text-neutral-900">Étudiants</h3>
          <p class="text-xs text-neutral-500 mt-0.5">{poolname}</p>
        </div>
        <button onclick={() => open = false} class="text-neutral-400 hover:text-neutral-700 transition-colors p-1 rounded hover:bg-neutral-100">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>

      {#if error}
        <div class="mb-4 px-3 py-2.5 rounded bg-red-50 border border-red-200 text-red-700 text-sm animate-fade-in">{error}</div>
      {/if}

      {#if loading && !addModal}
        <div class="flex items-center justify-center py-16">
          <div class="w-8 h-8 rounded-full border-2 border-neutral-200 border-t-primary-700" style="animation: spinnerGlow 0.7s linear infinite;"></div>
        </div>
      {:else if users.length === 0}
        <div class="flex flex-col items-center justify-center py-14 text-neutral-400">
          <svg class="w-10 h-10 mb-3 text-neutral-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"/>
          </svg>
          <p class="text-sm">Aucun étudiant enregistré</p>
        </div>
      {:else}
        <div class="space-y-1 max-h-72 overflow-y-auto pr-1 mb-4">
          {#each users as user, i}
            <div
              class="flex items-center justify-between px-4 py-3 rounded border border-neutral-100 bg-neutral-50 animate-slide-right"
              style="animation-delay:{i*0.03}s"
            >
              <div>
                <p class="text-sm font-semibold text-neutral-900">{user.name}</p>
                {#if user.ip}
                  <p class="text-xs text-neutral-500 font-mono mt-0.5">{user.ip}</p>
                {/if}
              </div>
              <div class="flex items-center gap-2">
                {#if user.ip}
                  <span class="badge badge-ready">Attribué</span>
                {:else}
                  <span class="badge badge-starting">En attente</span>
                {/if}
                <button
                  onclick={() => handleDeleteStudent(user.name)}
                  class="p-1.5 rounded text-neutral-400 hover:text-red-600 hover:bg-red-50 transition-colors"
                  title="Supprimer"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                  </svg>
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}

      <button onclick={() => addModal = true} class="btn btn-primary text-sm w-full">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
        </svg>
        Ajouter des étudiants
      </button>
    </div>
  </div>
{/if}

<!-- Add students modal -->
{#if addModal}
  <div class="modal-overlay" style="z-index:60;" role="dialog" aria-modal="true">
    <div class="modal-box" style="max-width:600px;">
      <div class="flex items-center justify-between mb-5">
        <h3 class="text-base font-bold text-neutral-900">Ajouter des étudiants</h3>
        <button onclick={() => addModal = false} class="text-neutral-400 hover:text-neutral-700 transition-colors p-1 rounded hover:bg-neutral-100">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>

      <!-- Mode toggle -->
      <div class="flex gap-1 mb-5 p-1 bg-neutral-100 rounded border border-neutral-200 w-fit">
        {#each modes as [mode, label]}
          <button
            onclick={() => addMode = mode}
            class="px-4 py-1.5 rounded text-sm font-semibold transition-all {addMode === mode ? 'bg-white text-primary-700 shadow-sm border border-neutral-200' : 'text-neutral-500 hover:text-neutral-700'}"
          >{label}</button>
        {/each}
      </div>

      {#if addMode === 'github'}
        {#if githubLoading}
          <div class="flex items-center justify-center py-10">
            <div class="w-6 h-6 rounded-full border-2 border-neutral-200 border-t-primary-700" style="animation: spinnerGlow 0.7s linear infinite;"></div>
          </div>
        {:else if githubStudents.length === 0}
          <div class="flex flex-col items-center justify-center py-10 text-neutral-400 text-center">
            <svg class="w-8 h-8 mb-2 text-neutral-300" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/>
            </svg>
            <p class="text-sm">Aucun étudiant connecté via GitHub</p>
            <p class="text-xs text-neutral-400 mt-1">Les étudiants doivent se connecter avec leur compte GitHub sur le portail.</p>
          </div>
        {:else}
          <div class="space-y-3 max-h-72 overflow-y-auto pr-1">
            {#each githubStudents as gh}
              <div class="p-3 rounded border transition-colors {githubSelected.has(gh.login) ? 'border-primary-300 bg-primary-50' : 'border-neutral-200 bg-neutral-50'}">
                <div class="flex items-center gap-2 mb-2">
                  <input type="checkbox" checked={githubSelected.has(gh.login)} onchange={() => toggleGitHubStudent(gh.login)} class="w-4 h-4 accent-primary-700" />
                  <svg class="w-4 h-4 text-neutral-500" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/>
                  </svg>
                  <span class="text-sm font-semibold text-neutral-800 font-mono">{gh.login}</span>
                  <span class="text-xs text-neutral-400">{gh.keys.length} clé{gh.keys.length > 1 ? 's' : ''}</span>
                </div>
                {#if githubSelected.has(gh.login)}
                  <div class="flex gap-2 mt-1">
                    <input class="field flex-1 text-xs" type="text" placeholder="Prénom" bind:value={githubFirstNames[gh.login]} />
                    <input class="field flex-1 text-xs" type="text" placeholder="Nom" bind:value={githubLastNames[gh.login]} />
                  </div>
                  {#if gh.keys.length > 1}
                    <select class="field text-xs mt-2 font-mono" bind:value={githubKeyChoice[gh.login]}>
                      {#each gh.keys as key, i}
                        <option value={i}>Clé {i+1} — {key.slice(0,40)}…</option>
                      {/each}
                    </select>
                  {/if}
                {/if}
              </div>
            {/each}
          </div>
          <div class="flex justify-end mt-4">
            <button onclick={handleAddGitHub} disabled={loading || githubSelected.size === 0} class="btn btn-primary text-sm">
              {#if loading}
                <span class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>
              {/if}
              Ajouter les sélectionnés
            </button>
          </div>
        {/if}
      {:else if addMode === 'moodle'}
        <div class="space-y-3">
          <div>
            <label class="section-label block mb-1">Cours Moodle</label>
            <select
              class="field text-sm"
              bind:value={moodleCourseId}
              onchange={() => { if (moodleCourseId != null) loadMoodleEnrolments(moodleCourseId); }}
            >
              <option value={null} disabled selected>— Choisir un cours —</option>
              {#each moodleCourses as c}
                <option value={c.id}>{c.shortname} — {c.fullname}</option>
              {/each}
            </select>
            <p class="text-xs text-neutral-400 mt-1">Les étudiants importés se connecteront via Moodle (clé SSH non requise).</p>
          </div>

          {#if moodleLoading}
            <div class="flex items-center justify-center py-10">
              <div class="w-6 h-6 rounded-full border-2 border-neutral-200 border-t-primary-700" style="animation: spinnerGlow 0.7s linear infinite;"></div>
            </div>
          {:else if moodleStudents.length > 0}
            <div class="space-y-1 max-h-60 overflow-y-auto pr-1">
              {#each moodleStudents as s}
                <label class="flex items-center gap-2 px-3 py-2 rounded border cursor-pointer transition-colors {moodleSelected.has(s.email) ? 'border-primary-300 bg-primary-50' : 'border-neutral-200 bg-neutral-50'}">
                  <input type="checkbox" checked={moodleSelected.has(s.email)} onchange={() => toggleMoodleStudent(s.email)} class="w-4 h-4 accent-primary-700" />
                  <span class="text-sm font-semibold text-neutral-800">{s.fullname}</span>
                  <span class="text-xs text-neutral-400 font-mono ml-auto">{s.email}</span>
                </label>
              {/each}
            </div>
            <button onclick={handleImportMoodle} disabled={loading || moodleSelected.size === 0} class="btn btn-primary text-sm w-full">
              {#if loading}
                <span class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>
              {/if}
              Importer {moodleSelected.size} étudiant{moodleSelected.size > 1 ? 's' : ''}
            </button>
          {:else if moodleCourseId != null}
            <p class="text-sm text-neutral-400 py-6 text-center">Aucun étudiant dans ce cours.</p>
          {/if}
        </div>
      {:else if rawMode}
        <div class="space-y-3">
          <label class="section-label block mb-1">Un étudiant par ligne : <code class="text-primary-700 font-mono">prenom.nom;cle_ssh</code></label>
          <textarea
            class="field font-mono text-xs resize-none"
            rows="10"
            placeholder={"jean.dupont;ssh-ed25519 AAAA...\npaul.martin;ssh-ed25519 BBBB..."}
            bind:value={rawInput}
          ></textarea>
          <button onclick={handleAddRaw} disabled={!rawInput.trim() || loading} class="btn btn-primary text-sm">
            Importer
          </button>
        </div>
      {:else}
        <div class="space-y-3 max-h-72 overflow-y-auto pr-1">
          {#each newStudents as student, i}
            <div class="p-3 rounded border border-neutral-200 bg-neutral-50 space-y-2">
              <div class="flex gap-2">
                <input class="field flex-1" type="text" placeholder="Prénom" bind:value={student.firstName} />
                <input class="field flex-1" type="text" placeholder="Nom" bind:value={student.lastName} />
              </div>
              <div class="flex gap-2">
                <input class="field flex-1 font-mono text-xs" type="text" placeholder="ssh-ed25519 AAAA..." bind:value={student.sshKey} />
                {#if newStudents.length > 1}
                  <button onclick={() => removeRow(i)} class="btn btn-danger p-2 shrink-0">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                    </svg>
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
        <div class="flex justify-between items-center mt-4">
          <button onclick={addRow} class="btn btn-secondary text-sm">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
            </svg>
            Ajouter une ligne
          </button>
          <button onclick={handleAdd} disabled={loading} class="btn btn-primary text-sm">
            {#if loading}
              <span class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full" style="animation: spinnerGlow 0.6s linear infinite;"></span>
            {/if}
            Enregistrer
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}

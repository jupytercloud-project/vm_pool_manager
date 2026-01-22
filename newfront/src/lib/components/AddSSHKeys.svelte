<script lang="ts">
    import { 
        Modal,
        Button,
        Label,
        Input,
        VirtualList,
        checkbox,
        Spinner,
		Checkbox,
		Textarea,
        Helper,
    } from 'flowbite-svelte';

    import { CloseOutline } from 'flowbite-svelte-icons';

    import {
	ListStudentsRequestSchema,
    type ListStudentsRequest,
    type ListStudentsResponse,
    AddStudentRequestSchema,
    type AddStudentRequest,
    type AddStudentResponse,
    } from '$lib/grpc/frontcontrol_pb';

    import { addStudents } from '$lib/index';

    import { listStudents } from '$lib/index';
	import { create } from '@bufbuild/protobuf';
	import { authStore } from '$lib/store';

    export let open: boolean;
    export let poolname: string;

    let openSSHModal = false;
    let loading = false;
    let error: string | null = null;
    let Rawinput: string = "";
    let Rawinputcheckbox = false;

    interface User {
        name: string;
        sshKey: string;
        ip: string;
    }
    let users: User[] = [];

    interface NewStudent {
    login: string;
    sshKey: string;
    }

    let newStudents: NewStudent[] = [
    { login: "", sshKey: "" }
    ];

    function addStudentRow() {
    newStudents = [...newStudents, { login: "", sshKey: "" }];
    }

    function removeStudentRow(index: number) {
        newStudents = newStudents.filter((_, i) => i !== index);
    }



    async function handleListStudents() {
        const req: ListStudentsRequest = create(ListStudentsRequestSchema, {
            user: $authStore?.email,
            poolname: poolname,
        });
        try {
            loading = true;
            error = null;
            const res: ListStudentsResponse = await listStudents(req);
            users = res.students.map((student) => ({
                name: student.name,
                sshKey: student.sshKey,
                ip: student.ip,
            }));
        } catch (err) {
            error = "Erreur lors du chargement des étudiants.";
            console.error(err);
        } finally {
            loading = false;
        }
    }

    async function handleAddingStudent() {
        const validStudents = newStudents.filter(
            s => s.login.trim() !== "" && s.sshKey.trim() !== ""
        );
        if (validStudents.length === 0) {
            error = "Aucun étudiant valide à ajouter.";
            return;
        }

        // TODO: Implement the logic to add students to the backend
        const req: AddStudentRequest = create(AddStudentRequestSchema, {
            user: $authStore?.email,
            poolname: poolname,
            students: validStudents.map(s => ({
                name: s.login,
                sshKey: s.sshKey,
            })),
        });
        console.log("students send : ", req)
        try {
            loading = true;
            error = null;
            const res: AddStudentResponse = await addStudents(req);
            // Après ajout, recharger la liste des étudiants
            await handleListStudents();
        } catch (err) {
            error = "Erreur lors de l'ajout des étudiants.";
            console.error(err);
        } finally {
            loading = false;
        }

        newStudents = [{ login: "", sshKey: "" }];
        openSSHModal = false;
    }

    async function handleAddingStudentRaw() {
        // TODO: Implement the logic to add students from raw input
    }


    // charger automatiquement à l’ouverture du modal
    $: if (open) {
        handleListStudents();
    }

    $: if (Rawinputcheckbox) {
    newStudents = [{ login: "", sshKey: "" }];
}


</script>

<!-- Modal d'affichage des clés SSH des utilisateurs -->
<Modal bind:open class="bg-gray-500 bg-opacity-50" focustrap>
    <div class="p-4 w-full max-w-xl">
        <h2 class="text-lg font-semibold mb-4">
            Liste des étudiants
        </h2>

        {#if loading}
            <div class="flex h-[400px] items-center justify-center">
                <Spinner size="12" />
            </div>
        {:else if error}
            <div class="text-center text-red-500 h-[400px] flex items-center justify-center">
                {error}
            </div>
        {:else if users.length === 0}
            <div class="text-center text-gray-500 h-[400px] flex items-center justify-center">
                Aucun étudiant trouvé
            </div>
        {:else}
            <VirtualList
            items={users}
            minItemHeight={60}
            height={400}
            class="rounded-lg border"
            contained>
                {#snippet children(item, index)}
                    {@const user = item as User}
                    <div class="flex items-center justify-between border-b p-4 transition-colors
                    {index % 2 === 0 ? 'bg-gray-100 dark:bg-gray-900' : 'bg-gray-300 dark:bg-gray-800'}
                    hover:bg-blue-100 dark:hover:bg-blue-900/20"
                    style="height:70px">
                        <div class="flex-1">
                            <div class="font-medium text-gray-900 dark:text-white"> {user.name}</div>
                            <div class="text-medium text-gray-900">{user.ip}</div>
                        </div>
                        {#if user.ip !== ""}
                            <span class="rounded-full px-3 py-1 text-xs font-semibold bg-green-100 text-green-800">attributed</span>
                        {:else}
                            <span class="rounded-full px-3 py-1 text-xs font-semibold bg-yellow-400 text-yellow-800">not attributed</span>
                        {/if}
                    </div>
                {/snippet}
            </VirtualList>
        {/if}
    </div>
    <Button size="xs" onclick={() => openSSHModal = true}>
        Ajouter student
    </Button>
</Modal>


<!-- Modal d'ajout de clés SSH -->
<Modal bind:open={openSSHModal} class="bg-gray-500 bg-opacity-50" focustrap>
    <Checkbox bind:checked={Rawinputcheckbox}> Raw </Checkbox>
    {#if Rawinputcheckbox}
        <Textarea 
        placeholder="Entrez les logins et clés SSH des étudiants ici."
        rows={15} class="w-full mb-4"
        bind:value={Rawinput}/>
        <Helper class="mt-2 text-sm"> une input par ligne. La forme doit etre : prenom.nom;sshKey; </Helper>
        <Button 
        size="md" 
        onclick={() => handleAddingStudentRaw()}
        disabled={Rawinput.trim() === ""}>
            Ajouter
        </Button>
    {:else}
    <div class="space-y-3">
        {#each newStudents as student, index}
            <div class="flex items-center gap-2">
                <Input
                type="text"
                placeholder="login étudiant: prenom.nom"
                bind:value={student.login}
                class="flex-1"
                />

                <Input
                type="text"
                placeholder="clé SSH"
                bind:value={student.sshKey}
                class="flex-1"
                />

                {#if newStudents.length > 1}
                    <Button
                    size="xs"
                    color="red"
                    onclick={() => removeStudentRow(index)}>
                        <CloseOutline class="shrink-0 h-6 w-6" />
                    </Button>
                {/if}
            </div>
        {/each}

        <div class="flex justify-between pt-2">
            <Button size="xs" onclick={addStudentRow}>
                Ajouter une ligne
            </Button>

            <Button size="md" onclick={handleAddingStudent}>
                Ajouter
            </Button>
        </div>
    </div>
    {/if}
</Modal>


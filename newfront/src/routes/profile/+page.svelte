<script lang="ts">
  import { 
    authStore,
    configs,
    serverPools,
    flavors,
    images,
    networks,
  } from '$lib/store';
  import { goto } from '$app/navigation';
  import { 
    Table,
    TableBody,
    TableBodyCell,
    TableBodyRow,
    TableHead,
    TableHeadCell,
    Button,
	Modal,
	Textarea,
  } from "flowbite-svelte";

  let token: string | null = null;
  $: token = $authStore?.token ?? null;
  let sshModal = false;
  let sshKey = "";

  async function handleSSHKeySubmit() {
    console.log("Submitting SSH Key:", sshKey);
    sshModal = false;
  }

</script>

<Table shadow hoverable={true} class="w-full text-tertiary-50">
    <caption class="text-2xl text-left font-bold mb-4 pl-4">
      Profil de l'utilisateur
      <p class="mt-1 text-sm font-normal text-gray-300 dark:text-gray-400">
        <strong>Email :</strong> {$authStore?.email}
      </p>
    </caption>

    {#if !$serverPools || $serverPools.length === 0}
      <p class="text-gray-500">Aucun serverpool trouvé</p>
    {:else}
      <TableHead class="bg-secondary-200">
        <TableHeadCell>Serverpool Name</TableHeadCell>
        <TableHeadCell>Image</TableHeadCell>
        <TableHeadCell>Flavor</TableHeadCell>
        <TableHeadCell>Network</TableHeadCell>
        <TableHeadCell>Minimum VM</TableHeadCell>
        <TableHeadCell>Maximum VM</TableHeadCell>
        <TableHeadCell><span class="sr-only">Inspect</span></TableHeadCell>
      </TableHead>
      <TableBody>
        {#each $serverPools as sp, i}
          <TableBodyRow class={i % 2 === 0 ? 
            'bg-tertiary-400 hover:bg-tertiary-200' :
            'bg-tertiary-300 hover:bg-tertiary-200'}>
            <TableBodyCell>{sp.name}</TableBodyCell>
            <TableBodyCell>
              {$images.find(img => img.id === sp.image)?.name ?? sp.image}
            </TableBodyCell>
            <TableBodyCell>
              {$flavors.find(f => f.id === sp.flavor)?.name ?? sp.flavor}
            </TableBodyCell>
            <TableBodyCell>
              {$networks.find(n => n.id === sp.network)?.name ?? sp.network}
            </TableBodyCell>
            <TableBodyCell>{sp.minVm}</TableBodyCell>
            <TableBodyCell>{sp.maxVm}</TableBodyCell>
            <TableBodyCell class="flex justify-center">
              <Button class="bg-option-500"
                onclick={() => goto(`/serverpool/${sp.name}`)}>
                Inspect
              </Button>
            </TableBodyCell>
          </TableBodyRow>
        {/each}
      </TableBody>
    {/if}
</Table>

<Table shadow hoverable={true} class="w-full text-tertiary-50">
  <caption class="text-2xl text-left font-bold mb-4 pl-4">
    Mes Configs
  </caption>
  <TableHead class="bg-secondary-200">
    <TableHeadCell>Nom de la Config</TableHeadCell>
    <TableHeadCell>Data</TableHeadCell>
  </TableHead>
  <TableBody>
  {#each $configs as conf, i}
    <TableBodyRow class={i % 2 === 0 ? 
            'bg-tertiary-400 hover:bg-tertiary-200' :
            'bg-tertiary-300 hover:bg-tertiary-200'}>
      <TableBodyCell>{conf.name}</TableBodyCell>
      <TableBodyCell>{conf.data}</TableBodyCell>
    </TableBodyRow>
  {/each}
  </TableBody>
</Table>

<Button size="md" class="mt-4 bg-option-500"
  onclick={() => sshModal = true}>
  Add SSH Key
</Button>

{#if sshModal}
  <Modal
    bind:open={sshModal}
    class="bg-gray-500 bg-opacity-50"
    focustrap>
    <Textarea
    placeholder="Enter your SSH Public Key"
    class="w-full h-20"
    bind:value={sshKey} />
    <Button
      size="md"
      onclick={handleSSHKeySubmit}
      class="mt-4 bg-option-500">
      Submit SSH Key
    </Button>
  </Modal>
{/if}

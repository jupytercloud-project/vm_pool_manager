<script lang="ts">

import { onDestroy, onMount } from 'svelte';
import { goto } from '$app/navigation';
import { authStore, serverpoolStore } from '$lib/index';
import { Button, Dropdown, DropdownItem, Table, TableBody, TableHead, TableBodyCell, TableBodyRow, TableHeadCell } from 'flowbite-svelte';
import { ChevronDownOutline } from 'flowbite-svelte-icons';

let token: string | null = null;
$: token = $authStore;

let user;
let serverpools;
let error;
$: ({ user, serverpools, error } = $serverpoolStore);

let interval: ReturnType<typeof setInterval>;

onMount(async () => {
    if (!token) {
        goto('/'); // redirige si pas connecté
        return;
    } else {
        serverpoolStore.fetchServerpools();
        interval = setInterval(serverpoolStore.fetchServerpools, 50000);
    }
  });

    onDestroy(() => {
    clearInterval(interval);
});

let selectedsp: string = 'Choisissez le serverpool';
let select = false;

const handleClick = (e: Event) => {
    e.preventDefault();
    const target = e.target as HTMLButtonElement;
    selectedsp = target.name;
    select = true;
}

</script>

<Button size="md" class=" w-48 h-12">{selectedsp}<ChevronDownOutline class="ms-2 h-6 text-white" /></Button>
<Dropdown simple>
    {#each serverpools as sp}
        <DropdownItem name={sp.serverpool_id} onclick={handleClick}>{sp.serverpool_id}</DropdownItem>      
    {/each}
</Dropdown>

{#if select}
    <Table>
        <TableHead>

        </TableHead>
        <TableBody>
            
        </TableBody>
    </Table>
{/if}
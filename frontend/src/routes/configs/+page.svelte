<script lang="ts">
	import { Button, Dropdown, DropdownItem , Label, Textarea, Input } from "flowbite-svelte";
    import { authStore, serverpoolStore, createConfig } from '$lib/index';
	import { ChevronDownOutline } from "flowbite-svelte-icons";
	import { onMount } from "svelte";
	import type { Config } from "@sveltejs/kit";


    let configs: string = "Configurations";
    let token: string | null = null;
    let textspacedisplay: boolean = false;
    let text: string = "";
    let newconfigname: string = "";
    let configlist: Config[] = [];
    
    $: token = $authStore;
    $: configlist = $serverpoolStore.configs;


    const handleClickDropdown = async (e: Event) => {
        e.preventDefault();
        const target = e.target as HTMLButtonElement;
        configs = target.name;
        text = configlist.find(c => c.name === target.name)?.script || "";
        textspacedisplay = true;

    }
    
    onMount(async () => {
        if (!token) {
            // Rediriger vers la page de connexion si le token n'existe pas
            window.location.href = '/';
        }
    });

    async function handlecreateConfig() {
        // Logique pour créer une nouvelle configuration
        console.log("Creating new configuration:", newconfigname, text);
        await createConfig(newconfigname, text);
    }

</script>

<Button size="md" class="w-48 h-12">
    {configs} <ChevronDownOutline class="ms-2 h-6 text-white" />
</Button>
<Dropdown simple isOpen={false} class="mt-2">
    {#each configlist as config}
        <DropdownItem name={config.name} onclick={handleClickDropdown}>{config.name}</DropdownItem>
    {/each}
</Dropdown>

<Button size="md" class="w-48 h-12 mt-4" onclick={() => textspacedisplay = true}>
    Create a new configuration
</Button>

{#if textspacedisplay}
    <Label for="textarea-id" class="mb-2">Votre script de configuration</Label>
    <Textarea id="textarea-id" placeholder="#!/bin/bash" rows={15} bind:value={text} />
    <Label for="config-name" class="mb-2 mt-2">Nom de la configuration</Label>
    <Input id="config-name" type="text" placeholder="Configuration Name" class="mt-2 mb-2" bind:value={newconfigname} />
    <Button size="md" class="w-48 h-12 mt-2" onclick={handlecreateConfig}>Save Configuration</Button>
{/if}
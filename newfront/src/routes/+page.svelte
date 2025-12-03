<script lang="ts">
    import {
        images,
        flavors,
        networks,
        servers,
        serverPools,
        configs
    } from "$lib/store";

</script>

<h3>Configs</h3>
{#each $configs as conf}
    {conf.name}
{/each}
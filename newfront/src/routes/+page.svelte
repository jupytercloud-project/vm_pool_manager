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
  <p>{conf}</p>
{/each}
<h3>Server Pools</h3>
{#each $serverPools as pool}
  <div style="border:1px solid #ccc;padding:8px;margin-bottom:10px;">
    <h4>{pool.name}</h4>
    {#each $servers.filter(serv => serv.metadata?.user_id
        === pool.metadata?.user_id
        && serv.metadata?.serverpool === pool.name) as serv}
      <p>{serv.name}</p>
    {:else}
      <p><em>Aucun serveur associé</em></p>
    {/each}
  </div>
{/each}

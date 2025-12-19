<script lang="ts">
import {
  Button,
  Dropdown,
  DropdownItem,
  Table,
  TableBody,
  TableHead,
  TableBodyCell,
  TableBodyRow,
  TableHeadCell,
  Modal,
  Label,
  Input,
  Select,
  MultiSelect,
  Spinner,
  Clipboard,
} from 'flowbite-svelte';
import { CheckOutline, ChevronDownOutline } from 'flowbite-svelte-icons';
import {
  rebuildServer,
  RebuildServerRequestSchema,
  CreatePoolRequestSchema,
  DeletePoolRequestSchema,
  deletePool,
  createPool,
  addServer,
} from '$lib/index';
import type {
  ServerPool,
  Server,
  CreatePoolRequest,
  DeletePoolRequest,
  RebuildServerRequest,
  Image
} from '$lib/type';
import {
  authStore,
  serverPools,
  servers,
  configs,
  images,
  flavors,
  networks
} from '$lib/store';
import { onMount } from 'svelte';
import { page } from '$app/state';
import { create } from '@bufbuild/protobuf';
import type {
  DeletePoolResponse,
  RebuildServerResponse,
} from '$lib/grpc/frontcontrol_pb';


let token: string | null = null;
let selectedsp: string = 'Choisissez le serverpool';
let serversp: Server[] = [];

let selectedNetwork: string = "";
let selectedFlavor: string = "";
let selectedConfigFile: string = "";
let createspModal: boolean = false;
let createError: string = "";
let createSuccess = false;
let scheduleDay: string = "";
let scheduleTime: string = "";
let scheduleWindowHours: number = 1; // optionnel

type CreateServerPoolForm = {
    name: string;
    image: string;
    flavor: string;
    networks: string;
    minVm: number;
    maxVm: number;
    config: string;
};

onMount(async() => {
	if (!token) {
		window.location.href = '/';
	}
	selectedsp = page.params.id || 'Choisissez le serverpool';
});

const handleClick = async (e: Event) => {
	e.preventDefault();
	const target = e.target as HTMLButtonElement;
	selectedsp = target.name;
};

$: token = $authStore?.token ?? null;
$: selectedPool = $serverPools.find(p => p.name === selectedsp);
$: serversp = selectedPool
  ? $servers.filter(server => {
      let metadata = server.metadata;
      if (typeof metadata === "string") {
        try {
          metadata = JSON.parse(metadata);
        } catch {
          metadata = {};
        }
      }

      return (
        metadata?.serverpool_id === selectedPool.name
      );
    })
  : [];


$: networkOptions = $networks.map(net => ({
    value: net.id,
    name: net.name,
  }));
  
  $: sortedFlavors = [...$flavors].sort((a, b) =>
  a.name.localeCompare(b.name, undefined, {numeric: true, sensitivity:"base"})
);

async function handleRebuildServer(serv: Server) {
	if (!confirm(`Voulez-vous rebuild le serveur ${serv.name} ?`)) {
		return;
	}
	const req: RebuildServerRequest = create(RebuildServerRequestSchema,{
    user: $authStore?.email,
    poolId: serv.metadata?.serverpool_id,
    serverId: serv.name
  });
	console.log("Rebuild request: ", req);
  try {
		const res: RebuildServerResponse = await rebuildServer(req);
		if (!res.success) {
      console.error("Erreur rebuild server");
		}
	} catch (err) {
		console.error("Erreur rebuild server: ", err);
		throw err;
	}
}

async function handleDeleteServerpool(sp: ServerPool) {
	if (!confirm(`Voulez-vous supprimer le serveur ${sp.name} ?`)) {
		return;
	}
	const req: DeletePoolRequest = create(DeletePoolRequestSchema,{
    user: $authStore?.email,
    poolId: sp.name
  });
	try {
		const res: DeletePoolResponse = await deletePool(req);
		if (res.success) {
      selectedsp = "Choisissez le serverpool";
		}
	} catch (err) {
		console.error("Erreur lors de la suppression du pool: ", err);
		throw err;
	}
}

async function handleCreateServer(sp: ServerPool) {
  if (!confirm(`Voulez-vous ajouter un serveur au serverpool ${sp.name} ?`)) {
    return;
  }
  const req: CreatePoolRequest = create(CreatePoolRequestSchema, {
    user: $authStore?.email,
    name: sp.name,
    image: sp.image,
    flavor: sp.flavor,
    network: sp.network,
    minVm: String(sp.minVm),
    maxVm: String(sp.maxVm),
    config: sp.config,
  });

  try {
    const res: RebuildServerResponse = await addServer(req);
    if (res.success) {
      console.log("Serveur ajouté avec succès au serverpool.");
    } else {
      console.error("Erreur lors de l'ajout du serveur au serverpool.");
    }
  } catch (err) {
    console.error("Impossible d'ajouter le serveur au serverpool.", err);
  }
}

export function getUniqueFirstAlphaBlocks(images: Image[]): string[] {
  const prefixes = images
    .map(img => {
      const match = img.name.match(/^[A-Za-z]+/);
      return match ? match[0] : null;
    })
    .filter((x): x is string => x !== null);

  return Array.from(new Set(prefixes));
}

export function filterImagesByPrefix(images: Image[], prefix:string): Image[] {
  return images.filter(img => img.name.startsWith(prefix));
}

let selectedGroupImage: string | null = null;
let selectedImage: string | null = null;


async function handleCreateServerpool(event: Event) {
    event.preventDefault();

    const form = event.target as HTMLFormElement;
    const formData = new FormData(form);

    const data: CreateServerPoolForm = {
        name: formData.get("namesp") as string,
        image: selectedImage ?? "",
        flavor: selectedFlavor,
        networks: selectedNetwork,
        minVm: Number(formData.get("min_vm")),
        maxVm: Number(formData.get("max_vm")),
        config: selectedConfigFile,
    };

    
    if (!data.image || !data.flavor || !data.config) {
      createError = "Veuillez remplir tous les champs obligatoires.";
      return;
    }
    
    console.log("📤 Creating pool:", data);
    const startDate = computeNextSchedule(
      Number(scheduleDay),
      scheduleTime
    );

    const req: CreatePoolRequest = create(CreatePoolRequestSchema, {
        user: $authStore?.email,
        name: data.name,
        image: data.image,
        flavor: data.flavor,
        network: data.networks,
        minVm: String(data.minVm),
        maxVm: String(data.maxVm),
        config: data.config,
        startTime: {
          seconds: BigInt(Math.floor(startDate.getTime()/1000)),
          nanos: (startDate.getDate() % 1000) * 1_000_000,
        },
        timeWindow: scheduleWindowHours,
    });

    console.log(req)

    try {
        createError = "";
        const res = await createPool(req);

        if (res.success) {
            createSuccess = true;
            setTimeout(() => (createspModal = false), 1200);
        } else {
            createError = "Erreur lors de la création du serverpool.";
        }
    } catch (err) {
        console.error(err);
        createError = "Impossible de créer le serverpool.";
    }
}

function computeNextSchedule(dayOfWeek: number, time: string): Date {
  const [hours, minutes] = time.split(":").map(Number);
  const now = new Date();

  const target = new Date(now);
  target.setHours(hours, minutes, 0, 0);

  let delta = dayOfWeek - now.getDay();
  if (delta < 0 || (delta === 0 && target < now)) {
    // Si le jour est déjà passé cette semaine, on ajoute 7 jours
    delta += 7;
  }

  target.setDate(now.getDate() + delta);
  return target;
}


</script>

<!-- Dropdown -->
<Button size="md" class="w-48 h-12">
  {selectedsp}<ChevronDownOutline class="ms-2 h-6 text-white" />
</Button>
<Dropdown simple isOpen={false} class="mt-2">
  {#each $serverPools as sp}
	<DropdownItem name={sp.name} onclick={handleClick}>{sp.name}</DropdownItem>
  {/each}
</Dropdown>

<!-- Table -->
{#if serversp.length > 0}
  <Table
    hoverable={true}
    striped={false}
    class="mt-4 w-full text-tertiary-50">
  <caption class="text-left mb-2">
	{selectedsp}
	<p class="text-sm font-normal">
    Flavor: {$flavors.find(img => img.id === selectedPool?.flavor)?.name 
      ?? selectedPool?.flavor}
  </p>
	<p class="text-sm font-normal">
    Image: {$images.find(img => img.id === selectedPool?.image)?.name
      ?? selectedPool?.image}
  </p>
  <p class="text-sm font-normal">
    Network: {$networks.find(img => img.id === selectedPool?.network)?.name
      ?? selectedPool?.network}
  </p>
  </caption>

  <TableHead class="bg-tertiary-500 text-white">
	<TableHeadCell>Nom</TableHeadCell>
	<TableHeadCell>Status</TableHeadCell>
	<TableHeadCell>IP</TableHeadCell>
	<TableHeadCell></TableHeadCell>
  </TableHead>

  <TableBody>
	{#each serversp as s, i}
	  <TableBodyRow class={i % 2 === 0
    ? 'bg-tertiary-400 hover:bg-tertiary-200'
    : 'bg-tertiary-300 hover:bg-tertiary-200'}>
		<TableBodyCell>{s.name}</TableBodyCell>
		<TableBodyCell>
		  {#if s.status === 'BUILD' || s.status === 'REBUILD'}
			<Spinner />
			{/if}
			{s.status}
		</TableBodyCell>
		<TableBodyCell>{s.ipAddress}</TableBodyCell>
		<TableBodyCell>
		  {#if s.status === 'BUILD' || s.status === 'REBUILD'}
			<Button
        disabled
        size="sm"
        class="bg-option-500"
        onclick={() => handleRebuildServer(s)}>
          Rebuild
      </Button>
		  {:else}
			<Button
        size="sm"
        class="bg-option-500"
        onclick={() => handleRebuildServer(s)}>
          Rebuild
      </Button>
		  {/if}
		</TableBodyCell>
	  </TableBodyRow>
	{/each}
  </TableBody>
</Table>

{#if selectedPool}
	<Button
    class="bg-tertiary-500 mt-4"
    onclick={() => handleDeleteServerpool(selectedPool)}>
		  Supprimer le serverpool
	</Button>
  <Button
  class="bg-option-400 mt-4"
  onclick={() => handleCreateServer(selectedPool)}>
      Ajouter un serveur au serverpool
  </Button>
{/if}

{:else}
  <p>Aucun serveur trouvé pour ce serverpool.</p>
{/if}

<!-- Modal -->
<Button
  size="md"
  class="bg-option-500 mt-4"
  onclick={() => createspModal = true}>
    Créer un serverpool
</Button>

{#if createspModal}
  <Modal
    bind:open={createspModal}
    class="bg-gray-500 bg-opacity-50"
    focustrap>
      <form
        class="flex flex-col space-y-6 p-6 bg-white rounded-lg"
        on:submit|preventDefault={handleCreateServerpool}
      >
      <h3 class="text-xl font-medium text-gray-800">Créer un Serverpool</h3>

      {#if createError}
        <p class="text-red-500">{createError}</p>
      {/if}
      {#if createSuccess}
        <p class="text-green-600 font-semibold">Serverpool créé !</p>
      {/if}

      <!-- NOM -->
      <Label>
        <span>Nom du Serverpool</span>
        <Input type="text" name="namesp" required />
      </Label>

      <!-- IMAGE (group + exact image) -->
      <Label>
        <span>Image</span>
        <Select bind:value={selectedGroupImage} required>
          <option disabled selected value="">Choisir un groupe d’images</option>
          {#each getUniqueFirstAlphaBlocks($images) as prefix}
            <option value={prefix}>{prefix}</option>
          {/each}
        </Select>

        {#if selectedGroupImage}
          <Select bind:value={selectedImage} required>
            <option disabled selected value="">Choisir une image</option>
            {#each filterImagesByPrefix($images, selectedGroupImage) as img}
              <option value={img.id}>{img.name}</option>
            {/each}
          </Select>
        {/if}
      </Label>

      <!-- FLAVOR -->
      <Label>
        <span>Flavor</span>
        <Select bind:value={selectedFlavor} required>
          <option disabled selected value="">Choisir un flavor</option>
          {#each sortedFlavors as f}
            <option value={f.id}>{f.name}</option>
          {/each}
        </Select>
      </Label>

      <!-- NETWORKS MULTI -->
      <Label>
        <span>Réseaux</span>
        <Select bind:value={selectedNetwork} required>
    	<option disabled selected value="">Choisir un réseau</option>
    		{#each networkOptions as net}
        		<option value={net.value}>{net.name}</option>
    		{/each}
		</Select>


      </Label>

      <!-- MIN / MAX VM -->
      <Label>
        <span>Min VM</span>
        <Input type="number" name="min_vm" min="1" value="1" required />
      </Label>

      <Label>
        <span>Max VM</span>
        <Input type="number" name="max_vm" min="1" value="1" required />
      </Label>

      <!-- CONFIG -->
      <Label>
        <span>Config</span>
        <Select bind:value={selectedConfigFile} required>
          <option disabled selected value="">Choisir une config</option>
          {#each $configs as c}
            <option value={c.name}>{c.name}</option>
          {/each}
        </Select>
      </Label>

      <!-- SCHEDULE -->
      <Label>
        <span>Schedule</span>
      
        <div class="grid grid-cols-3 gap-3 mt-1">
          <!-- Jour -->
          <Select bind:value={scheduleDay} required>
            <option disabled selected value="">Jour</option>
            <option value="1">Lundi</option>
            <option value="2">Mardi</option>
            <option value="3">Mercredi</option>
            <option value="4">Jeudi</option>
            <option value="5">Vendredi</option>
            <option value="6">Samedi</option>
            <option value="0">Dimanche</option>
          </Select>
        
          <!-- Heure -->
          <Input
            type="time"
            bind:value={scheduleTime}
            required
          />
        
          <!-- Fenêtre -->
          <Input
            type="number"
            min="1"
            max="24"
            bind:value={scheduleWindowHours}
            placeholder="Durée (h)"
            required
          />
        </div>
      
        <p class="text-xs text-gray-500 mt-1">
          Ex: Tous les lundis à 02:00 pendant 2h
        </p>
      </Label>

      <!-- ACTIONS -->
      <div class="flex justify-end gap-4 pt-4">
        <Button
          type="button"
          class="bg-gray-400"
          onclick={() => createspModal = false}>
            Annuler
        </Button>
        <Button type="submit" class="bg-option-500">
          Créer
        </Button>
      </div>
    </form>
  </Modal>
{/if}

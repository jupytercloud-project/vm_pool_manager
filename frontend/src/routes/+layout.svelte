<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { Navbar, NavBrand, NavLi, NavUl, NavHamburger, Button} from 'flowbite-svelte';
	import { Modal, Label, Input, Checkbox } from 'flowbite-svelte'

	let { children } = $props();

	// script pour modal login
	let loginModal = $state(false);
	let loginError = $state("");

	async function trylogin(event: Event) {
		const form = event.target as HTMLFormElement;
		const data = new FormData(form);

		loginError = "";
		console.log("coucou");
		if ((data.get("email") as string)?.length < 1 || (data.get("password") as string)?.length < 1) {
			loginError = "Champs non rempli";
			return false
		}

		const jsonData = Object.fromEntries(data.entries());

		try {
			const response = await fetch('http://localhost:8080/login', {
				method: 'POST',
				headers: {'Content-Type': 'application/json'},
				body: JSON.stringify(jsonData)
			});

			if (!response.ok) {
				loginError = "Erreur lors de la connexion";
				return
			}
		
				const result = await response.json();
				const token = result.token;

				localStorage.setItem('authToken', token);
				loginModal = false;
				console.log("Login reussi");

		} catch (err) {
			loginError = "Erreur backend";
			console.error(err);
		}
	}

	// script pour modal createAccount
	let createAccountModal = $state(false);
	let createAccountError = $state("");

	async function tryCreate(event:Event) {
		
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children?.()}


<!-- NavBar -->
<div class="min-h-screen bg-gradient-to-b from-gray-600 via-gray-400 to-gray-800">
	<Navbar class=" sticky start-0 top-0 z-20 w-full bg-gray-600/30 backdrop-blur-md shadow-md rounded-2xl ">
		<NavBrand href="/">
			<img src="src/lib/assets/IDCS.png" class="me-3 h-6 sm:h-9" alt="ICDS Logo" />
			<span class="self-center text-xl font-semibold whitespace-nowrap text-gray-300 dark:text-white">CloudPoolManager</span>
		</NavBrand>
	<div class="flex md:order-2 gap-2">
		<Button size="sm" color="blue" onclick={() => (loginModal = true)}>Login</Button>
		<Button size="sm" color="green" onclick={() => (createAccountModal = true)}>Create Account</Button>
		<NavHamburger />
	</div>
	<NavUl>
		<NavLi href="/" class="text-gray-300">Home</NavLi>
		<NavLi href="/" class="text-gray-300">About</NavLi>
	</NavUl>
	
	</Navbar>

	<!-- Login Modal -->
	 <Modal bind:open={loginModal} class="bg-gray-400">
		<form class="flex flex-col space-y-6" onsubmit={trylogin}>
			<h3 class="mb-4 text-xl font-medium text-gray-300">Connexion</h3>
			{#if loginError}
				<Label color="red">{loginError}</Label>
			{/if}
			<Label class="space-y-2">
				<span>Email</span>
				<Input type="email" name="email" placeholder="name@company.com" required/>
			</Label>
			<Label class="space-y-2">
				<span>Password</span>
				<Input type="password" name="password" placeholder="votre mot de passe" required/>
			</Label>
			<Button type="submit">Se connecter</Button>
		</form>
	 </Modal>

	<!-- Create Account Modal -->
	<Modal bind:open={createAccountModal} class="bg-gray-400">
		<form class="flex flex-col space-y-6" onsubmit={tryCreate}>
			<h3 class="mb-4 text-xl font-medium text-gray-300">Creer votre compte</h3>
			{#if createAccountError}
				<Label color="red">{createAccountError}</Label>
			{/if}
			<Label class="space-y-2">
				<span>Name</span>
				<Input type="text" name="name" placeholder="votre nom" required/>
			</Label>
			<Label class="space-y-2">
				<span>Email</span>
				<Input type="email" name="email" placeholder="name@company.com" required/>
			</Label>
			<Label class="space-y-2">
				<span>Password</span>
				<Input type="password" name="password" placeholder="votre mot de passe" required/>
			</Label>
			<Label class="space-y-2">
				<span>Confirme Password</span>
				<Input type="password" name="Confirmpassword" placeholder="Confirmez votre mot de passe" required/>
			</Label>
			<Button type="submit">Creer</Button>
		</form>
	</Modal>

</div>

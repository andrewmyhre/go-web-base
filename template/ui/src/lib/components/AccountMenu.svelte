<script lang="ts">
	import Avatar from './Avatar.svelte';
	import AccountModal from './AccountModal.svelte';
	import type { Me } from '$lib/api';

	let { user, onSignOut }: { user: Me; onSignOut: () => void } = $props();

	let menuOpen = $state(false);
	let accountOpen = $state(false);
	let trigger: HTMLButtonElement | undefined = $state();
	let firstItem: HTMLButtonElement | undefined = $state();

	async function toggleMenu() {
		menuOpen = !menuOpen;
		if (menuOpen) {
			// Wait for the menu to render before focusing its first item.
			await Promise.resolve();
			firstItem?.focus();
		}
	}

	function closeMenu() {
		menuOpen = false;
		trigger?.focus();
	}

	function openAccount() {
		menuOpen = false;
		accountOpen = true;
	}

	// Closing the account modal returns focus to the avatar.
	function closeAccount() {
		accountOpen = false;
		trigger?.focus();
	}

	function signOut() {
		menuOpen = false;
		onSignOut();
	}

	function onMenuKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			closeMenu();
		}
	}
</script>

<div class="account-menu">
	<button
		bind:this={trigger}
		class="avatar-button"
		type="button"
		aria-label="Account menu for {user.name}"
		aria-haspopup="true"
		aria-expanded={menuOpen}
		aria-controls="account-menu-items"
		onclick={toggleMenu}
	>
		<Avatar name={user.name} avatar={user.avatar} size={36} />
	</button>
	{#if menuOpen}
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<ul id="account-menu-items" class="account-menu-items" onkeydown={onMenuKeydown}>
			<li><button bind:this={firstItem} type="button" onclick={openAccount}>Account</button></li>
			<li><button type="button" onclick={signOut}>Sign out</button></li>
		</ul>
	{/if}
</div>

<AccountModal {user} open={accountOpen} onclose={closeAccount} />

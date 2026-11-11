<script lang="ts">
	import Avatar from './Avatar.svelte';
	import { roleLabel, type Me } from '../api/index';

	let { user, open, onclose }: { user: Me; open: boolean; onclose: () => void } = $props();

	let dialog: HTMLDialogElement | undefined = $state();
	let closeButton: HTMLButtonElement | undefined = $state();

	// The dialog is driven by `open`: shown modally (so the page behind it
	// is inert) with focus on Close, and closed when `open` goes false.
	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) {
			dialog.showModal();
			closeButton?.focus();
		} else if (!open && dialog.open) {
			dialog.close();
		}
	});

	// Escape fires `cancel` on a modal dialog. The browser would close it
	// itself; it is prevented so `open` stays the one source of truth and
	// the parent returns focus.
	function oncancel(e: Event) {
		e.preventDefault();
		onclose();
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onclose();
		}
	}
</script>

<dialog
	bind:this={dialog}
	class="account-modal"
	aria-modal="true"
	aria-labelledby="account-modal-title"
	{oncancel}
	{onkeydown}
>
	<h2 id="account-modal-title">Account</h2>
	<div class="account-identity">
		<Avatar name={user.name} avatar={user.avatar} size={64} />
		<div>
			<p class="name">{user.name}</p>
			{#if user.email}<p class="email">{user.email}</p>{/if}
		</div>
	</div>
	<h3 id="account-modal-roles">Roles</h3>
	{#if user.roles.length}
		<ul class="roles" aria-labelledby="account-modal-roles">
			{#each user.roles as r (r.domain + '\u0000' + r.role)}
				<li>{roleLabel(r)}</li>
			{/each}
		</ul>
	{:else}
		<p class="status">No roles.</p>
	{/if}
	<button class="button" type="button" bind:this={closeButton} onclick={onclose}>Close</button>
</dialog>

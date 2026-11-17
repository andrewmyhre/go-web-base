<script lang="ts">
	import { onMount } from 'svelte';
	import {
		AdminApiError,
		SIGN_IN_PATH,
		createApiClient,
		createSiteInvite,
		listSiteInvites,
		listSiteSettings,
		revokeSiteInvite,
		updateSiteSetting,
		type SiteInvite,
		type SiteSetting
	} from '../api/index';

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let { client = createApiClient() }: { client?: import('openapi-fetch').Client<any> } = $props();

	type State =
		| { kind: 'loading' }
		| { kind: 'forbidden' }
		| { kind: 'error' }
		| { kind: 'ready'; settings: SiteSetting[]; invites: SiteInvite[] };

	let view = $state<State>({ kind: 'loading' });
	let message = $state('');
	let email = $state('');
	// Chosen but unsaved values, by setting key.
	let draft = $state<Record<string, string>>({});

	// silent keeps the page on screen while it refreshes, so a form being used
	// is not torn down under the user.
	async function load(silent = false) {
		if (!silent) view = { kind: 'loading' };
		try {
			const [settings, invites] = await Promise.all([listSiteSettings(client), listSiteInvites(client)]);
			draft = Object.fromEntries(settings.map((s) => [s.key, s.value]));
			view = { kind: 'ready', settings, invites };
		} catch (e) {
			view = e instanceof AdminApiError && (e.status === 401 || e.status === 403) ? { kind: 'forbidden' } : { kind: 'error' };
		}
	}

	onMount(() => load());

	// Run an action, then reload; show the API's reason when it refuses.
	async function act(what: () => Promise<unknown>, done: string) {
		message = '';
		try {
			await what();
			message = done;
			await load(true);
		} catch (e) {
			message = e instanceof AdminApiError && e.detail ? e.detail : 'That did not work. Try again.';
		}
	}

	const save = (s: SiteSetting) => act(() => updateSiteSetting(s.key, draft[s.key], client), `${s.label} saved.`);
	const invite = async (e: SubmitEvent) => {
		e.preventDefault();
		const address = email.trim();
		if (!address) return;
		await act(() => createSiteInvite(address, client), `Invited ${address}.`);
		email = '';
	};
	const revoke = (i: SiteInvite) => act(() => revokeSiteInvite(i.id, client), `Revoked the invite for ${i.email}.`);
	const signInLink = () => (typeof location === 'undefined' ? SIGN_IN_PATH : location.origin + '/');
</script>

<section class="site-settings">
	<h1>Settings</h1>
	{#if view.kind === 'loading'}
		<p class="status">Loading settings...</p>
	{:else if view.kind === 'forbidden'}
		<p class="status" role="alert">Only an administrator can change the site settings.</p>
	{:else if view.kind === 'error'}
		<p class="status" role="alert">Could not load the settings. Try again.</p>
		<button class="button" type="button" onclick={() => load()}>Retry</button>
	{:else}
		{#if message}<p class="status" role="status">{message}</p>{/if}

		{#each view.settings as s (s.key)}
			<form class="setting" onsubmit={(e) => { e.preventDefault(); save(s); }}>
				<label for="setting-{s.key}">{s.label}</label>
				<p class="help">{s.description}</p>
				<select id="setting-{s.key}" value={draft[s.key]} onchange={(e) => (draft[s.key] = e.currentTarget.value)}>
					{#each s.options as o (o)}<option value={o}>{o}</option>{/each}
				</select>
				<button class="button" type="submit" disabled={draft[s.key] === s.value}>Save</button>
			</form>
		{/each}

		<h2>Invites</h2>
		<p class="help">
			Core sends no email. Invite an address, then share the site's address ({signInLink()}) with them: their first
			sign-in with that Google account is let in while sign-up is invite-only.
		</p>
		<form class="invite-form" onsubmit={invite}>
			<label for="invite-email">Email address</label>
			<input id="invite-email" type="email" bind:value={email} required />
			<button class="button" type="submit">Invite</button>
		</form>
		{#if view.invites.length === 0}
			<p class="status">No invites yet.</p>
		{:else}
			<table class="invites">
				<thead><tr><th>Email</th><th>Status</th><th></th></tr></thead>
				<tbody>
					{#each view.invites as i (i.id)}
						<tr>
							<td>{i.email}</td>
							<td>{i.status}</td>
							<td>
								{#if i.status === 'pending'}
									<button type="button" onclick={() => revoke(i)} aria-label="Revoke the invite for {i.email}">Revoke</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	{/if}
</section>

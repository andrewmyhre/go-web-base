<script lang="ts" module>
	/** One entry of the side menu. */
	export interface SideMenuItem {
		label: string;
		href: string;
		/** Entries sharing a group sit under its heading; none means the top list. */
		group?: string;
		/** false omits the entry (for example when /me says the user cannot open it). Default true. */
		visible?: boolean;
		/**
		 * Overrides whether the entry is the current page. By default it is when
		 * the path equals `href`; set it for entries that own several paths.
		 */
		current?: boolean;
	}
</script>

<script lang="ts">
	import { page } from '$app/state';

	/**
	 * The entries, in order. Ungrouped ones form the first list; each group
	 * follows in order of first appearance under its heading. Without `items`
	 * the menu is a single Home link.
	 */
	let { items }: { items?: SideMenuItem[] } = $props();

	const shown = $derived(
		(items ?? [{ label: 'Home', href: '/' }]).filter((i) => i.visible !== false)
	);
	const top = $derived(shown.filter((i) => !i.group));
	const groups = $derived.by(() => {
		const out: { name: string; items: SideMenuItem[] }[] = [];
		for (const i of shown) {
			if (!i.group) continue;
			let g = out.find((x) => x.name === i.group);
			if (!g) out.push((g = { name: i.group, items: [] }));
			g.items.push(i);
		}
		return out;
	});
	const current = (i: SideMenuItem) => i.current ?? page.url.pathname === i.href;
</script>

<nav class="menu" aria-label="Main">
	{#if top.length}
		<ul>
			{#each top as i (i.href)}
				<li><a href={i.href} aria-current={current(i) ? 'page' : undefined}>{i.label}</a></li>
			{/each}
		</ul>
	{/if}
	{#each groups as g, n (g.name)}
		<div class="group" id="menu-group-{n}">{g.name}</div>
		<ul aria-labelledby="menu-group-{n}">
			{#each g.items as i (i.href)}
				<li><a href={i.href} aria-current={current(i) ? 'page' : undefined}>{i.label}</a></li>
			{/each}
		</ul>
	{/each}
</nav>

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
	// Unique per menu instance, so two menus never share heading ids.
	const uid = $props.id();

	const shown = $derived(
		(items ?? []).filter((i) => i.visible !== false)
	);
	// The ungrouped list first (when it has items), then each group in order of first appearance.
	const sections = $derived.by(() => {
		const out: { name: string; items: SideMenuItem[] }[] = [];
		if (shown.some((i) => !i.group)) out.push({ name: '', items: shown.filter((i) => !i.group) });
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
	{#if items}
		{#each sections as g, n (n)}
			{#if g.name}<div class="group" id="{uid}-group-{n}">{g.name}</div>{/if}
			<ul aria-labelledby={g.name ? `${uid}-group-${n}` : undefined}>
				{#each g.items as i, m (m + i.href)}
					<li><a href={i.href} aria-current={current(i) ? 'page' : undefined}>{i.label}</a></li>
				{/each}
			</ul>
		{/each}
	{:else}
		<ul>
			<li><a href="/" aria-current={page.url.pathname === '/' ? 'page' : undefined}>Home</a></li>
		</ul>
	{/if}
</nav>

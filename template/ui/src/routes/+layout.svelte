<script lang="ts">
	import type { Snippet } from 'svelte';
	import SideMenu from '$lib/components/SideMenu.svelte';
	import '../app.css';

	let { children }: { children: Snippet } = $props();
</script>

<div class="shell">
	<SideMenu />
	<div class="content">
		{@render children()}
	</div>
</div>

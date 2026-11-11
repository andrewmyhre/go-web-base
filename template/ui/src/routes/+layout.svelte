<script lang="ts">
	import type { Snippet } from 'svelte';
	import { SideMenu } from '@andrewmyhre/web-base-ui';
	import '../app.css';

	let { children }: { children: Snippet } = $props();
</script>

<div class="shell">
	<SideMenu />
	<div class="content">
		{@render children()}
	</div>
</div>

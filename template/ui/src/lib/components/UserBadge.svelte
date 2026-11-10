<script lang="ts">
	import Avatar from './Avatar.svelte';

	let { name, avatar }: { name: string; avatar: string } = $props();
</script>

<div class="user">
	<Avatar {name} {avatar} />
	<p class="name">{name}</p>
</div>

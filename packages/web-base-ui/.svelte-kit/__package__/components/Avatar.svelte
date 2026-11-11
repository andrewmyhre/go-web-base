<script lang="ts">
	let { name, avatar, size = 48 }: { name: string; avatar: string; size?: number } = $props();

	const initials = $derived(
		name
			.trim()
			.split(/\s+/)
			.filter(Boolean)
			.map((part) => part.charAt(0))
			.join('')
			.slice(0, 2)
			.toUpperCase()
	);
</script>

{#if avatar}
	<!-- Google avatars refuse requests carrying a referrer. -->
	<img
		class="avatar"
		src={avatar}
		alt=""
		width={size}
		height={size}
		style:width="{size}px"
		style:height="{size}px"
		referrerpolicy="no-referrer"
	/>
{:else}
	<span class="avatar placeholder" style:width="{size}px" style:height="{size}px" aria-hidden="true">{initials}</span>
{/if}

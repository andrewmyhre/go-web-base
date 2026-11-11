<script lang="ts">
	import { page } from '$app/state';
</script>

<nav class="menu" aria-label="Main">
	<ul>
		<li><a href="/" aria-current={page.url.pathname === '/' ? 'page' : undefined}>Home</a></li>
	</ul>
</nav>

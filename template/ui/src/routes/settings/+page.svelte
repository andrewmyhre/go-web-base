<script lang="ts">
	import { SiteSettings } from '@andrewmyhre/web-base-ui';
	import { api } from '$lib/api';
</script>

<main>
	<!-- Core serves the routes this page calls (/admin/settings and
	     /admin/invites) and answers 403 to anyone who is not an Admin. -->
	<SiteSettings client={api} />
</main>

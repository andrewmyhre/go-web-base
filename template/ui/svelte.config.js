import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		// A single-page app: nothing is prerendered, and every path the
		// server does not hold a file for gets index.html (the static server
		// does the same; see deploy/ui-nginx.conf).
		adapter: adapter({ fallback: 'index.html' }),
		// SvelteKit registers src/service-worker.ts itself.
		serviceWorker: { register: true }
	}
};

export default config;

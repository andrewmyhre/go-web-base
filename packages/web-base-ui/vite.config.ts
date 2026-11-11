import { fileURLToPath } from 'node:url';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';
import { defineConfig } from 'vitest/config';

// The package is built with svelte-package; this config is for the tests only.
export default defineConfig({
	plugins: [svelte(), svelteTesting()],
	resolve: {
		// Outside SvelteKit there is no $app/state; tests mock it over this stub.
		alias: { '$app/state': fileURLToPath(new URL('./test/app-state.ts', import.meta.url)) }
	},
	test: {
		environment: 'happy-dom',
		include: ['src/**/*.test.ts']
	}
});

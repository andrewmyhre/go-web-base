import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';

const read = (rel: string) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)));

describe('LICENSE', () => {
	// Lives in test/, not src/lib, so svelte-package does not ship it in dist.
	// pnpm pack includes a LICENSE in the package root whatever `files` says, so the
	// tarball carries this copy. It must stay byte-identical to the repository's.
	it('the package copy is identical to the repository root copy', () => {
		const pkg = read('../LICENSE');
		expect(pkg.length).toBeGreaterThan(10000);
		expect(pkg.equals(read('../../../LICENSE'))).toBe(true);
	});
});

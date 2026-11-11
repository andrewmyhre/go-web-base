/**
 * What the service worker does with each request. Kept apart from
 * src/service-worker.ts so it can be tested without a worker.
 *
 * The rule that matters: API paths are never answered from, or written to,
 * the cache. A cached /me would show a signed-out browser someone else's
 * name, and a cached /auth/* redirect would replay a spent sign-in. They
 * are bypassed before anything else is considered, navigations included.
 */

/** Paths the API serves (the dev ingress sends these to the API). */
export const API_PREFIXES = ['/auth', '/me', '/health', '/openapi.json'] as const;

/**
 * True for a path the API owns: a prefix itself, or below it as a segment.
 * `extraPrefixes` are the project's own API paths beyond the base ones.
 */
export function isApiPath(pathname: string, extraPrefixes: readonly string[] = []): boolean {
	return [...API_PREFIXES, ...extraPrefixes].some((p) => pathname === p || pathname.startsWith(p + '/'));
}

/**
 * - `bypass`: the worker does not touch the request; the browser fetches it.
 * - `cache-first`: a build asset precached at install, answered from cache.
 * - `shell`: an app navigation, fetched from the network, falling back to the
 *   precached index.html when offline. The response is never cached.
 */
export type Strategy = 'bypass' | 'cache-first' | 'shell';

export interface RequestLike {
	method: string;
	url: string;
	mode: string;
}

export function strategyFor(
	req: RequestLike,
	origin: string,
	precached: ReadonlySet<string>,
	extraApiPrefixes: readonly string[] = []
): Strategy {
	if (req.method !== 'GET') return 'bypass';
	const url = new URL(req.url);
	if (url.origin !== origin) return 'bypass';
	if (isApiPath(url.pathname, extraApiPrefixes)) return 'bypass';
	// Navigations first, even to a precached path such as `/`, so a deploy
	// reaches an online browser on its next load.
	if (req.mode === 'navigate') return 'shell';
	if (precached.has(url.pathname)) return 'cache-first';
	return 'bypass';
}

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
export declare const API_PREFIXES: readonly ["/auth", "/me", "/health", "/openapi.json"];
/** True for a path the API owns: a prefix itself, or below it as a segment. */
export declare function isApiPath(pathname: string): boolean;
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
export declare function strategyFor(req: RequestLike, origin: string, precached: ReadonlySet<string>): Strategy;

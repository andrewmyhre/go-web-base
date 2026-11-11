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
export const API_PREFIXES = ['/auth', '/me', '/health', '/openapi.json'];
/** True for a path the API owns: a prefix itself, or below it as a segment. */
export function isApiPath(pathname) {
    return API_PREFIXES.some((p) => pathname === p || pathname.startsWith(p + '/'));
}
export function strategyFor(req, origin, precached) {
    if (req.method !== 'GET')
        return 'bypass';
    const url = new URL(req.url);
    if (url.origin !== origin)
        return 'bypass';
    if (isApiPath(url.pathname))
        return 'bypass';
    // Navigations first, even to a precached path such as `/`, so a deploy
    // reaches an online browser on its next load.
    if (req.mode === 'navigate')
        return 'shell';
    if (precached.has(url.pathname))
        return 'cache-first';
    return 'bypass';
}

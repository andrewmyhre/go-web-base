import createClient, {} from 'openapi-fetch';
/** A role as the account view shows it, for example "Admin (global)". */
export function roleLabel(r) {
    return `${r.role} (${r.domain})`;
}
/**
 * Where the browser goes to sign in. It is a full-page redirect flow (the
 * API redirects to Google and back, then to `/`), not a typed call, so it is
 * a path rather than a client method. Links to it must bypass the SvelteKit
 * router (`data-sveltekit-reload`).
 */
export const SIGN_IN_PATH = '/auth/google';
/** Ends the session. POST, answered with 204 and a cleared cookie. */
export const LOGOUT_PATH = '/auth/logout';
/**
 * The typed client, generic over the project's generated `paths` type (from
 * `openapi-typescript`). The UI and the API share an origin (the ingress
 * routes API paths to the API), so the base URL is empty and requests are
 * relative. `baseUrl` and `fetch` exist for tests.
 *
 *     import type { paths } from './openapi';
 *     export const api = createApiClient<paths>();
 */
export function createApiClient(opts = {}) {
    return createClient({
        baseUrl: opts.baseUrl ?? '',
        credentials: 'same-origin',
        ...(opts.fetch ? { fetch: opts.fetch } : {})
    });
}
/** The API answered something other than a user or "not signed in". */
export class ApiError extends Error {
    status;
    operation;
    constructor(status, operation = 'GET /me') {
        super(`${operation} answered ${status}`);
        this.status = status;
        this.operation = operation;
        this.name = 'ApiError';
    }
}
const base = (client) => client;
/** The signed-in user, or null when the browser has no session (401). */
export async function getMe(client = createApiClient()) {
    const { data, response } = await base(client).GET('/me');
    if (response.status === 401)
        return null;
    if (!response.ok || !data)
        throw new ApiError(response.status);
    return data;
}
/**
 * Sign the browser out: POST /auth/logout, which deletes the session and
 * clears the cookie. The cookie is HttpOnly, so only the API can clear it.
 */
export async function signOut(client = createApiClient()) {
    const { response } = await base(client).POST(LOGOUT_PATH);
    if (!response.ok)
        throw new ApiError(response.status, `POST ${LOGOUT_PATH}`);
}

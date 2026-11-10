import createClient from 'openapi-fetch';
import type { components, paths } from './openapi';

export type { paths };

/** The signed-in user, as GET /me returns it. Taken from the generated contract. */
export type Me = components['schemas']['MeOutputBody'];

/** One role the user is bound to, and the domain it applies in. */
export type MeRole = components['schemas']['MeRole'];

/** A role as the account view shows it, for example "Admin (global)". */
export function roleLabel(r: MeRole): string {
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
export const LOGOUT_PATH = '/auth/logout' satisfies keyof paths;

export type ApiClient = ReturnType<typeof createApiClient>;

/**
 * The typed client, generated from the API's OpenAPI document. The UI and
 * the API share an origin (the ingress routes API paths to the API), so the
 * base URL is empty and requests are relative. `baseUrl` and `fetch` exist
 * for tests.
 */
export function createApiClient(opts: { baseUrl?: string; fetch?: typeof globalThis.fetch } = {}) {
	return createClient<paths>({
		baseUrl: opts.baseUrl ?? '',
		credentials: 'same-origin',
		...(opts.fetch ? { fetch: opts.fetch } : {})
	});
}

/** The API answered something other than a user or "not signed in". */
export class ApiError extends Error {
	constructor(
		readonly status: number,
		readonly operation: string = 'GET /me'
	) {
		super(`${operation} answered ${status}`);
		this.name = 'ApiError';
	}
}

/** The signed-in user, or null when the browser has no session (401). */
export async function getMe(client: ApiClient = createApiClient()): Promise<Me | null> {
	const { data, response } = await client.GET('/me');
	if (response.status === 401) return null;
	if (!response.ok || !data) throw new ApiError(response.status);
	return data;
}

/**
 * Sign the browser out: POST /auth/logout, which deletes the session and
 * clears the cookie. The cookie is HttpOnly, so only the API can clear it.
 */
export async function signOut(client: ApiClient = createApiClient()): Promise<void> {
	const { response } = await client.POST(LOGOUT_PATH);
	if (!response.ok) throw new ApiError(response.status, `POST ${LOGOUT_PATH}`);
}

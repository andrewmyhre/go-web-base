import createClient, { type Client } from 'openapi-fetch';

/** One role the user is bound to, and the domain it applies in. */
export interface MeRole {
	role: string;
	domain: string;
}

/** The signed-in user, as the API's GET /me returns it. */
export interface Me {
	/** Google account ID (the OIDC sub). This is the identity. */
	id: string;
	name: string;
	/** Shown only, never matched on. Empty unless Google says it is verified. */
	email: string;
	avatar: string;
	roles: MeRole[];
}

/**
 * The part of a project's generated `paths` type that these helpers use: a
 * `GET /me` answering a `Me` as JSON and a `POST /auth/logout`. The generated
 * type of any go-web-base project satisfies it, so `createApiClient<paths>()`
 * checks that the project's OpenAPI document still offers what the helpers
 * call.
 */
export interface BasePaths {
	'/me': {
		get: {
			responses: { 200: { content: { 'application/json': Me } } };
		};
	};
	'/auth/logout': {
		post: {
			responses: { 204: unknown };
		};
	};
}

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
export const LOGOUT_PATH = '/auth/logout';

/** The typed client for a project's generated `paths`. */
export type ApiClient<P extends object = BasePaths> = Client<P>;

/**
 * The typed client, generic over the project's generated `paths` type (from
 * `openapi-typescript`). The UI and the API share an origin (the ingress
 * routes API paths to the API), so the base URL is empty and requests are
 * relative. `baseUrl` and `fetch` exist for tests.
 *
 *     import type { paths } from './openapi';
 *     export const api = createApiClient<paths>();
 */
export function createApiClient<P extends BasePaths = BasePaths>(
	opts: { baseUrl?: string; fetch?: typeof globalThis.fetch } = {}
): Client<P> {
	return createClient<P>({
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

// The helpers use only BasePaths, whatever else the project's paths hold.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyClient = Client<any>;
const base = (client: AnyClient) => client as unknown as Client<BasePaths>;

/** The signed-in user, or null when the browser has no session (401). */
export async function getMe(client: AnyClient = createApiClient()): Promise<Me | null> {
	const { data, response } = await base(client).GET('/me');
	if (response.status === 401) return null;
	if (!response.ok || !data) throw new ApiError(response.status);
	return data;
}

/**
 * Sign the browser out: POST /auth/logout, which deletes the session and
 * clears the cookie. The cookie is HttpOnly, so only the API can clear it.
 */
export async function signOut(client: AnyClient = createApiClient()): Promise<void> {
	const { response } = await base(client).POST(LOGOUT_PATH);
	if (!response.ok) throw new ApiError(response.status, `POST ${LOGOUT_PATH}`);
}

/** One site setting, as GET /admin/settings returns it. */
export interface SiteSetting {
	key: string;
	label: string;
	description: string;
	/** The values the setting allows. */
	options: string[];
	value: string;
	default: string;
	updated_at?: string;
	updated_by?: string;
}

/** One sign-up invite, as GET /admin/invites returns it. */
export interface SiteInvite {
	id: number;
	email: string;
	status: 'pending' | 'revoked' | 'consumed';
	created_by?: string;
	created_at: string;
	revoked_at?: string;
	consumed_at?: string;
	consumed_by?: string;
}

/**
 * The part of a project's `paths` the site settings helpers use. It is in
 * every project's OpenAPI document because core registers these routes; the
 * helpers use only these, whatever else the project's paths hold.
 */
export interface SiteSettingsPaths {
	'/admin/settings': {
		get: { responses: { 200: { content: { 'application/json': { settings: SiteSetting[] } } } } };
	};
	'/admin/settings/{key}': {
		put: {
			parameters: { path: { key: string } };
			requestBody: { content: { 'application/json': { value: string } } };
			responses: { 200: { content: { 'application/json': SiteSetting } } };
		};
	};
	'/admin/invites': {
		get: { responses: { 200: { content: { 'application/json': { invites: SiteInvite[] } } } } };
		post: {
			requestBody: { content: { 'application/json': { email: string } } };
			responses: { 200: { content: { 'application/json': SiteInvite } } };
		};
	};
	'/admin/invites/{id}': {
		delete: { parameters: { path: { id: number } }; responses: { 204: unknown } };
	};
}

const admin = (client: AnyClient) => client as unknown as Client<SiteSettingsPaths>;

/** The API refused with this status and, when it sent one, this reason. */
export class AdminApiError extends ApiError {
	constructor(
		status: number,
		operation: string,
		readonly detail: string = ''
	) {
		super(status, operation);
		this.name = 'AdminApiError';
	}
}

function failed(response: Response, operation: string, error?: unknown): never {
	const detail = (error as { detail?: unknown } | undefined)?.detail;
	throw new AdminApiError(response.status, operation, typeof detail === 'string' ? detail : '');
}

/** Every site setting with its value. 401 or 403 throws an AdminApiError with that status. */
export async function listSiteSettings(client: AnyClient = createApiClient()): Promise<SiteSetting[]> {
	const { data, error, response } = await admin(client).GET('/admin/settings');
	if (!response.ok || !data) return failed(response, 'GET /admin/settings', error);
	return data.settings;
}

/** Change a setting. The API refuses a value the setting does not allow with 422. */
export async function updateSiteSetting(
	key: string,
	value: string,
	client: AnyClient = createApiClient()
): Promise<SiteSetting> {
	const { data, error, response } = await admin(client).PUT('/admin/settings/{key}', {
		params: { path: { key } },
		body: { value }
	});
	if (!response.ok || !data) return failed(response, 'PUT /admin/settings/{key}', error);
	return data;
}

/** Every invite, newest first, whatever its state. */
export async function listSiteInvites(client: AnyClient = createApiClient()): Promise<SiteInvite[]> {
	const { data, error, response } = await admin(client).GET('/admin/invites');
	if (!response.ok || !data) return failed(response, 'GET /admin/invites', error);
	return data.invites;
}

/** Invite an email address. 409 when it already has a pending invite, 422 for a bad address. */
export async function createSiteInvite(email: string, client: AnyClient = createApiClient()): Promise<SiteInvite> {
	const { data, error, response } = await admin(client).POST('/admin/invites', { body: { email } });
	if (!response.ok || !data) return failed(response, 'POST /admin/invites', error);
	return data;
}

/** Withdraw a pending invite. */
export async function revokeSiteInvite(id: number, client: AnyClient = createApiClient()): Promise<void> {
	const { error, response } = await admin(client).DELETE('/admin/invites/{id}', { params: { path: { id } } });
	if (!response.ok) return failed(response, 'DELETE /admin/invites/{id}', error);
}

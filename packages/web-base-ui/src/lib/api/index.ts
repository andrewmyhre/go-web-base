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
 * `GET <prefix>/me` answering a `Me` as JSON and a `POST <prefix>/auth/logout`.
 * The prefix is '' by default. A project whose API lives under `/api/v1` has
 * `paths` keyed `/api/v1/me`, which satisfies `BasePaths<'/api/v1'>`. The
 * generated type of any go-web-base project satisfies it, so
 * `createApiClient<paths>()` checks that the project's OpenAPI document still
 * offers what the helpers call.
 */
export type BasePaths<Prefix extends string = ''> = {
	[K in `${Prefix}/me`]: {
		get: {
			responses: { 200: { content: { 'application/json': Me } } };
		};
	};
} & {
	[K in `${Prefix}/auth/logout`]: {
		post: {
			responses: { 204: unknown };
		};
	};
};

/**
 * The prefixes P offers both `<prefix>/me` and `<prefix>/auth/logout` under:
 * '' for `/me`, `/api/v1` for `/api/v1/me`. `never` when P has neither pair.
 */
export type ApiPrefixOf<P> = {
	[K in keyof P]: K extends `${infer X}/me`
		? `${X}/auth/logout` extends keyof P
			? X
			: never
		: never;
}[keyof P] &
	string;

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

/** The path of GET /me under an API prefix ('' by default). */
export const mePath = (apiPrefix = ''): string => `${apiPrefix}/me`;

/** The path of POST /auth/logout under an API prefix ('' by default). */
export const logoutPath = (apiPrefix = ''): string => `${apiPrefix}${LOGOUT_PATH}`;

/** The typed client for a project's generated `paths`. */
export type ApiClient<P extends object = BasePaths> = Client<P>;

/** Options for createApiClient. */
export interface ApiClientOptions<Prefix extends string> {
	baseUrl?: string;
	fetch?: typeof globalThis.fetch;
	/** Where `paths` keeps /me and /auth/logout: '' (default) or for example '/api/v1'. */
	apiPrefix?: Prefix;
}

/**
 * The arguments for `paths` P: options are optional when P keeps /me and
 * /auth/logout unprefixed, `apiPrefix` is required (and checked) when it keeps
 * them under a prefix, and P without either pair is a type error.
 */
export type CreateApiClientArgs<P> = [ApiPrefixOf<P>] extends [never]
	? [error: 'paths must offer <prefix>/me and <prefix>/auth/logout']
	: '' extends ApiPrefixOf<P>
		? [opts?: ApiClientOptions<ApiPrefixOf<P>>]
		: [opts: ApiClientOptions<ApiPrefixOf<P>> & { apiPrefix: ApiPrefixOf<P> }];

// The prefix each client was made with, so getMe(api) and signOut(api) call
// the right paths without repeating it.
const prefixes = new WeakMap<object, string>();

/**
 * The typed client, generic over the project's generated `paths` type (from
 * `openapi-typescript`). The UI and the API share an origin (the ingress
 * routes API paths to the API), so the base URL is empty and requests are
 * relative. `baseUrl` and `fetch` exist for tests.
 *
 *     import type { paths } from './openapi';
 *     export const api = createApiClient<paths>();
 *
 * A project whose paths are keyed `/api/v1/me` passes the prefix, which must
 * be one `paths` offers `/me` and `/auth/logout` under:
 *
 *     export const api = createApiClient<paths>({ apiPrefix: '/api/v1' });
 *
 * The prefix is already in the generated keys, so it does not change the
 * request URLs; it tells `getMe` and `signOut` which keys to call.
 */
export function createApiClient<P extends object = BasePaths>(
	...args: CreateApiClientArgs<P>
): Client<P> {
	const opts: ApiClientOptions<string> = (args[0] as ApiClientOptions<string> | undefined) ?? {};
	const client = createClient<P>({
		baseUrl: opts.baseUrl ?? '',
		credentials: 'same-origin',
		...(opts.fetch ? { fetch: opts.fetch } : {})
	});
	if (opts.apiPrefix) prefixes.set(client, opts.apiPrefix);
	return client;
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
const prefixOf = (client: AnyClient, apiPrefix?: string) => apiPrefix ?? prefixes.get(client) ?? '';

/**
 * The signed-in user, or null when the browser has no session (401). The
 * prefix defaults to the one the client was made with.
 */
export async function getMe(
	client: AnyClient = createApiClient<BasePaths>(),
	opts: { apiPrefix?: string } = {}
): Promise<Me | null> {
	const path = mePath(prefixOf(client, opts.apiPrefix));
	const { data, response } = await base(client).GET(path as '/me');
	if (response.status === 401) return null;
	if (!response.ok || !data) throw new ApiError(response.status, `GET ${path}`);
	return data;
}

/**
 * Sign the browser out: POST /auth/logout, which deletes the session and
 * clears the cookie. The cookie is HttpOnly, so only the API can clear it.
 */
export async function signOut(
	client: AnyClient = createApiClient<BasePaths>(),
	opts: { apiPrefix?: string } = {}
): Promise<void> {
	const path = logoutPath(prefixOf(client, opts.apiPrefix));
	const { response } = await base(client).POST(path as '/auth/logout');
	if (!response.ok) throw new ApiError(response.status, `POST ${path}`);
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
export async function listSiteSettings(client: AnyClient = createApiClient<BasePaths>()): Promise<SiteSetting[]> {
	const { data, error, response } = await admin(client).GET('/admin/settings');
	if (!response.ok || !data) return failed(response, 'GET /admin/settings', error);
	return data.settings;
}

/** Change a setting. The API refuses a value the setting does not allow with 422. */
export async function updateSiteSetting(
	key: string,
	value: string,
	client: AnyClient = createApiClient<BasePaths>()
): Promise<SiteSetting> {
	const { data, error, response } = await admin(client).PUT('/admin/settings/{key}', {
		params: { path: { key } },
		body: { value }
	});
	if (!response.ok || !data) return failed(response, 'PUT /admin/settings/{key}', error);
	return data;
}

/** Every invite, newest first, whatever its state. */
export async function listSiteInvites(client: AnyClient = createApiClient<BasePaths>()): Promise<SiteInvite[]> {
	const { data, error, response } = await admin(client).GET('/admin/invites');
	if (!response.ok || !data) return failed(response, 'GET /admin/invites', error);
	return data.invites;
}

/** Invite an email address. 409 when it already has a pending invite, 422 for a bad address. */
export async function createSiteInvite(email: string, client: AnyClient = createApiClient<BasePaths>()): Promise<SiteInvite> {
	const { data, error, response } = await admin(client).POST('/admin/invites', { body: { email } });
	if (!response.ok || !data) return failed(response, 'POST /admin/invites', error);
	return data;
}

/** Withdraw a pending invite. */
export async function revokeSiteInvite(id: number, client: AnyClient = createApiClient<BasePaths>()): Promise<void> {
	const { error, response } = await admin(client).DELETE('/admin/invites/{id}', { params: { path: { id } } });
	if (!response.ok) return failed(response, 'DELETE /admin/invites/{id}', error);
}

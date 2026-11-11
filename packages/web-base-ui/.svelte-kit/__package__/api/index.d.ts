import { type Client } from 'openapi-fetch';
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
            responses: {
                200: {
                    content: {
                        'application/json': Me;
                    };
                };
            };
        };
    };
    '/auth/logout': {
        post: {
            responses: {
                204: unknown;
            };
        };
    };
}
/** A role as the account view shows it, for example "Admin (global)". */
export declare function roleLabel(r: MeRole): string;
/**
 * Where the browser goes to sign in. It is a full-page redirect flow (the
 * API redirects to Google and back, then to `/`), not a typed call, so it is
 * a path rather than a client method. Links to it must bypass the SvelteKit
 * router (`data-sveltekit-reload`).
 */
export declare const SIGN_IN_PATH = "/auth/google";
/** Ends the session. POST, answered with 204 and a cleared cookie. */
export declare const LOGOUT_PATH = "/auth/logout";
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
export declare function createApiClient<P extends BasePaths = BasePaths>(opts?: {
    baseUrl?: string;
    fetch?: typeof globalThis.fetch;
}): Client<P>;
/** The API answered something other than a user or "not signed in". */
export declare class ApiError extends Error {
    readonly status: number;
    readonly operation: string;
    constructor(status: number, operation?: string);
}
type AnyClient = Client<any>;
/** The signed-in user, or null when the browser has no session (401). */
export declare function getMe(client?: AnyClient): Promise<Me | null>;
/**
 * Sign the browser out: POST /auth/logout, which deletes the session and
 * clears the cookie. The cookie is HttpOnly, so only the API can clear it.
 */
export declare function signOut(client?: AnyClient): Promise<void>;
export {};

import { describe, it, expect, vi } from 'vitest';
import { createApiClient, getMe, signOut, roleLabel, ApiError, SIGN_IN_PATH, LOGOUT_PATH, logoutPath, mePath, type Me } from './index';

const andrew: Me = {
	id: '104242424242424242424',
	name: 'Andrew',
	email: 'andrew@example.com',
	avatar: 'https://lh3.googleusercontent.com/a/andrew',
	roles: [{ role: 'Admin', domain: 'global' }]
};

function json(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': status < 400 ? 'application/json' : 'application/problem+json' }
	});
}

/** A client whose fetch records each request and answers with res. */
function clientAnswering(res: Response) {
	const seen: Request[] = [];
	const fetch = vi.fn(async (input: Request) => {
		seen.push(input);
		return res;
	}) as unknown as typeof globalThis.fetch;
	return { client: createApiClient({ baseUrl: 'https://app.test', fetch }), seen };
}

describe('paths', () => {
	it('signs in and out at the API routes', () => {
		expect(SIGN_IN_PATH).toBe('/auth/google');
		expect(LOGOUT_PATH).toBe('/auth/logout');
	});
});

describe('getMe', () => {
	it('returns the user from GET /me, sending the session cookie', async () => {
		const { client, seen } = clientAnswering(json(200, andrew));
		expect(await getMe(client)).toEqual(andrew);
		expect(seen).toHaveLength(1);
		expect(seen[0].method).toBe('GET');
		expect(new URL(seen[0].url).pathname).toBe('/me');
		expect(seen[0].credentials).toBe('same-origin');
	});

	it('returns null when the API answers 401', async () => {
		const { client } = clientAnswering(json(401, { status: 401, title: 'Unauthorized' }));
		expect(await getMe(client)).toBeNull();
	});

	it('throws on any other failure rather than showing a signed-out page', async () => {
		const { client } = clientAnswering(json(503, { status: 503 }));
		await expect(getMe(client)).rejects.toEqual(new ApiError(503));
	});
});

describe('signOut', () => {
	it('posts to /auth/logout through the generated client, sending the session cookie', async () => {
		const { client, seen } = clientAnswering(new Response(null, { status: 204 }));
		await signOut(client);
		expect(seen).toHaveLength(1);
		expect(seen[0].method).toBe('POST');
		expect(new URL(seen[0].url).pathname).toBe('/auth/logout');
		expect(seen[0].credentials).toBe('same-origin');
	});

	it('throws when the API refuses', async () => {
		const { client } = clientAnswering(json(503, { status: 503 }));
		await expect(signOut(client)).rejects.toEqual(new ApiError(503, 'POST /auth/logout'));
	});
});

describe('roleLabel', () => {
	it('names the role and the domain it applies in', () => {
		expect(roleLabel({ role: 'Admin', domain: 'global' })).toBe('Admin (global)');
		expect(roleLabel({ role: 'Contributor', domain: 'proj-a' })).toBe('Contributor (proj-a)');
	});
});

describe('apiPrefix', () => {
	// A paths map keyed like huddle-go's generated one.
	interface prefixed {
		'/api/v1/me': {
			get: { responses: { 200: { content: { 'application/json': Me } } } };
		};
		'/api/v1/auth/logout': { post: { responses: { 204: unknown } } };
		'/api/v1/other': { get: { responses: { 200: unknown } } };
	}

	function prefixedClient(res: Response) {
		const seen: Request[] = [];
		const fetch = (async (input: Request) => (seen.push(input), res)) as unknown as typeof globalThis.fetch;
		const client = createApiClient<prefixed>({ baseUrl: 'https://app.test', fetch, apiPrefix: '/api/v1' });
		return { client, seen };
	}

	it('getMe and signOut call the prefixed paths using the prefix the client was made with', async () => {
		const me = prefixedClient(json(200, andrew));
		expect(await getMe(me.client)).toEqual(andrew);
		expect(new URL(me.seen[0].url).pathname).toBe('/api/v1/me');
		const out = prefixedClient(new Response(null, { status: 204 }));
		await signOut(out.client);
		expect(new URL(out.seen[0].url).pathname).toBe('/api/v1/auth/logout');
		expect(logoutPath('/api/v1')).toBe('/api/v1/auth/logout');
		expect(mePath('/api/v1')).toBe('/api/v1/me');
	});

	it('an explicit apiPrefix on getMe and signOut wins, and errors name the prefixed operation', async () => {
		const seen: Request[] = [];
		const fetch = (async (input: Request) => (seen.push(input), json(503, { status: 503 }))) as unknown as typeof globalThis.fetch;
		const client = createApiClient({ baseUrl: 'https://app.test', fetch });
		await expect(getMe(client, { apiPrefix: '/api/v1' })).rejects.toEqual(new ApiError(503, 'GET /api/v1/me'));
		await expect(signOut(client, { apiPrefix: '/api/v1' })).rejects.toEqual(new ApiError(503, 'POST /api/v1/auth/logout'));
		expect(seen.map((r) => new URL(r.url).pathname)).toEqual(['/api/v1/me', '/api/v1/auth/logout']);
	});

	it('type checks only for a prefix the paths offer', () => {
		// @ts-expect-error '/api/v2' is not a prefix of /me and /auth/logout in these paths
		createApiClient<prefixed>({ apiPrefix: '/api/v2' });
		// @ts-expect-error these paths have no unprefixed /me
		createApiClient<prefixed>();
		interface noMe { '/other': { get: { responses: { 200: unknown } } } }
		// @ts-expect-error no /me and /auth/logout at all
		createApiClient<noMe>({});
	});
});

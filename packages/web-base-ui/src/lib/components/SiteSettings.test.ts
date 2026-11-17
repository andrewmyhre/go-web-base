import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import SiteSettings from './SiteSettings.svelte';
import { createApiClient, type SiteInvite, type SiteSetting } from '../api/index';

const mode = (value: string): SiteSetting => ({
	key: 'sign_up_mode',
	label: 'Sign-up',
	description: 'open lets anyone in; invite_only needs an invite.',
	options: ['open', 'invite_only'],
	value,
	default: 'open'
});

const invite = (id: number, email: string, status: SiteInvite['status']): SiteInvite => ({
	id,
	email,
	status,
	created_at: '2026-11-17T00:00:00Z'
});

function json(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': status < 400 ? 'application/json' : 'application/problem+json' }
	});
}

/** A fake API: handlers by "METHOD /path", recording every request made. */
function fakeApi(handlers: Record<string, (req: Request) => Response | Promise<Response>>) {
	const calls: string[] = [];
	const fetch = vi.fn(async (req: Request) => {
		const key = `${req.method} ${new URL(req.url).pathname}`;
		calls.push(key);
		const h = handlers[key];
		return h ? h(req) : json(404, { status: 404 });
	}) as unknown as typeof globalThis.fetch;
	return { client: createApiClient({ baseUrl: 'https://app.test', fetch }), calls };
}

describe('SiteSettings', () => {
	it('lists the settings and the invites', async () => {
		const { client } = fakeApi({
			'GET /admin/settings': () => json(200, { settings: [mode('invite_only')] }),
			'GET /admin/invites': () => json(200, { invites: [invite(2, 'a@example.com', 'pending'), invite(1, 'b@example.com', 'consumed')] })
		});
		render(SiteSettings, { props: { client } });
		const select = (await screen.findByLabelText('Sign-up')) as HTMLSelectElement;
		expect(select.value).toBe('invite_only');
		expect(screen.getByText('a@example.com')).toBeTruthy();
		// Only a pending invite can be revoked.
		expect(screen.getAllByRole('button', { name: /Revoke the invite for/ })).toHaveLength(1);
	});

	it('says only an administrator can use it when the API answers 403', async () => {
		const { client } = fakeApi({
			'GET /admin/settings': () => json(403, { status: 403, detail: 'forbidden' }),
			'GET /admin/invites': () => json(403, { status: 403, detail: 'forbidden' })
		});
		render(SiteSettings, { props: { client } });
		expect((await screen.findByRole('alert')).textContent).toContain('Only an administrator');
		expect(screen.queryByLabelText('Sign-up')).toBeNull();
	});

	it('saves a changed setting with PUT and shows the new value', async () => {
		let stored = 'open';
		let sent = '';
		const { client, calls } = fakeApi({
			'GET /admin/settings': () => json(200, { settings: [mode(stored)] }),
			'GET /admin/invites': () => json(200, { invites: [] }),
			'PUT /admin/settings/sign_up_mode': async (req) => {
				sent = ((await req.json()) as { value: string }).value;
				stored = sent;
				return json(200, mode(stored));
			}
		});
		render(SiteSettings, { props: { client } });
		const select = (await screen.findByLabelText('Sign-up')) as HTMLSelectElement;
		const save = screen.getByRole('button', { name: 'Save' }) as HTMLButtonElement;
		expect(save.disabled).toBe(true); // nothing changed yet
		await fireEvent.change(select, { target: { value: 'invite_only' } });
		await waitFor(() => expect(save.disabled).toBe(false));
		await fireEvent.click(save);
		await waitFor(() => expect(sent).toBe('invite_only'));
		expect((await screen.findByRole('status')).textContent).toContain('Sign-up saved');
		expect(calls.filter((c) => c.startsWith('GET /admin/settings')).length).toBe(2); // reloaded
	});

	it('invites an address and clears the field', async () => {
		let body = '';
		const { client } = fakeApi({
			'GET /admin/settings': () => json(200, { settings: [mode('invite_only')] }),
			'GET /admin/invites': () => json(200, { invites: [] }),
			'POST /admin/invites': async (req) => {
				body = JSON.stringify(await req.json());
				return json(200, invite(1, 'new@example.com', 'pending'));
			}
		});
		render(SiteSettings, { props: { client } });
		const input = (await screen.findByLabelText('Email address')) as HTMLInputElement;
		await fireEvent.input(input, { target: { value: ' new@example.com ' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Invite' }));
		await waitFor(() => expect(body).toBe('{"email":"new@example.com"}'));
		await waitFor(() => expect(input.value).toBe(''));
	});

	it('shows the API reason when an invite is refused', async () => {
		const { client } = fakeApi({
			'GET /admin/settings': () => json(200, { settings: [mode('open')] }),
			'GET /admin/invites': () => json(200, { invites: [] }),
			'POST /admin/invites': () => json(409, { status: 409, detail: 'an invite for this address is already pending' })
		});
		render(SiteSettings, { props: { client } });
		await fireEvent.input(await screen.findByLabelText('Email address'), { target: { value: 'dup@example.com' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Invite' }));
		expect((await screen.findByRole('status')).textContent).toContain('already pending');
	});

	it('revokes a pending invite with DELETE', async () => {
		let deleted = '';
		const { client } = fakeApi({
			'GET /admin/settings': () => json(200, { settings: [mode('invite_only')] }),
			'GET /admin/invites': () => json(200, { invites: [invite(7, 'a@example.com', 'pending')] }),
			'DELETE /admin/invites/7': () => {
				deleted = 'yes';
				return new Response(null, { status: 204 });
			}
		});
		render(SiteSettings, { props: { client } });
		await fireEvent.click(await screen.findByRole('button', { name: 'Revoke the invite for a@example.com' }));
		await waitFor(() => expect(deleted).toBe('yes'));
	});

	it('offers a retry when the API is down', async () => {
		const { client } = fakeApi({
			'GET /admin/settings': () => json(503, { status: 503 }),
			'GET /admin/invites': () => json(503, { status: 503 })
		});
		render(SiteSettings, { props: { client } });
		expect((await screen.findByRole('alert')).textContent).toContain('Could not load');
		expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy();
	});
});

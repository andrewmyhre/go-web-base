import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import AccountMenu from './AccountMenu.svelte';
import type { Me } from '$lib/api';

const andrew: Me = {
	id: '104242424242424242424',
	name: 'Andrew Myhre',
	email: 'andrew@example.com',
	avatar: 'https://lh3.googleusercontent.com/a/andrew',
	roles: [{ role: 'Admin', domain: 'global' }]
};

function setup(user: Me = andrew) {
	const onSignOut = vi.fn();
	render(AccountMenu, { props: { user, onSignOut } });
	const avatar = screen.getByRole('button', { name: 'Account menu for Andrew Myhre' });
	return { avatar, onSignOut };
}

async function openAccount(avatar: HTMLElement) {
	await fireEvent.click(avatar);
	await fireEvent.click(screen.getByRole('button', { name: 'Account' }));
	return screen.getByRole('dialog', { name: 'Account' });
}

describe('AccountMenu', () => {
	it('the avatar is a button that reports whether its menu is open', async () => {
		const { avatar } = setup();
		expect(avatar.getAttribute('aria-expanded')).toBe('false');
		expect(screen.queryByRole('button', { name: 'Sign out' })).toBeNull();
		await fireEvent.click(avatar);
		expect(avatar.getAttribute('aria-expanded')).toBe('true');
		expect(screen.getByRole('button', { name: 'Account' })).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy();
	});

	it('Escape closes the menu and returns focus to the avatar', async () => {
		const { avatar } = setup();
		await fireEvent.click(avatar);
		await fireEvent.keyDown(screen.getByRole('button', { name: 'Account' }), { key: 'Escape' });
		expect(avatar.getAttribute('aria-expanded')).toBe('false');
		expect(document.activeElement).toBe(avatar);
	});

	it('Sign out calls onSignOut and closes the menu', async () => {
		const { avatar, onSignOut } = setup();
		await fireEvent.click(avatar);
		await fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
		expect(onSignOut).toHaveBeenCalledTimes(1);
		expect(avatar.getAttribute('aria-expanded')).toBe('false');
	});

	it('Account opens a labelled modal dialog with focus inside it', async () => {
		const { avatar } = setup();
		const dialog = (await openAccount(avatar)) as HTMLDialogElement;
		expect(dialog.open).toBe(true);
		expect(dialog.getAttribute('aria-modal')).toBe('true');
		expect(document.getElementById(dialog.getAttribute('aria-labelledby')!)?.textContent).toBe('Account');
		expect(dialog.contains(document.activeElement)).toBe(true);
	});

	it('the modal shows the avatar, name, email and the roles from /me', async () => {
		const { avatar } = setup({
			...andrew,
			roles: [
				{ role: 'Contributor', domain: 'proj-a' },
				{ role: 'Manager', domain: 'proj-b' }
			]
		});
		const dialog = await openAccount(avatar);
		expect(dialog.textContent).toContain('Andrew Myhre');
		expect(dialog.textContent).toContain('andrew@example.com');
		expect(dialog.querySelector('img.avatar')?.getAttribute('src')).toBe(andrew.avatar);
		const roles = [...dialog.querySelectorAll('.roles li')].map((li) => li.textContent);
		expect(roles).toEqual(['Contributor (proj-a)', 'Manager (proj-b)']);
	});

	it('the modal says so when the user has no roles', async () => {
		const { avatar } = setup({ ...andrew, roles: [] });
		const dialog = await openAccount(avatar);
		expect(dialog.querySelectorAll('.roles li')).toHaveLength(0);
		expect(dialog.textContent).toContain('No roles.');
	});

	it('Close closes the modal and returns focus to the avatar (IMM-2001)', async () => {
		const { avatar } = setup();
		const dialog = (await openAccount(avatar)) as HTMLDialogElement;
		expect(document.activeElement).not.toBe(avatar);
		await fireEvent.click(screen.getByRole('button', { name: 'Close' }));
		expect(dialog.open).toBe(false);
		expect(document.activeElement).toBe(avatar);
	});

	it('Escape closes the modal and returns focus to the avatar', async () => {
		const { avatar } = setup();
		const dialog = (await openAccount(avatar)) as HTMLDialogElement;
		await fireEvent.keyDown(dialog, { key: 'Escape' });
		expect(dialog.open).toBe(false);
		expect(document.activeElement).toBe(avatar);
	});
});

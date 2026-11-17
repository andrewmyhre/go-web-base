import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';

vi.mock('$app/state', () => ({ page: { url: new URL('https://app.test/') } }));

import SideMenu from './SideMenu.svelte';

describe('SideMenu', () => {
	it('links Home to / and marks it current on the home page', () => {
		render(SideMenu);
		const link = screen.getByRole('link', { name: 'Home' });
		expect(link.getAttribute('href')).toBe('/');
		expect(link.getAttribute('aria-current')).toBe('page');
	});

	it('without props renders exactly the single Home list', () => {
		const { container } = render(SideMenu);
		expect(container.querySelector('nav')?.innerHTML.replace(/<!--.*?-->/g, '').trim()).toBe(
			'<ul><li><a href="/" aria-current="page">Home</a></li></ul>'
		);
	});

	it('renders items in order, grouped under headings, omitting hidden ones', () => {
		render(SideMenu, {
			props: {
				items: [
					{ label: 'Home', href: '/' },
					{ label: 'Providers', href: '/providers', visible: false },
					{ label: 'Projects', href: '/projects' },
					{ label: 'Activity', href: '/activity', group: 'Admin' },
					{ label: 'Admin', href: '/admin', group: 'Admin', visible: true },
					{ label: 'Lab', href: '/lab', visible: false }
				]
			}
		});
		expect(screen.queryByRole('link', { name: 'Providers' })).toBeNull();
		expect(screen.queryByRole('link', { name: 'Lab' })).toBeNull();
		const lists = screen.getAllByRole('list');
		expect(lists).toHaveLength(2);
		expect(Array.from(lists[0].querySelectorAll('a')).map((a) => a.textContent)).toEqual(['Home', 'Projects']);
		const admin = screen.getByRole('list', { name: 'Admin' });
		expect(Array.from(admin.querySelectorAll('a')).map((a) => a.getAttribute('href'))).toEqual(['/activity', '/admin']);
	});

	it('omits a group heading when every item in it is hidden', () => {
		render(SideMenu, {
			props: { items: [{ label: 'Home', href: '/' }, { label: 'Admin', href: '/admin', group: 'Admin', visible: false }] }
		});
		expect(screen.queryByText('Admin')).toBeNull();
		expect(screen.getAllByRole('list')).toHaveLength(1);
	});

	it('marks the current item, and lets an item override it', () => {
		render(SideMenu, {
			props: {
				items: [
					{ label: 'Home', href: '/' },
					{ label: 'Orgs', href: '/orgs', current: true },
					{ label: 'Lab', href: '/lab', current: false }
				]
			}
		});
		expect(screen.getByRole('link', { name: 'Home' }).getAttribute('aria-current')).toBe('page');
		expect(screen.getByRole('link', { name: 'Orgs' }).getAttribute('aria-current')).toBe('page');
		expect(screen.getByRole('link', { name: 'Lab' }).getAttribute('aria-current')).toBeNull();
	});
});

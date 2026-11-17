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
		// Same bytes as main, apart from Svelte's comment anchors: no extra whitespace nodes.
		expect(container.innerHTML.replace(/<!---->/g, '')).toBe(
			'<nav class="menu" aria-label="Main"><ul><li><a href="/" aria-current="page">Home</a></li></ul></nav>'
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

	it('groups across several groups in order of first appearance, collecting non-adjacent items', () => {
		render(SideMenu, {
			props: {
				items: [
					{ label: 'A1', href: '/a1', group: 'Alpha' },
					{ label: 'Top', href: '/top' },
					{ label: 'B1', href: '/b1', group: 'Beta' },
					{ label: 'A2', href: '/a2', group: 'Alpha' },
					{ label: 'B2', href: '/b2', group: 'Beta' },
					{ label: 'Top2', href: '/top2' }
				]
			}
		});
		const lists = screen.getAllByRole('list');
		expect(lists).toHaveLength(3);
		const names = (el: HTMLElement) => Array.from(el.querySelectorAll('a')).map((a) => a.textContent);
		expect(names(lists[0])).toEqual(['Top', 'Top2']);
		expect(names(screen.getByRole('list', { name: 'Alpha' }))).toEqual(['A1', 'A2']);
		expect(names(screen.getByRole('list', { name: 'Beta' }))).toEqual(['B1', 'B2']);
		expect(lists[1].previousElementSibling?.textContent).toBe('Alpha');
		expect(lists[2].previousElementSibling?.textContent).toBe('Beta');
	});

	it('does not crash on a duplicate href', () => {
		render(SideMenu, { props: { items: [{ label: 'One', href: '/x' }, { label: 'Two', href: '/x' }, { label: 'Three', href: '/x', group: 'G' }, { label: 'Four', href: '/x', group: 'G' }] } });
		expect(screen.getAllByRole('link')).toHaveLength(4);
	});

	it('gives two menus distinct heading ids', () => {
		const items = [{ label: 'A', href: '/a', group: 'Admin' }];
		const one = render(SideMenu, { props: { items } });
		const two = render(SideMenu, { props: { items } });
		const ids = [...one.container.querySelectorAll('.group'), ...two.container.querySelectorAll('.group')].map((e) => e.id);
		expect(ids).toHaveLength(2);
		expect(new Set(ids).size).toBe(2);
		for (const c of [one.container, two.container]) {
			const h = c.querySelector('.group')!;
			expect(c.querySelector('ul')!.getAttribute('aria-labelledby')).toBe(h.id);
		}
	});
});

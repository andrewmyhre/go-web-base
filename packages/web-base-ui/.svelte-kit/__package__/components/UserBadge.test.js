import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import UserBadge from './UserBadge.svelte';
describe('UserBadge', () => {
    it('shows the name and the avatar', () => {
        const { container } = render(UserBadge, {
            props: { name: 'Andrew Myhre', avatar: 'https://lh3.googleusercontent.com/a/andrew' }
        });
        expect(screen.getByText('Andrew Myhre')).toBeTruthy();
        const img = container.querySelector('img.avatar');
        expect(img?.getAttribute('src')).toBe('https://lh3.googleusercontent.com/a/andrew');
        expect(img?.getAttribute('referrerpolicy')).toBe('no-referrer');
    });
    it('falls back to initials when there is no avatar', () => {
        const { container } = render(UserBadge, { props: { name: 'Andrew Myhre', avatar: '' } });
        expect(container.querySelector('img')).toBeNull();
        expect(screen.getByText('AM')).toBeTruthy();
    });
});

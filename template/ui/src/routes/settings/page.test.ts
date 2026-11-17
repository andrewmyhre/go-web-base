import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import Page from './+page.svelte';

// The page hands the API client to the shared component. The test setup has
// no API, so the fetch fails and the component shows its error: enough to
// prove the page renders the component and that it reached for the settings.
describe('settings page', () => {
	it('renders the site settings component', async () => {
		render(Page);
		expect(screen.getByRole('heading', { name: 'Settings' })).toBeTruthy();
		expect(await screen.findByRole('alert')).toBeTruthy();
	});
});

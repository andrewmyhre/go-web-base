import { describe, it, expect } from 'vitest';
import { isApiPath, strategyFor } from './sw-policy';

const origin = 'https://app.test';
// A precache that wrongly lists API paths too: the bypass must still win.
const precached = new Set(['/', '/_app/immutable/entry/start.js', '/manifest.webmanifest', '/me', '/auth/google']);

const get = (path: string, mode = 'cors') => ({ method: 'GET', url: origin + path, mode });

describe('strategyFor', () => {
	it.each([
		'/auth/google',
		'/auth/google/callback?state=s&code=c',
		'/auth/logout',
		'/me',
		'/health',
		'/openapi.json'
	])('never handles API path %s, as a fetch or as a navigation', (path) => {
		expect(strategyFor(get(path), origin, precached)).toBe('bypass');
		expect(strategyFor(get(path, 'navigate'), origin, precached)).toBe('bypass');
	});

	it('bypasses anything but GET', () => {
		expect(strategyFor({ method: 'POST', url: origin + '/', mode: 'cors' }, origin, precached)).toBe('bypass');
	});

	it('bypasses other origins, such as the Google avatar', () => {
		const avatar = { method: 'GET', url: 'https://lh3.googleusercontent.com/a/andrew', mode: 'no-cors' };
		expect(strategyFor(avatar, origin, precached)).toBe('bypass');
	});

	it('serves precached build assets from the cache', () => {
		expect(strategyFor(get('/_app/immutable/entry/start.js'), origin, precached)).toBe('cache-first');
		expect(strategyFor(get('/manifest.webmanifest'), origin, precached)).toBe('cache-first');
	});

	it('treats an app navigation as the shell, even to a precached path', () => {
		expect(strategyFor(get('/settings', 'navigate'), origin, precached)).toBe('shell');
		expect(strategyFor(get('/', 'navigate'), origin, precached)).toBe('shell');
	});

	it('leaves an unknown non-navigation request alone', () => {
		expect(strategyFor(get('/unknown.png'), origin, precached)).toBe('bypass');
	});
});

describe('extra API prefixes', () => {
	it('bypasses a project API path, as a fetch or a navigation', () => {
		expect(strategyFor(get('/reports'), origin, precached, ['/reports'])).toBe('bypass');
		expect(strategyFor(get('/reports/7', 'navigate'), origin, precached, ['/reports'])).toBe('bypass');
		expect(strategyFor(get('/reports', 'navigate'), origin, precached)).toBe('shell');
	});
});

describe('isApiPath', () => {
	it('matches whole segments only', () => {
		expect(isApiPath('/me')).toBe(true);
		expect(isApiPath('/me/')).toBe(true);
		expect(isApiPath('/meetings')).toBe(false);
		expect(isApiPath('/authors')).toBe(false);
		expect(isApiPath('/')).toBe(false);
	});
});

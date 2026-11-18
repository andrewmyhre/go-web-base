import { describe, it, expect } from 'vitest';
import { existsSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { styles, findStyle, DEFAULT_STYLE, PAIRS, ratio, REQUIRED_TOKENS, resolveTokens, styleCss, cssErrors, fontHref, fontSpecValid } from './index';
import { BASE_CSS } from './base';

const here = __dirname;
const repo = resolve(here, '../../../../..');
const HEX = /^#[0-9a-f]{6}$/i;
const slugs = styles.map((s) => s.slug);

// `pnpm run generate:styles` rewrites css/<slug>.css from the data; without
// UPDATE_STYLE_CSS this file only compares.
const update = process.env.UPDATE_STYLE_CSS === '1';
const cssFile = (slug: string) => resolve(here, 'css', slug + '.css');

describe('the style list', () => {
	it('has 20 styles, sorted and unique by slug', () => {
		expect(styles).toHaveLength(20);
		expect(slugs).toEqual([...slugs].sort());
		expect(new Set(slugs).size).toBe(slugs.length);
		expect(new Set(styles.map((s) => s.name)).size).toBe(styles.length);
	});
	it('has one data file per style, each named after its slug', () => {
		const files = readdirSync(resolve(here, 'data')).map((f) => f.replace(/\.ts$/, '')).sort();
		expect(files).toEqual(slugs);
	});
	it('finds a style by slug, and the default is none', () => {
		expect(findStyle('broadsheet')?.name).toBe('Broadsheet');
		expect(findStyle(DEFAULT_STYLE)).toBeUndefined();
		expect(findStyle(undefined)).toBeUndefined();
		expect(DEFAULT_STYLE).toBe('default');
	});
	it('uses neutral names: no film, label, brand, place or person', () => {
		const banned = /murder|blue note|budapest|third man|nostromo|alien|poppy|marimekko|bauhaus|weimar|memphis|milano|penguin|blade runner|wes anderson|saul bass|macintosh|\bmac\b|apple|chrysler|gatsby|\bbeck\b|sottsass|tschichold|reid miles|bayer/i;
		for (const s of styles) {
			expect(s.slug + ' ' + s.name + ' ' + s.description, s.slug).not.toMatch(banned);
			expect(s.slug, s.slug).toMatch(/^[a-z][a-z0-9]*(-[a-z0-9]+)*$/);
			expect(s.description.trim(), s.slug).not.toBe('');
			expect(s.description, s.slug).not.toContain('\n');
		}
	});
});

describe('font specs', () => {
	it('accepts css2 family specs and rejects malformed ones', () => {
		for (const ok of ['Inter', 'Space Grotesk:wght@400;700', 'Fraunces:opsz,wght@9..144,400;9..144,700', 'Lora:ital,wght@0,400;1,400'])
			expect(fontSpecValid(ok), ok).toBe(true);
		for (const bad of ['inter', 'Inter:wght400', 'Inter:wght@', 'Lora:ital,wght@400;700', 'Inter:bogus@400', 'Inter wght@400', 'A:wght@4 00'])
			expect(fontSpecValid(bad), bad).toBe(false);
	});
	it('builds the Google Fonts URL', () => {
		expect(fontHref({ fonts: ['Anton:wght@400', 'Source Sans 3:wght@400;600'] })).toBe(
			'https://fonts.googleapis.com/css2?family=Anton:wght@400&family=Source+Sans+3:wght@400;600&display=swap'
		);
	});
});

describe('extra css', () => {
	it('rejects @import, @font-face, url(), external urls and unbalanced braces', () => {
		expect(cssErrors('h1 { color: red }')).toEqual([]);
		expect(cssErrors("@import 'x.css';")).toHaveLength(1);
		expect(cssErrors('@font-face { font-family: x }')).toHaveLength(1);
		expect(cssErrors('h1 { background: url(a.png) }').length).toBeGreaterThan(0);
		expect(cssErrors('h1 { color: red')).toEqual(['unbalanced braces']);
	});
});

describe.each(styles.map((s) => [s.slug, s] as const))('style %s', (_slug, s) => {
	it('has the required tokens in both themes', () => {
		for (const theme of ['light', 'dark'] as const) for (const t of REQUIRED_TOKENS) expect(s[theme][t], `${theme} ${t}`).toBeTruthy();
	});
	it('gives both themes the same tokens, all named --wb-*', () => {
		expect(Object.keys(s.dark).sort()).toEqual(Object.keys(s.light).sort());
		for (const k of Object.keys(s.light)) expect(k).toMatch(/^--wb-/);
	});
	it('uses at most 2 font families and 4 font styles, validly', () => {
		expect(s.fonts.length).toBeGreaterThan(0);
		expect(s.fonts.length).toBeLessThanOrEqual(2);
		expect(s.fonts.reduce((n, f) => n + (f.includes('@') ? f.split('@')[1].split(';').length : 1), 0)).toBeLessThanOrEqual(4);
		for (const f of s.fonts) expect(fontSpecValid(f), f).toBe(true);
	});
	it('has a body font with a generic fallback that is not the heading font', () => {
		for (const theme of ['light', 'dark'] as const) {
			expect(s[theme]['--wb-font-body']).toMatch(/(serif|monospace)\s*$/);
			expect(s[theme]['--wb-font-body']).not.toBe(s[theme]['--wb-font-heading']);
		}
	});
	it('has safe extra css', () => {
		expect(cssErrors(s.css ?? '')).toEqual([]);
	});
	it('names no role-up or per-app selector', () => {
		expect(s.css ?? '').not.toMatch(/data-concept|\.ru-|--ru-/);
	});
	for (const [theme, idx] of [['light', 0], ['dark', 1]] as const) {
		for (const [fg, bg, min] of PAIRS) {
			it(`${theme}: ${fg} on ${bg} is at least ${min}:1`, () => {
				const t = resolveTokens(s);
				const f = t[`--wb-${fg}`]?.[idx];
				const b = t[`--wb-${bg}`]?.[idx];
				expect(f, fg).toMatch(HEX);
				expect(b, bg).toMatch(HEX);
				expect(ratio(f, b)).toBeGreaterThanOrEqual(min);
			});
		}
	}
	it('has a generated css file that matches the data', () => {
		const want = styleCss(s);
		if (update) writeFileSync(cssFile(s.slug), want);
		expect(existsSync(cssFile(s.slug)), `run pnpm run generate:styles to make css/${s.slug}.css`).toBe(true);
		expect(readFileSync(cssFile(s.slug), 'utf8')).toBe(want);
	});
});

describe('the generated css', () => {
	const s = styles[0];
	const css = styleCss(s);
	it('starts with the font import, then tokens as light-dark(), then the shared rules', () => {
		expect(css.indexOf('@import url(')).toBeLessThan(css.indexOf(':root {'));
		expect(css).toContain(fontHref(s));
		expect(css).toMatch(/--wb-bg: light-dark\(#[0-9a-f]{6}, #[0-9a-f]{6}\);/i);
		expect(css.indexOf(':root {')).toBeLessThan(css.indexOf(BASE_CSS));
	});
	it('puts the style css after the shared rules, so it can override them', () => {
		expect(css.indexOf(BASE_CSS)).toBeLessThan(css.indexOf(`/* ${s.name} */`));
		expect(css.indexOf(`/* ${s.name} */`)).toBeGreaterThan(0);
	});
	it('gives a token set for one theme to both, and fills a missing overlay', () => {
		const t = resolveTokens({ light: { '--wb-bg': '#ffffff' }, dark: {} });
		expect(t['--wb-bg']).toEqual(['#ffffff', '#ffffff']);
		expect(t['--wb-overlay']).toBeDefined();
	});
	it('leaves no unfilled ${...} or [[ ]] in any file', () => {
		for (const slug of slugs) {
			const text = readFileSync(cssFile(slug), 'utf8');
			expect(text).not.toMatch(/\$\{|\[\[|\]\]/);
		}
	});
});

describe('every component the package renders is styled', () => {
	const dir = resolve(here, '../components');
	const sources = readdirSync(dir).filter((f) => f.endsWith('.svelte')).map((f) => [f, readFileSync(resolve(dir, f), 'utf8')] as const);
	// The wrapper that only positions the menu; app.css does that.
	const structural = new Set(['account-menu']);
	it('found the components', () => {
		expect(sources.length).toBeGreaterThanOrEqual(6);
	});
	for (const [file, src] of sources) {
		it(`${file}: every class it emits has a rule in the shared css`, () => {
			const classes = new Set<string>();
			for (const m of src.matchAll(/\bclass="([^"{}]+)"/g)) for (const c of m[1].split(/\s+/).filter(Boolean)) classes.add(c);
			for (const c of classes) {
				if (structural.has(c)) continue;
				expect(BASE_CSS, `.${c} in ${file}`).toMatch(new RegExp('\\.' + c + '(?![\\w-])'));
			}
		});
		it(`${file}: every form control, table and dialog it emits has a rule`, () => {
			for (const tag of ['button', 'input', 'select', 'table', 'dialog', 'code']) {
				if (!new RegExp('<' + tag + '\\b').test(src)) continue;
				const ok = tag === 'dialog' ? '.account-modal' : tag === 'table' ? '.invites' : tag;
				expect(BASE_CSS, `${tag} in ${file}`).toMatch(new RegExp('(^|[\\s,>])' + ok.replace('.', '\\.') + '\\b', 'm'));
			}
		});
	}
});

describe('the docs and the template agree with the list', () => {
	it('docs/styles.md lists every style with its name and description', () => {
		const doc = readFileSync(resolve(repo, 'docs/styles.md'), 'utf8');
		for (const s of styles) {
			expect(doc, s.slug).toContain('`' + s.slug + '`');
			expect(doc, s.slug).toContain(s.name);
			expect(doc, s.slug).toContain(s.description);
		}
	});
	it('copier.yml offers the default and exactly these styles', () => {
		const yml = readFileSync(resolve(repo, 'copier.yml'), 'utf8');
		const block = /\nstyle:\n([\s\S]*?)\n\n/.exec(yml)?.[1] ?? '';
		const offered = [...block.matchAll(/^ {4}".*": ([a-z0-9-]+)$/gm)].map((m) => m[1]);
		expect(offered).toEqual([DEFAULT_STYLE, ...slugs]);
	});
});

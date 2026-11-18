import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'transit-line',
	name: 'Transit Line',
	description: 'Metro wayfinding: bold line colours under headings, pill-shaped route markers and station-sign lettering.',
	fonts: ['Overpass:wght@400;600;800'],
	light: {
		'--wb-font-heading': "'Overpass', 'Arial Narrow', system-ui, sans-serif",
		'--wb-font-body': "'Overpass', system-ui, sans-serif",
		'--wb-bg': '#f4f4f1',
		'--wb-surface': '#ffffff',
		'--wb-text': '#10151c',
		'--wb-muted': '#4b5563',
		'--wb-accent': '#0b3d91',
		'--wb-accent-text': '#ffffff',
		'--wb-border': '#d6d8dc',
		'--wb-border-strong': '#6b7280',
		'--wb-radius': '6px',
		'--wb-focus': '#c8102e',
		'--wb-success': '#126b2f',
		'--wb-success-bg': '#e1f3e7',
		'--wb-warning': '#7a4b00',
		'--wb-warning-bg': '#fdf0c8',
		'--wb-danger': '#b00020',
		'--wb-danger-bg': '#fde7ea',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0b3d91',
		'--wb-info-bg': '#e3ebf8',
		'--wb-neutral-bg': '#e9eaec'
	},
	dark: {
		'--wb-font-heading': "'Overpass', 'Arial Narrow', system-ui, sans-serif",
		'--wb-font-body': "'Overpass', system-ui, sans-serif",
		'--wb-bg': '#0c1018',
		'--wb-surface': '#141a25',
		'--wb-text': '#f1f3f6',
		'--wb-muted': '#a7b0be',
		'--wb-accent': '#ff5a6e',
		'--wb-accent-text': '#0c1018',
		'--wb-border': '#2a3345',
		'--wb-border-strong': '#7b879b',
		'--wb-radius': '6px',
		'--wb-focus': '#ffd23f',
		'--wb-success': '#5fe08a',
		'--wb-success-bg': '#10261a',
		'--wb-warning': '#ffd23f',
		'--wb-warning-bg': '#2c2410',
		'--wb-danger': '#ff8a96',
		'--wb-danger-bg': '#341519',
		'--wb-danger-text': '#0c1018',
		'--wb-info': '#7fb0ff',
		'--wb-info-bg': '#14233f',
		'--wb-neutral-bg': '#1f2736'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 800;
	letter-spacing: 0.01em;
}
h1 {
	padding-bottom: 0.25em;
	border-bottom: 8px solid var(--wb-accent);
}
h2 {
	padding-left: 0.6em;
	border-left: 6px solid var(--wb-accent);
}
.button,
button {
	border-radius: 999px;
	font-weight: 600;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border-top: 4px solid var(--wb-accent);
}
`
};

export default style;

import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'phosphor-terminal',
	name: 'Phosphor Terminal',
	description: 'A 1970s ship-computer console: green phosphor on black, monospaced capitals, scan lines and bracket-framed cards.',
	fonts: ['Share Tech Mono:wght@400', 'IBM Plex Sans:wght@400;600'],
	light: {
		'--wb-font-heading': "'Share Tech Mono', ui-monospace, 'Courier New', monospace",
		'--wb-font-body': "'IBM Plex Sans', system-ui, sans-serif",
		'--wb-bg': '#dfe8e0',
		'--wb-surface': '#f1f6f1',
		'--wb-border': '#b4c8b8',
		'--wb-border-strong': '#4f7a5c',
		'--wb-text': '#06210f',
		'--wb-muted': '#3d5a46',
		'--wb-accent': '#0b6b2e',
		'--wb-accent-text': '#f1f6f1',
		'--wb-radius': '0px',
		'--wb-success': '#0b6b2e',
		'--wb-success-bg': '#d3ecd9',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#f3e3bd',
		'--wb-danger': '#a8281b',
		'--wb-danger-bg': '#f6dad5',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0b5a63',
		'--wb-info-bg': '#d1e9ea',
		'--wb-neutral-bg': '#d3ded4',
		'--wb-focus': '#0b6b2e'
	},
	dark: {
		'--wb-font-heading': "'Share Tech Mono', ui-monospace, 'Courier New', monospace",
		'--wb-font-body': "'IBM Plex Sans', system-ui, sans-serif",
		'--wb-bg': '#030a05',
		'--wb-surface': '#07140b',
		'--wb-border': '#14502a',
		'--wb-border-strong': '#2f9b57',
		'--wb-text': '#c8f7d2',
		'--wb-muted': '#7fc592',
		'--wb-accent': '#4dff88',
		'--wb-accent-text': '#021008',
		'--wb-radius': '0px',
		'--wb-success': '#4dff88',
		'--wb-success-bg': '#0b2a16',
		'--wb-warning': '#ffc547',
		'--wb-warning-bg': '#2b2207',
		'--wb-danger': '#ff7a6b',
		'--wb-danger-bg': '#2e100c',
		'--wb-danger-text': '#1a0402',
		'--wb-info': '#6fe0ee',
		'--wb-info-bg': '#08262b',
		'--wb-neutral-bg': '#0e2216',
		'--wb-focus': '#4dff88'
	},
	css: `
{
	background-image: repeating-linear-gradient(0deg, rgba(0, 0, 0, 0.14) 0, rgba(0, 0, 0, 0.14) 1px, transparent 1px, transparent 3px);
}
h1,
h2,
h3 {
	text-transform: uppercase;
	letter-spacing: 0.14em;
	font-weight: 400;
}
h1::before,
h2::before {
	content: '> ';
	color: var(--wb-accent);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 1px solid var(--wb-border-strong);
	box-shadow: inset 0 0 0 3px var(--wb-surface), inset 0 0 0 4px var(--wb-border);
}
.button {
	font-family: var(--wb-font-heading);
	font-weight: 400;
	text-transform: uppercase;
	letter-spacing: 0.1em;
}
`
};

export default style;

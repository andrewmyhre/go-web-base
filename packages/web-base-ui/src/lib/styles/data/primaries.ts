import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'primaries',
	name: 'Primaries',
	description: 'Early-modernist poster geometry: flat primary colours, hard black bars and a geometric sans in capitals.',
	fonts: ['Jost:wght@500;700', 'Public Sans:wght@400;600'],
	light: {
		'--wb-font-heading': "'Jost', 'Futura', 'Century Gothic', sans-serif",
		'--wb-font-body': "'Public Sans', system-ui, sans-serif",
		'--wb-bg': '#f3ecd9',
		'--wb-surface': '#fffaf0',
		'--wb-border': '#d3c9ae',
		'--wb-border-strong': '#111111',
		'--wb-text': '#141414',
		'--wb-muted': '#4d4637',
		'--wb-accent': '#b8231a',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '0px',
		'--wb-success': '#14602f',
		'--wb-success-bg': '#dcefd9',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#fbe7a6',
		'--wb-danger': '#a8160c',
		'--wb-danger-bg': '#f9d9d3',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#12449c',
		'--wb-info-bg': '#d9e3f7',
		'--wb-neutral-bg': '#e6dfca',
		'--wb-focus': '#12449c',
		'--wb-overlay': 'rgba(20, 20, 20, 0.6)'
	},
	dark: {
		'--wb-font-heading': "'Jost', 'Futura', 'Century Gothic', sans-serif",
		'--wb-font-body': "'Public Sans', system-ui, sans-serif",
		'--wb-radius': '0px',
		'--wb-bg': '#121212',
		'--wb-surface': '#1c1c1c',
		'--wb-border': '#3a3a3a',
		'--wb-border-strong': '#9a9a9a',
		'--wb-text': '#f2ecdc',
		'--wb-muted': '#b7b09e',
		'--wb-accent': '#ffcf2e',
		'--wb-accent-text': '#141414',
		'--wb-success': '#7fdc9a',
		'--wb-success-bg': '#14301d',
		'--wb-warning': '#ffcf2e',
		'--wb-warning-bg': '#33290a',
		'--wb-danger': '#ff8f84',
		'--wb-danger-bg': '#3a1613',
		'--wb-danger-text': '#141414',
		'--wb-info': '#8fb3ff',
		'--wb-info-bg': '#14213f',
		'--wb-neutral-bg': '#2a2a2a',
		'--wb-focus': '#ffcf2e',
		'--wb-overlay': 'rgba(0, 0, 0, 0.7)'
	},
	css: `
h1,
h2,
h3 {
	text-transform: uppercase;
	letter-spacing: 0.06em;
	font-weight: 700;
}
h1 {
	display: inline-block;
	padding: 0.15em 0.5em 0.1em 0;
	border-bottom: 8px solid var(--wb-accent);
}
h2 {
	border-left: 10px solid var(--wb-accent);
	padding-left: 0.5em;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 2px solid var(--wb-border-strong);
	box-shadow: 6px 6px 0 var(--wb-border-strong);
}
.button {
	text-transform: uppercase;
	letter-spacing: 0.08em;
	font-weight: 600;
}
`
};

export default style;

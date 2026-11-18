import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'journal',
	name: 'Journal Article',
	description: 'A scientific journal: condensed sans section heads, serif running text, hairline rules and one reference blue.',
	fonts: ['IBM Plex Sans Condensed:wght@600', 'IBM Plex Serif:wght@400;600'],
	light: {
		'--wb-font-heading': "'IBM Plex Sans Condensed', 'Arial Narrow', 'Helvetica Neue', sans-serif",
		'--wb-font-body': "'IBM Plex Serif', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#ffffff',
		'--wb-surface': '#f6f7f9',
		'--wb-border': '#d3d7de',
		'--wb-border-strong': '#6b7280',
		'--wb-text': '#14181f',
		'--wb-muted': '#4b5563',
		'--wb-accent': '#0b5cad',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '2px',
		'--wb-success': '#14673a',
		'--wb-success-bg': '#e0f2e7',
		'--wb-warning': '#85470a',
		'--wb-warning-bg': '#fbeacb',
		'--wb-danger': '#b3201a',
		'--wb-danger-bg': '#fbe3e0',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0b5cad',
		'--wb-info-bg': '#e0ecf8',
		'--wb-neutral-bg': '#e9ebef',
		'--wb-focus': '#0b5cad'
	},
	dark: {
		'--wb-font-heading': "'IBM Plex Sans Condensed', 'Arial Narrow', 'Helvetica Neue', sans-serif",
		'--wb-font-body': "'IBM Plex Serif', Georgia, 'Times New Roman', serif",
		'--wb-radius': '2px',
		'--wb-bg': '#0f1318',
		'--wb-surface': '#171c23',
		'--wb-border': '#2b323c',
		'--wb-border-strong': '#7b8696',
		'--wb-text': '#e8ecf1',
		'--wb-muted': '#a3adbb',
		'--wb-accent': '#6db0f5',
		'--wb-accent-text': '#0f1318',
		'--wb-success': '#74dd9f',
		'--wb-success-bg': '#12281c',
		'--wb-warning': '#f3c15a',
		'--wb-warning-bg': '#2b2410',
		'--wb-danger': '#ff8f86',
		'--wb-danger-bg': '#2e1816',
		'--wb-danger-text': '#0f1318',
		'--wb-info': '#6db0f5',
		'--wb-info-bg': '#12253a',
		'--wb-neutral-bg': '#202730',
		'--wb-focus': '#6db0f5'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 600;
	letter-spacing: 0.01em;
	line-height: 1.15;
}
h2,
h3 {
	font-size: 0.95rem;
	text-transform: uppercase;
	letter-spacing: 0.12em;
	color: var(--wb-accent);
	border-top: 2px solid var(--wb-accent);
	padding-top: 0.3rem;
}
h1 {
	font-size: 2rem;
	font-weight: 600;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border-width: 0 0 1px;
	border-radius: 0;
	background: transparent;
	padding-left: 0;
	padding-right: 0;
}
.help {
	font-size: 0.9em;
	font-style: italic;
}
.button {
	font-family: var(--wb-font-heading);
	letter-spacing: 0.04em;
}
table {
	font-variant-numeric: tabular-nums;
}
`
};

export default style;

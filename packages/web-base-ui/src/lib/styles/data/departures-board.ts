import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'departures-board',
	name: 'Departures Board',
	description: 'Airport and rail departure boards: amber on black, tabular mono capitals, one row per item.',
	fonts: ['Share Tech Mono', 'IBM Plex Sans:wght@400;600'],
	light: {
		'--wb-font-heading': "'Share Tech Mono', ui-monospace, Menlo, monospace",
		'--wb-font-body': "'IBM Plex Sans', system-ui, sans-serif",
		'--wb-bg': '#e9e6dc',
		'--wb-surface': '#fbfaf5',
		'--wb-text': '#14120d',
		'--wb-muted': '#55503f',
		'--wb-accent': '#8a4b00',
		'--wb-accent-text': '#ffffff',
		'--wb-border': '#cfc9b6',
		'--wb-border-strong': '#7d7762',
		'--wb-radius': '2px',
		'--wb-focus': '#8a4b00',
		'--wb-success': '#14602f',
		'--wb-success-bg': '#e0f2e6',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#fbecc8',
		'--wb-danger': '#a62319',
		'--wb-danger-bg': '#fbe4e1',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0b5a99',
		'--wb-info-bg': '#e0eef9',
		'--wb-neutral-bg': '#e4e0d2'
	},
	dark: {
		'--wb-font-heading': "'Share Tech Mono', ui-monospace, Menlo, monospace",
		'--wb-font-body': "'IBM Plex Sans', system-ui, sans-serif",
		'--wb-bg': '#0b0c0e',
		'--wb-surface': '#15171b',
		'--wb-text': '#f2e8c9',
		'--wb-muted': '#a39c86',
		'--wb-accent': '#ffb000',
		'--wb-accent-text': '#0b0c0e',
		'--wb-border': '#2c2f36',
		'--wb-border-strong': '#7a7d86',
		'--wb-radius': '2px',
		'--wb-focus': '#ffb000',
		'--wb-success': '#5fe08a',
		'--wb-success-bg': '#10261a',
		'--wb-warning': '#ffb000',
		'--wb-warning-bg': '#2e2306',
		'--wb-danger': '#ff7a6e',
		'--wb-danger-bg': '#2f1513',
		'--wb-danger-text': '#0b0c0e',
		'--wb-info': '#6fc3ff',
		'--wb-info-bg': '#112332',
		'--wb-neutral-bg': '#20232a'
	},
	css: `
h1,
h2,
h3 {
	text-transform: uppercase;
	letter-spacing: 0.14em;
	font-weight: 400;
}
h1 {
	display: inline-block;
	padding: 0.3em 0.6em;
	background: var(--wb-accent);
	color: var(--wb-accent-text);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border-width: 1px 1px 1px 6px;
	border-left-color: var(--wb-accent);
	background-image: repeating-linear-gradient(to bottom, transparent 0 2.4rem, var(--wb-border) 2.4rem calc(2.4rem + 1px));
}
table {
	font-variant-numeric: tabular-nums;
}
th {
	font-family: var(--wb-font-heading);
	text-transform: uppercase;
	letter-spacing: 0.1em;
	font-weight: 400;
}
`
};

export default style;

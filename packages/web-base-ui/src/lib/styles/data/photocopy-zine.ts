import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'photocopy-zine',
	name: 'Photocopy Zine',
	description: 'A 1990s photocopied zine: condensed capitals, hard offset shadows, cut-and-paste borders and a highlighter accent.',
	fonts: ['Anton', 'Archivo:wght@400;700'],
	light: {
		'--wb-font-heading': "'Anton', 'Impact', 'Haettenschweiler', sans-serif",
		'--wb-font-body': "'Archivo', system-ui, 'Segoe UI', sans-serif",
		'--wb-bg': '#e9e5d6',
		'--wb-surface': '#fffdf5',
		'--wb-border': '#111111',
		'--wb-border-strong': '#111111',
		'--wb-text': '#111111',
		'--wb-muted': '#4a463c',
		'--wb-accent': '#b8005a',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '0px',
		'--wb-success': '#0b5d2a',
		'--wb-success-bg': '#c8f0d3',
		'--wb-warning': '#6b4200',
		'--wb-warning-bg': '#ffe36e',
		'--wb-danger': '#b00020',
		'--wb-danger-bg': '#ffd0d6',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0a3fa8',
		'--wb-info-bg': '#cfe0ff',
		'--wb-neutral-bg': '#dcd8c8',
		'--wb-focus': '#0a3fa8'
	},
	dark: {
		'--wb-font-heading': "'Anton', 'Impact', 'Haettenschweiler', sans-serif",
		'--wb-font-body': "'Archivo', system-ui, 'Segoe UI', sans-serif",
		'--wb-radius': '0px',
		'--wb-bg': '#0a0a0a',
		'--wb-surface': '#161616',
		'--wb-border': '#f2f2f2',
		'--wb-border-strong': '#f2f2f2',
		'--wb-text': '#f5f5f0',
		'--wb-muted': '#b8b8ae',
		'--wb-accent': '#e4ff2e',
		'--wb-accent-text': '#0a0a0a',
		'--wb-success': '#5df58f',
		'--wb-success-bg': '#0d2a17',
		'--wb-warning': '#ffd23f',
		'--wb-warning-bg': '#2e2503',
		'--wb-danger': '#ff7a8a',
		'--wb-danger-bg': '#33101a',
		'--wb-danger-text': '#0a0a0a',
		'--wb-info': '#7fd4ff',
		'--wb-info-bg': '#0b2433',
		'--wb-neutral-bg': '#262626',
		'--wb-focus': '#e4ff2e'
	},
	css: `
h1,
h2,
h3 {
	text-transform: uppercase;
	font-weight: 400;
	letter-spacing: 0.02em;
	line-height: 1;
}
h1 {
	font-size: 3rem;
	display: inline-block;
	background: var(--wb-accent);
	color: var(--wb-accent-text);
	padding: 0.1em 0.3em;
	transform: rotate(-1.5deg);
	box-shadow: 5px 5px 0 var(--wb-text);
}
h2 {
	border-left: 10px solid var(--wb-accent);
	padding-left: 0.4rem;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 2px solid var(--wb-border);
	box-shadow: 5px 5px 0 var(--wb-border);
}
.button {
	text-transform: uppercase;
	font-weight: 700;
	letter-spacing: 0.05em;
	border-width: 2px;
	box-shadow: 3px 3px 0 var(--wb-border);
}
.button:active {
	box-shadow: none;
	transform: translate(3px, 3px);
}
body {
	background-image: radial-gradient(color-mix(in srgb, var(--wb-text) 14%, transparent) 1px, transparent 1.2px);
	background-size: 5px 5px;
}
`
};

export default style;

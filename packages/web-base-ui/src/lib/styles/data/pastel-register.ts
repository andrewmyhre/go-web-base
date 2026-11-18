import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'pastel-register',
	name: 'Pastel Register',
	description: 'Symmetrical pastel title cards: flat pink and plum, a geometric Futura-style face and a serif for reading.',
	fonts: ['Josefin Sans:wght@600', 'Lora:wght@400;700'],
	light: {
		'--wb-font-heading': "'Josefin Sans', 'Century Gothic', 'Avenir Next', sans-serif",
		'--wb-font-body': "'Lora', Georgia, serif",
		'--wb-bg': '#f6d9de',
		'--wb-surface': '#fff6f1',
		'--wb-border': '#e2b9c3',
		'--wb-border-strong': '#8a4a63',
		'--wb-text': '#3a1c2b',
		'--wb-muted': '#6a4558',
		'--wb-accent': '#7b2b52',
		'--wb-accent-text': '#fff6f1',
		'--wb-radius': '3px',
		'--wb-success': '#1f5c3f',
		'--wb-success-bg': '#dff0e3',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#fbe6b8',
		'--wb-danger': '#a31d3a',
		'--wb-danger-bg': '#fbdadf',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#274a8f',
		'--wb-info-bg': '#dde6f7',
		'--wb-neutral-bg': '#f0dde0',
		'--wb-focus': '#7b2b52'
	},
	dark: {
		'--wb-font-heading': "'Josefin Sans', 'Century Gothic', 'Avenir Next', sans-serif",
		'--wb-font-body': "'Lora', Georgia, serif",
		'--wb-bg': '#2a1522',
		'--wb-surface': '#371c2d',
		'--wb-border': '#5a3349',
		'--wb-border-strong': '#a77489',
		'--wb-text': '#fbe9ec',
		'--wb-muted': '#d9b4c2',
		'--wb-accent': '#f2a1bb',
		'--wb-accent-text': '#2a1522',
		'--wb-radius': '3px',
		'--wb-success': '#8fd9ae',
		'--wb-success-bg': '#1d3a2b',
		'--wb-warning': '#f5c869',
		'--wb-warning-bg': '#44301a',
		'--wb-danger': '#ff9aa8',
		'--wb-danger-bg': '#4a1c28',
		'--wb-danger-text': '#2a1522',
		'--wb-info': '#a9c1f5',
		'--wb-info-bg': '#26324f',
		'--wb-neutral-bg': '#432638',
		'--wb-focus': '#f2a1bb'
	},
	css: `
h1,
h2,
h3 {
	text-transform: uppercase;
	letter-spacing: 0.14em;
	font-weight: 600;
}
h1 {
	text-align: center;
	padding-bottom: 0.5rem;
	border-bottom: 4px double var(--wb-accent);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	box-shadow: inset 0 0 0 4px var(--wb-surface), inset 0 0 0 5px var(--wb-border-strong);
	padding: 0.5rem;
}
.button {
	text-transform: uppercase;
	letter-spacing: 0.1em;
	font-family: var(--wb-font-heading);
}
`
};

export default style;

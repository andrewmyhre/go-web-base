import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'letterpress',
	name: 'Letterpress',
	description: 'Letterpress job printing: black and red ink on cream stock, double-rule borders and a fat didone display face.',
	fonts: ['Abril Fatface', 'Libre Caslon Text:ital,wght@0,400;0,700;1,400'],
	light: {
		'--wb-font-heading': "'Abril Fatface', 'Didot', 'Bodoni 72', Georgia, serif",
		'--wb-font-body': "'Libre Caslon Text', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#f2ead9',
		'--wb-surface': '#fbf6ea',
		'--wb-border': '#cdbf9f',
		'--wb-border-strong': '#6b5d44',
		'--wb-text': '#191611',
		'--wb-muted': '#524a3a',
		'--wb-accent': '#b32a14',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '2px',
		'--wb-success': '#2b5d2e',
		'--wb-success-bg': '#dfe9cf',
		'--wb-warning': '#7f4300',
		'--wb-warning-bg': '#f4dca6',
		'--wb-danger': '#a3200f',
		'--wb-danger-bg': '#f3d6c9',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#254a78',
		'--wb-info-bg': '#d9e2ec',
		'--wb-neutral-bg': '#e6dcc3',
		'--wb-focus': '#254a78',
		'--wb-overlay': 'rgba(25, 22, 17, 0.58)'
	},
	dark: {
		'--wb-font-heading': "'Abril Fatface', 'Didot', 'Bodoni 72', Georgia, serif",
		'--wb-font-body': "'Libre Caslon Text', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#16130e',
		'--wb-surface': '#1f1b14',
		'--wb-border': '#3a3326',
		'--wb-border-strong': '#9a8c70',
		'--wb-text': '#f0e8d6',
		'--wb-muted': '#bcb198',
		'--wb-accent': '#ff7a5c',
		'--wb-accent-text': '#16130e',
		'--wb-radius': '2px',
		'--wb-success': '#a3d58c',
		'--wb-success-bg': '#1d2916',
		'--wb-warning': '#f0b94d',
		'--wb-warning-bg': '#31260f',
		'--wb-danger': '#ff917a',
		'--wb-danger-bg': '#341a12',
		'--wb-danger-text': '#16130e',
		'--wb-info': '#8db8ee',
		'--wb-info-bg': '#16243a',
		'--wb-neutral-bg': '#2a251b',
		'--wb-focus': '#8db8ee',
		'--wb-overlay': 'rgba(0, 0, 0, 0.7)'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 400;
	letter-spacing: 0.005em;
}
h1 {
	font-size: 2.6rem;
	text-align: center;
	border-top: 4px double var(--wb-text);
	border-bottom: 1px solid var(--wb-text);
	padding: 0.4rem 0;
}
h2 {
	font-size: 1.5rem;
	border-bottom: 1px solid var(--wb-border-strong);
	padding-bottom: 0.15rem;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 4px double var(--wb-text);
	background-image: linear-gradient(var(--wb-surface), var(--wb-surface));
}
.button {
	font-variant: small-caps;
	letter-spacing: 0.08em;
	font-weight: 700;
}
a {
	text-decoration-thickness: 1px;
	text-underline-offset: 3px;
}
`
};

export default style;

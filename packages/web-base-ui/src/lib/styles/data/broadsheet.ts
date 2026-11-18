import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'broadsheet',
	name: 'Broadsheet',
	description: 'A newspaper front page: high-contrast masthead headings, double column rules, newsprint cream and a book serif.',
	fonts: ['Playfair Display:wght@800', 'Source Serif 4:opsz,wght@8..60,400;8..60,600'],
	light: {
		'--wb-font-heading': "'Playfair Display', 'Didot', 'Bodoni 72', Georgia, serif",
		'--wb-font-body': "'Source Serif 4', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#ece4d0',
		'--wb-surface': '#f7f1e1',
		'--wb-border': '#c9bd9f',
		'--wb-border-strong': '#6e6246',
		'--wb-text': '#1a1712',
		'--wb-muted': '#585043',
		'--wb-accent': '#8a1c1c',
		'--wb-accent-text': '#fbf6e8',
		'--wb-radius': '0px',
		'--wb-success': '#1f5130',
		'--wb-success-bg': '#dce9d3',
		'--wb-warning': '#7a4a0a',
		'--wb-warning-bg': '#f3e1b5',
		'--wb-danger': '#9b1c1c',
		'--wb-danger-bg': '#f2d6cf',
		'--wb-danger-text': '#fbf6e8',
		'--wb-info': '#1f3f6e',
		'--wb-info-bg': '#d9e0ea',
		'--wb-neutral-bg': '#ddd3bb',
		'--wb-focus': '#8a1c1c'
	},
	dark: {
		'--wb-font-heading': "'Playfair Display', 'Didot', 'Bodoni 72', Georgia, serif",
		'--wb-font-body': "'Source Serif 4', Georgia, 'Times New Roman', serif",
		'--wb-radius': '0px',
		'--wb-bg': '#14120e',
		'--wb-surface': '#1d1a14',
		'--wb-border': '#3a3427',
		'--wb-border-strong': '#8c8266',
		'--wb-text': '#ebe3cf',
		'--wb-muted': '#b2a78c',
		'--wb-accent': '#e0b44a',
		'--wb-accent-text': '#14120e',
		'--wb-success': '#8fd09f',
		'--wb-success-bg': '#18291c',
		'--wb-warning': '#f0c25c',
		'--wb-warning-bg': '#2d2410',
		'--wb-danger': '#f19a8c',
		'--wb-danger-bg': '#2f1915',
		'--wb-danger-text': '#14120e',
		'--wb-info': '#9db8e6',
		'--wb-info-bg': '#162033',
		'--wb-neutral-bg': '#27231a',
		'--wb-focus': '#e0b44a'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 800;
	line-height: 1.05;
	letter-spacing: -0.01em;
}
h1 {
	text-align: center;
	font-size: 2.4rem;
	padding: 0.4rem 0 0.5rem;
	border-top: 4px solid var(--wb-text);
	border-bottom: 1px solid var(--wb-text);
	box-shadow: 0 3px 0 -2px var(--wb-bg), 0 4px 0 -2px var(--wb-text);
}
h2,
h3 {
	border-bottom: 1px solid var(--wb-text);
	padding-bottom: 0.15rem;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border-width: 1px 0 3px;
	border-style: solid;
	border-color: var(--wb-text);
	background-image: linear-gradient(var(--wb-surface), var(--wb-surface));
}
.button {
	font-variant: small-caps;
	letter-spacing: 0.06em;
	font-weight: 600;
}
p {
	hyphens: auto;
	line-height: 1.6;
}
`
};

export default style;

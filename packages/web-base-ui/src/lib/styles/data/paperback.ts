import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'paperback',
	name: 'Paperback',
	description: '1930s-60s paperback covers: coloured horizontal bands, a tracked humanist sans and cream stock.',
	fonts: ['Josefin Sans:wght@600;700', 'Crimson Pro:wght@400;600'],
	light: {
		'--wb-font-heading': "'Josefin Sans', 'Gill Sans', 'Century Gothic', sans-serif",
		'--wb-font-body': "'Crimson Pro', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#f1e9d2',
		'--wb-surface': '#fbf6e6',
		'--wb-border': '#d8cca9',
		'--wb-border-strong': '#7a6e50',
		'--wb-text': '#241f17',
		'--wb-muted': '#5a5240',
		'--wb-accent': '#aa3507',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '2px',
		'--wb-success': '#1e5a3a',
		'--wb-success-bg': '#d9ead9',
		'--wb-warning': '#7c4a03',
		'--wb-warning-bg': '#f6e2b0',
		'--wb-danger': '#a32020',
		'--wb-danger-bg': '#f5d9d2',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#1c4f7a',
		'--wb-info-bg': '#d8e4ee',
		'--wb-neutral-bg': '#e5dcc0',
		'--wb-focus': '#aa3507'
	},
	dark: {
		'--wb-font-heading': "'Josefin Sans', 'Gill Sans', 'Century Gothic', sans-serif",
		'--wb-font-body': "'Crimson Pro', Georgia, 'Times New Roman', serif",
		'--wb-radius': '2px',
		'--wb-bg': '#10211a',
		'--wb-surface': '#172b22',
		'--wb-border': '#2c4538',
		'--wb-border-strong': '#7f9c8c',
		'--wb-text': '#efe9d6',
		'--wb-muted': '#b5c4b9',
		'--wb-accent': '#ff9f5a',
		'--wb-accent-text': '#10211a',
		'--wb-success': '#8fe0a9',
		'--wb-success-bg': '#12301f',
		'--wb-warning': '#f5c65e',
		'--wb-warning-bg': '#2f2a10',
		'--wb-danger': '#ff9a8d',
		'--wb-danger-bg': '#341a16',
		'--wb-danger-text': '#10211a',
		'--wb-info': '#9cc7ee',
		'--wb-info-bg': '#14293a',
		'--wb-neutral-bg': '#1f352a',
		'--wb-focus': '#ff9f5a'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.14em;
}
h1 {
	font-size: 1.7rem;
	text-align: center;
	padding: 0.6rem 0;
	border-top: 12px solid var(--wb-accent);
	border-bottom: 12px solid var(--wb-accent);
	background: var(--wb-surface);
}
h2 {
	font-size: 1.1rem;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 1px solid var(--wb-border);
	border-top: 8px solid var(--wb-accent);
	border-bottom: 8px solid var(--wb-accent);
	padding-top: 1rem;
	padding-bottom: 1rem;
}
.button {
	font-family: var(--wb-font-heading);
	text-transform: uppercase;
	letter-spacing: 0.12em;
	font-weight: 700;
}
p,
li {
	font-size: 1.08rem;
	line-height: 1.55;
}
`
};

export default style;

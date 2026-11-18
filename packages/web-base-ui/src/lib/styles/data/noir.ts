import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'noir',
	name: 'Noir Newsprint',
	description: 'Mid-century crime thriller posters: newsprint grey, deep black, high-contrast Didone capitals and one lipstick red.',
	fonts: ['Bodoni Moda:wght@700', 'Source Serif 4:wght@400;600'],
	light: {
		'--wb-font-heading': "'Bodoni Moda', 'Didot', 'Bodoni 72', Georgia, serif",
		'--wb-font-body': "'Source Serif 4', Georgia, serif",
		'--wb-bg': '#e4e0d6',
		'--wb-surface': '#f2efe7',
		'--wb-border': '#b9b3a4',
		'--wb-border-strong': '#6b665b',
		'--wb-text': '#141414',
		'--wb-muted': '#4d4a44',
		'--wb-accent': '#b3121b',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '0px',
		'--wb-success': '#1d5a35',
		'--wb-success-bg': '#dcebdf',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#f3e3bd',
		'--wb-danger': '#9d1018',
		'--wb-danger-bg': '#f4d9d7',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#1f3f73',
		'--wb-info-bg': '#dae1ee',
		'--wb-neutral-bg': '#d8d4c9',
		'--wb-focus': '#141414'
	},
	dark: {
		'--wb-font-heading': "'Bodoni Moda', 'Didot', 'Bodoni 72', Georgia, serif",
		'--wb-font-body': "'Source Serif 4', Georgia, serif",
		'--wb-bg': '#070707',
		'--wb-surface': '#131313',
		'--wb-border': '#2b2b2b',
		'--wb-border-strong': '#7d7a72',
		'--wb-text': '#ebe7dc',
		'--wb-muted': '#aaa598',
		'--wb-accent': '#ff5a55',
		'--wb-accent-text': '#0a0a0a',
		'--wb-radius': '0px',
		'--wb-success': '#7fd4a0',
		'--wb-success-bg': '#14241b',
		'--wb-warning': '#f2c55c',
		'--wb-warning-bg': '#2a2110',
		'--wb-danger': '#ff7d78',
		'--wb-danger-bg': '#2e1313',
		'--wb-danger-text': '#0a0a0a',
		'--wb-info': '#9fbcf0',
		'--wb-info-bg': '#151d2e',
		'--wb-neutral-bg': '#1d1d1d',
		'--wb-focus': '#ebe7dc'
	},
	css: `
{
	background-image: radial-gradient(ellipse at 15% -10%, rgba(255, 255, 255, 0.12), transparent 55%);
	background-attachment: fixed;
}
h1,
h2,
h3 {
	text-transform: uppercase;
	letter-spacing: 0.03em;
	font-weight: 700;
}
h1 {
	transform: rotate(-1.2deg);
	transform-origin: left center;
	border-bottom: 3px solid var(--wb-accent);
	padding-bottom: 0.25rem;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 2px solid var(--wb-text);
	box-shadow: 6px 6px 0 var(--wb-border-strong);
}
.button {
	border-radius: 0;
	text-transform: uppercase;
	letter-spacing: 0.06em;
}
`
};

export default style;

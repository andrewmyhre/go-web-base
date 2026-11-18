import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'cutout',
	name: 'Cutout',
	description: 'Cut-paper poster graphics: flat vermilion blocks on cream, towering condensed capitals and thick black outlines.',
	fonts: ['Anton:wght@400', 'Libre Franklin:wght@400;600'],
	light: {
		'--wb-font-heading': "'Anton', 'Impact', 'Haettenschweiler', sans-serif",
		'--wb-font-body': "'Libre Franklin', 'Helvetica Neue', Arial, sans-serif",
		'--wb-bg': '#f1e6cf',
		'--wb-surface': '#fbf4e4',
		'--wb-border': '#c9bb9c',
		'--wb-border-strong': '#111111',
		'--wb-text': '#111111',
		'--wb-muted': '#4a4337',
		'--wb-accent': '#b82c0b',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '0px',
		'--wb-success': '#1d5a35',
		'--wb-success-bg': '#dcebdb',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#f5e0a8',
		'--wb-danger': '#a82a10',
		'--wb-danger-bg': '#f7d6cb',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#1b3d6b',
		'--wb-info-bg': '#dbe3ec',
		'--wb-neutral-bg': '#e6dac0',
		'--wb-focus': '#111111'
	},
	dark: {
		'--wb-font-heading': "'Anton', 'Impact', 'Haettenschweiler', sans-serif",
		'--wb-font-body': "'Libre Franklin', 'Helvetica Neue', Arial, sans-serif",
		'--wb-bg': '#111111',
		'--wb-surface': '#1c1a17',
		'--wb-border': '#3a352c',
		'--wb-border-strong': '#f1e6cf',
		'--wb-text': '#f1e6cf',
		'--wb-muted': '#b9ad94',
		'--wb-accent': '#ff6a3d',
		'--wb-accent-text': '#111111',
		'--wb-radius': '0px',
		'--wb-success': '#86d4a1',
		'--wb-success-bg': '#14251a',
		'--wb-warning': '#f2c55c',
		'--wb-warning-bg': '#2c2410',
		'--wb-danger': '#ff8a6b',
		'--wb-danger-bg': '#2f1610',
		'--wb-danger-text': '#111111',
		'--wb-info': '#9fbcf0',
		'--wb-info-bg': '#161e2c',
		'--wb-neutral-bg': '#26231e',
		'--wb-focus': '#f1e6cf'
	},
	css: `
h1,
h2,
h3 {
	text-transform: uppercase;
	letter-spacing: 0.02em;
	font-weight: 400;
	line-height: 1.05;
}
h1 {
	font-size: clamp(2.25rem, 7vw, 3.5rem);
	padding-left: 0.75rem;
	border-left: 14px solid var(--wb-accent);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 3px solid var(--wb-border-strong);
}
.button {
	font-family: var(--wb-font-heading);
	font-weight: 400;
	text-transform: uppercase;
	letter-spacing: 0.08em;
	border-width: 2px;
}
`
};

export default style;

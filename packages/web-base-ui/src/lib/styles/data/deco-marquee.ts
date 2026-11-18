import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'deco-marquee',
	name: 'Deco Marquee',
	description: 'Art Deco (1925-1935) marquee lettering: stepped corners, thin gold double rules, gilt on black.',
	fonts: ['Limelight', 'Crimson Pro:wght@400;600'],
	light: {
		'--wb-font-heading': "'Limelight', 'Didot', 'Bodoni MT', serif",
		'--wb-font-body': "'Crimson Pro', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#efe6d2',
		'--wb-surface': '#fbf6e9',
		'--wb-border': '#cdbb8c',
		'--wb-border-strong': '#7a5f1a',
		'--wb-text': '#10231f',
		'--wb-muted': '#4a4a3a',
		'--wb-accent': '#0f4a40',
		'--wb-accent-text': '#f7ecc4',
		'--wb-radius': '0px',
		'--wb-success': '#1b5e32',
		'--wb-success-bg': '#dcebd3',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#f6e3b0',
		'--wb-danger': '#8e1b1b',
		'--wb-danger-bg': '#f2d6cf',
		'--wb-danger-text': '#fff4e6',
		'--wb-info': '#0f4a40',
		'--wb-info-bg': '#d5e6df',
		'--wb-neutral-bg': '#e8dfc6',
		'--wb-focus': '#0f4a40',
		'--wb-overlay': 'rgba(16, 35, 31, 0.6)'
	},
	dark: {
		'--wb-font-heading': "'Limelight', 'Didot', 'Bodoni MT', serif",
		'--wb-font-body': "'Crimson Pro', Georgia, 'Times New Roman', serif",
		'--wb-radius': '0px',
		'--wb-bg': '#0a0d12',
		'--wb-surface': '#121821',
		'--wb-border': '#3d3418',
		'--wb-border-strong': '#a58b3c',
		'--wb-text': '#f1e8cf',
		'--wb-muted': '#bdb394',
		'--wb-accent': '#d9b44a',
		'--wb-accent-text': '#0a0d12',
		'--wb-success': '#8fd9a6',
		'--wb-success-bg': '#12291b',
		'--wb-warning': '#f0c35a',
		'--wb-warning-bg': '#2d2410',
		'--wb-danger': '#ff9a90',
		'--wb-danger-bg': '#341614',
		'--wb-danger-text': '#0a0d12',
		'--wb-info': '#d9b44a',
		'--wb-info-bg': '#2a2310',
		'--wb-neutral-bg': '#1c2330',
		'--wb-focus': '#d9b44a',
		'--wb-overlay': 'rgba(0, 0, 0, 0.72)'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 400;
	letter-spacing: 0.1em;
	text-transform: uppercase;
}
h1 {
	text-align: center;
	padding-bottom: 0.4em;
	border-bottom: 3px double var(--wb-border-strong);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 1px solid var(--wb-border-strong);
	outline: 1px solid var(--wb-border);
	outline-offset: 4px;
	clip-path: polygon(14px 0, calc(100% - 14px) 0, 100% 14px, 100% calc(100% - 14px), calc(100% - 14px) 100%, 14px 100%, 0 calc(100% - 14px), 0 14px);
}
.button {
	letter-spacing: 0.12em;
	text-transform: uppercase;
	font-family: var(--wb-font-heading);
	clip-path: polygon(8px 0, calc(100% - 8px) 0, 100% 8px, 100% calc(100% - 8px), calc(100% - 8px) 100%, 8px 100%, 0 calc(100% - 8px), 0 8px);
}
body,
{
	font-size: 1.0625rem;
}
`
};

export default style;

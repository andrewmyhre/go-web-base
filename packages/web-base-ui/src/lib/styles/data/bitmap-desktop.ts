import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'bitmap-desktop',
	name: 'Bitmap Desktop',
	description: '1980s black-and-white bitmap desktops: dithered backdrop, hard one-pixel shadows and pixel headings.',
	fonts: ['Pixelify Sans:wght@500;700', 'Work Sans:wght@400;600'],
	light: {
		'--wb-font-heading': "'Pixelify Sans', ui-monospace, monospace",
		'--wb-font-body': "'Work Sans', system-ui, sans-serif",
		'--wb-bg': '#d9d9d9',
		'--wb-surface': '#ffffff',
		'--wb-text': '#000000',
		'--wb-muted': '#444444',
		'--wb-accent': '#000000',
		'--wb-accent-text': '#ffffff',
		'--wb-border': '#8c8c8c',
		'--wb-border-strong': '#000000',
		'--wb-radius': '0px',
		'--wb-focus': '#0000cc',
		'--wb-success': '#0b5d1e',
		'--wb-success-bg': '#e3f1e6',
		'--wb-warning': '#6b4200',
		'--wb-warning-bg': '#f5ead0',
		'--wb-danger': '#a00000',
		'--wb-danger-bg': '#f6e0e0',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0000cc',
		'--wb-info-bg': '#e4e4f7',
		'--wb-neutral-bg': '#ececec'
	},
	dark: {
		'--wb-font-heading': "'Pixelify Sans', ui-monospace, monospace",
		'--wb-font-body': "'Work Sans', system-ui, sans-serif",
		'--wb-bg': '#1a1a1a',
		'--wb-surface': '#000000',
		'--wb-text': '#ffffff',
		'--wb-muted': '#bdbdbd',
		'--wb-accent': '#ffffff',
		'--wb-accent-text': '#000000',
		'--wb-border': '#5a5a5a',
		'--wb-border-strong': '#ffffff',
		'--wb-radius': '0px',
		'--wb-focus': '#ffff66',
		'--wb-success': '#7dff9a',
		'--wb-success-bg': '#0a2a12',
		'--wb-warning': '#ffd966',
		'--wb-warning-bg': '#2e2408',
		'--wb-danger': '#ff8a8a',
		'--wb-danger-bg': '#330c0c',
		'--wb-danger-text': '#000000',
		'--wb-info': '#9db4ff',
		'--wb-info-bg': '#10163a',
		'--wb-neutral-bg': '#262626'
	},
	css: `
{
	background-image: conic-gradient(var(--wb-border) 25%, transparent 0 50%, var(--wb-border) 0 75%, transparent 0);
	background-size: 4px 4px;
	background-color: var(--wb-bg);
}
h1,
h2,
h3 {
	font-weight: 700;
	letter-spacing: 0.02em;
}
h1 {
	display: inline-block;
	padding: 0.2em 0.8em;
	background: var(--wb-surface);
	border: 2px solid var(--wb-border-strong);
	box-shadow: 3px 3px 0 var(--wb-border-strong);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 2px solid var(--wb-border-strong);
	box-shadow: 3px 3px 0 var(--wb-border-strong);
}
.button,
button {
	border-width: 2px;
	border-radius: 8px;
	box-shadow: 2px 2px 0 var(--wb-border-strong);
}
`
};

export default style;

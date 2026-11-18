import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'confetti',
	name: 'Confetti',
	description: '1980s Italian postmodern furniture: confetti patterns, chunky outlines, offset shadows and clashing pastels.',
	fonts: ['Lilita One', 'DM Sans:wght@400;600'],
	light: {
		'--wb-font-heading': "'Lilita One', 'Arial Rounded MT Bold', sans-serif",
		'--wb-font-body': "'DM Sans', system-ui, sans-serif",
		'--wb-bg': '#ffe9f1',
		'--wb-surface': '#ffffff',
		'--wb-border': '#d9b8c8',
		'--wb-border-strong': '#1a1033',
		'--wb-text': '#1a1033',
		'--wb-muted': '#4f4568',
		'--wb-accent': '#5b2bd9',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '18px',
		'--wb-success': '#0b6b52',
		'--wb-success-bg': '#c9f2e3',
		'--wb-warning': '#7d4600',
		'--wb-warning-bg': '#ffe58a',
		'--wb-danger': '#b0123c',
		'--wb-danger-bg': '#ffd0dd',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#1c4fc4',
		'--wb-info-bg': '#cfe2ff',
		'--wb-neutral-bg': '#f0dfe8',
		'--wb-focus': '#1c4fc4',
		'--wb-overlay': 'rgba(26, 16, 51, 0.6)'
	},
	dark: {
		'--wb-font-heading': "'Lilita One', 'Arial Rounded MT Bold', sans-serif",
		'--wb-font-body': "'DM Sans', system-ui, sans-serif",
		'--wb-radius': '18px',
		'--wb-bg': '#17102e',
		'--wb-surface': '#221846',
		'--wb-border': '#45387a',
		'--wb-border-strong': '#a99be0',
		'--wb-text': '#fff4fb',
		'--wb-muted': '#c6bbe6',
		'--wb-accent': '#ff7ac0',
		'--wb-accent-text': '#17102e',
		'--wb-success': '#6ff0c2',
		'--wb-success-bg': '#0e3a2e',
		'--wb-warning': '#ffe066',
		'--wb-warning-bg': '#3d3208',
		'--wb-danger': '#ff8fa9',
		'--wb-danger-bg': '#45122a',
		'--wb-danger-text': '#17102e',
		'--wb-info': '#8fd0ff',
		'--wb-info-bg': '#14284f',
		'--wb-neutral-bg': '#2d2358',
		'--wb-focus': '#ffe066',
		'--wb-overlay': 'rgba(10, 5, 25, 0.75)'
	},
	css: `
{
	background-color: var(--wb-bg);
	background-image:
		radial-gradient(circle at 12% 18%, #2ec4b6 0 7px, transparent 8px),
		radial-gradient(circle at 78% 62%, #ffd23f 0 9px, transparent 10px),
		linear-gradient(135deg, transparent 0 46%, #ff5ca8 46% 54%, transparent 54%),
		repeating-linear-gradient(45deg, transparent 0 14px, rgba(91, 43, 217, 0.1) 14px 16px);
	background-size: 220px 220px, 300px 300px, 160px 160px, auto;
}
h1,
h2,
h3 {
	font-weight: 400;
	letter-spacing: 0.01em;
}
h1 {
	display: inline-block;
	background: var(--wb-surface);
	border: 3px solid var(--wb-border-strong);
	border-radius: 999px;
	padding: 0.1em 0.7em;
	box-shadow: 5px 5px 0 var(--wb-accent);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 3px solid var(--wb-border-strong);
	box-shadow: 7px 7px 0 #2ec4b6;
}
.button {
	border-width: 3px;
	font-weight: 600;
	box-shadow: 4px 4px 0 var(--wb-border-strong);
}
`
};

export default style;

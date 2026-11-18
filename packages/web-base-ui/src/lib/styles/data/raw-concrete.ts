import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'raw-concrete',
	name: 'Raw Concrete',
	description: 'Brutalist web design and 1990s manual pages: monospace everywhere, heavy black borders, underlined links and one highlighter yellow.',
	fonts: ['IBM Plex Mono:wght@400;700', 'IBM Plex Sans:wght@400;600'],
	light: {
		'--wb-font-heading': "'IBM Plex Mono', 'Courier New', monospace",
		'--wb-font-body': "'IBM Plex Sans', Arial, sans-serif",
		'--wb-bg': '#e4e4e0',
		'--wb-surface': '#ffffff',
		'--wb-border': '#9a9a94',
		'--wb-border-strong': '#000000',
		'--wb-text': '#000000',
		'--wb-muted': '#3d3d3a',
		'--wb-accent': '#0000d6',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '0px',
		'--wb-success': '#00600a',
		'--wb-success-bg': '#d6f5d6',
		'--wb-warning': '#6b3d00',
		'--wb-warning-bg': '#fff04a',
		'--wb-danger': '#b00000',
		'--wb-danger-bg': '#ffd9d9',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0000d6',
		'--wb-info-bg': '#dcdcff',
		'--wb-neutral-bg': '#d0d0ca',
		'--wb-focus': '#d60000',
		'--wb-overlay': 'rgba(0, 0, 0, 0.7)'
	},
	dark: {
		'--wb-font-heading': "'IBM Plex Mono', 'Courier New', monospace",
		'--wb-font-body': "'IBM Plex Sans', Arial, sans-serif",
		'--wb-radius': '0px',
		'--wb-bg': '#000000',
		'--wb-surface': '#0d0d0d',
		'--wb-border': '#4a4a4a',
		'--wb-border-strong': '#d8d8d8',
		'--wb-text': '#f2f2f2',
		'--wb-muted': '#b5b5b5',
		'--wb-accent': '#fff04a',
		'--wb-accent-text': '#000000',
		'--wb-success': '#5dff7a',
		'--wb-success-bg': '#052a0c',
		'--wb-warning': '#fff04a',
		'--wb-warning-bg': '#2e2a00',
		'--wb-danger': '#ff7a7a',
		'--wb-danger-bg': '#330808',
		'--wb-danger-text': '#000000',
		'--wb-info': '#9aa8ff',
		'--wb-info-bg': '#10143d',
		'--wb-neutral-bg': '#202020',
		'--wb-focus': '#ff4d4d',
		'--wb-overlay': 'rgba(0, 0, 0, 0.8)'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 700;
	letter-spacing: -0.02em;
	text-transform: lowercase;
}
h1::before { content: '# '; }
h2::before { content: '## '; }
h3::before { content: '### '; }
h1 {
	display: inline-block;
	background: var(--wb-text);
	color: var(--wb-bg);
	padding: 0.1em 0.4em;
}
a { text-decoration: underline; text-underline-offset: 3px; }
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 3px solid var(--wb-border-strong);
	background-image: linear-gradient(var(--wb-border) 1px, transparent 1px), linear-gradient(90deg, var(--wb-border) 1px, transparent 1px);
	background-size: 24px 24px;
	background-color: var(--wb-surface);
	background-blend-mode: soft-light;
}
.button {
	border-width: 3px;
	font-family: var(--wb-font-heading);
	font-weight: 700;
	text-transform: lowercase;
}
`
};

export default style;

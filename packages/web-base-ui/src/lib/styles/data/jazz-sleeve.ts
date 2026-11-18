import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'jazz-sleeve',
	name: 'Jazz Sleeve',
	description: '1960s jazz record sleeves: tall condensed capitals over a heavy ink bar on cream stock with a single blue.',
	fonts: ['Anton', 'Source Sans 3:wght@400;600'],
	light: {
		'--wb-font-heading': "'Anton', 'Impact', 'Arial Narrow', sans-serif",
		'--wb-font-body': "'Source Sans 3', system-ui, sans-serif",
		'--wb-bg': '#efe6d0',
		'--wb-surface': '#faf5e8',
		'--wb-border': '#cfc3a4',
		'--wb-border-strong': '#6e6246',
		'--wb-text': '#14110c',
		'--wb-muted': '#524a38',
		'--wb-accent': '#0a4a9a',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '0px',
		'--wb-success': '#1b5e32',
		'--wb-success-bg': '#dcebd5',
		'--wb-warning': '#8a3d00',
		'--wb-warning-bg': '#f6dfb4',
		'--wb-danger': '#a8211a',
		'--wb-danger-bg': '#f5d9d2',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0a4a9a',
		'--wb-info-bg': '#d8e3f2',
		'--wb-neutral-bg': '#e2d9c0',
		'--wb-focus': '#0a4a9a',
		'--wb-overlay': 'rgba(20, 17, 12, 0.6)'
	},
	dark: {
		'--wb-font-heading': "'Anton', 'Impact', 'Arial Narrow', sans-serif",
		'--wb-font-body': "'Source Sans 3', system-ui, sans-serif",
		'--wb-bg': '#0b0d12',
		'--wb-surface': '#141821',
		'--wb-border': '#2a3040',
		'--wb-border-strong': '#7b86a0',
		'--wb-text': '#f1ead8',
		'--wb-muted': '#b3ab95',
		'--wb-accent': '#5d9bff',
		'--wb-accent-text': '#0b0d12',
		'--wb-radius': '0px',
		'--wb-success': '#7bdc9a',
		'--wb-success-bg': '#13281c',
		'--wb-warning': '#f2b84b',
		'--wb-warning-bg': '#2c2210',
		'--wb-danger': '#ff8f85',
		'--wb-danger-bg': '#2f1715',
		'--wb-danger-text': '#0b0d12',
		'--wb-info': '#5d9bff',
		'--wb-info-bg': '#121f38',
		'--wb-neutral-bg': '#1d2330',
		'--wb-focus': '#5d9bff',
		'--wb-overlay': 'rgba(0, 0, 0, 0.7)'
	},
	css: `
h1,
h2,
h3 {
	text-transform: uppercase;
	letter-spacing: 0.02em;
	font-weight: 400;
	line-height: 1.02;
}
h1 {
	font-size: 2.6rem;
	border-bottom: 10px solid var(--wb-text);
	padding-bottom: 0.2rem;
}
h2 {
	border-left: 12px solid var(--wb-accent);
	padding-left: 0.5rem;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 2px solid var(--wb-text);
	box-shadow: 6px 6px 0 var(--wb-accent);
}
.button {
	text-transform: uppercase;
	letter-spacing: 0.06em;
	font-weight: 600;
}
`
};

export default style;

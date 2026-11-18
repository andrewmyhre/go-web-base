import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'mid-century-house',
	name: 'Mid-Century House',
	description: 'Mid-century modern houses: warm timber tones, one saturated accent, low wide geometric headings and a book serif.',
	fonts: ['Josefin Sans:wght@600;700', 'Lora:wght@400;600'],
	light: {
		'--wb-font-heading': "'Josefin Sans', 'Century Gothic', system-ui, sans-serif",
		'--wb-font-body': "'Lora', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#f3ece0',
		'--wb-surface': '#fdfaf3',
		'--wb-border': '#ddd0b8',
		'--wb-border-strong': '#7a6444',
		'--wb-text': '#2a2118',
		'--wb-muted': '#5e5240',
		'--wb-accent': '#0b6b6b',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '22px',
		'--wb-success': '#2c6a2c',
		'--wb-success-bg': '#e0eed3',
		'--wb-warning': '#7d4a00',
		'--wb-warning-bg': '#f7e2a8',
		'--wb-danger': '#a8321c',
		'--wb-danger-bg': '#f6dccf',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#0b6b6b',
		'--wb-info-bg': '#d4ebe8',
		'--wb-neutral-bg': '#e8dfcc',
		'--wb-focus': '#0b6b6b',
		'--wb-overlay': 'rgba(42, 33, 24, 0.55)'
	},
	dark: {
		'--wb-font-heading': "'Josefin Sans', 'Century Gothic', system-ui, sans-serif",
		'--wb-font-body': "'Lora', Georgia, 'Times New Roman', serif",
		'--wb-bg': '#1a1510',
		'--wb-surface': '#241d16',
		'--wb-border': '#3a2f23',
		'--wb-border-strong': '#9a8466',
		'--wb-text': '#f1e7d4',
		'--wb-muted': '#bdae94',
		'--wb-accent': '#e8b43c',
		'--wb-accent-text': '#1a1510',
		'--wb-radius': '22px',
		'--wb-success': '#9fd68a',
		'--wb-success-bg': '#1f2c18',
		'--wb-warning': '#f0b84a',
		'--wb-warning-bg': '#33260f',
		'--wb-danger': '#ff9a7e',
		'--wb-danger-bg': '#341b13',
		'--wb-danger-text': '#1a1510',
		'--wb-info': '#6fd0c8',
		'--wb-info-bg': '#12302d',
		'--wb-neutral-bg': '#2d241b',
		'--wb-focus': '#e8b43c',
		'--wb-overlay': 'rgba(0, 0, 0, 0.65)'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 700;
	letter-spacing: 0.14em;
	text-transform: lowercase;
}
h1 {
	font-size: 2.3rem;
	padding-bottom: 0.5rem;
	background: linear-gradient(var(--wb-accent), var(--wb-accent)) left bottom / 4.5rem 6px no-repeat,
		linear-gradient(var(--wb-border-strong), var(--wb-border-strong)) left bottom / 100% 1px no-repeat;
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: none;
	border-radius: 22px 22px 22px 4px;
	box-shadow: 0 1px 0 var(--wb-border), 0 8px 18px -12px rgba(60, 40, 10, 0.45);
}
.button {
	border-radius: 999px;
	font-family: var(--wb-font-heading);
	font-weight: 700;
	letter-spacing: 0.08em;
}
`
};

export default style;

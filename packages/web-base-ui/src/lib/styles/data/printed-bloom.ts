import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'printed-bloom',
	name: 'Printed Bloom',
	description: '1960s Scandinavian printed textiles: one fearless colour, a faint dot print, soft serif headings and pebble-shaped cards.',
	fonts: ['Young Serif', 'Figtree:wght@400;600'],
	light: {
		'--wb-font-heading': "'Young Serif', Georgia, serif",
		'--wb-font-body': "'Figtree', system-ui, sans-serif",
		'--wb-bg': '#fbf7f1',
		'--wb-surface': '#ffffff',
		'--wb-border': '#ecdfd2',
		'--wb-border-strong': '#8a6f60',
		'--wb-text': '#241a1c',
		'--wb-muted': '#5f4a4c',
		'--wb-accent': '#c4161c',
		'--wb-accent-text': '#ffffff',
		'--wb-radius': '28px',
		'--wb-success': '#25683a',
		'--wb-success-bg': '#e2f1e4',
		'--wb-warning': '#8a4300',
		'--wb-warning-bg': '#fbe6c4',
		'--wb-danger': '#b3141a',
		'--wb-danger-bg': '#fbdddc',
		'--wb-danger-text': '#ffffff',
		'--wb-info': '#1f4f94',
		'--wb-info-bg': '#dde8f7',
		'--wb-neutral-bg': '#f0e7dc',
		'--wb-focus': '#1f4f94',
		'--wb-overlay': 'rgba(36, 26, 28, 0.55)'
	},
	dark: {
		'--wb-font-heading': "'Young Serif', Georgia, serif",
		'--wb-font-body': "'Figtree', system-ui, sans-serif",
		'--wb-bg': '#171216',
		'--wb-surface': '#211a20',
		'--wb-border': '#3a2e37',
		'--wb-border-strong': '#9c8a96',
		'--wb-text': '#f6ece8',
		'--wb-muted': '#c4b2b4',
		'--wb-accent': '#ff6b6b',
		'--wb-accent-text': '#171216',
		'--wb-radius': '28px',
		'--wb-success': '#82d99a',
		'--wb-success-bg': '#15291b',
		'--wb-warning': '#f5b94d',
		'--wb-warning-bg': '#2f2312',
		'--wb-danger': '#ff8a85',
		'--wb-danger-bg': '#341619',
		'--wb-danger-text': '#171216',
		'--wb-info': '#8cb4ff',
		'--wb-info-bg': '#172238',
		'--wb-neutral-bg': '#2a2129',
		'--wb-focus': '#8cb4ff',
		'--wb-overlay': 'rgba(0, 0, 0, 0.68)'
	},
	css: `
{
	background-image: radial-gradient(circle at 20px 20px, color-mix(in srgb, var(--wb-accent) 14%, transparent) 0 5px, transparent 6px);
	background-size: 56px 56px;
}
h1,
h2,
h3 {
	font-weight: 400;
	letter-spacing: -0.01em;
}
h1 {
	font-size: 2.5rem;
	color: var(--wb-accent);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 2px solid var(--wb-border);
	border-radius: 28px 28px 28px 10px;
}
.button {
	border-radius: 999px;
	font-weight: 600;
}
`
};

export default style;

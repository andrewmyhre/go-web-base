import type { StyleDef } from '../types';

const style: StyleDef = {
	slug: 'park-sign',
	name: 'Park Sign',
	description: 'National park entrance signs: slab-serif cream letters on timber brown with forest green trail markers.',
	fonts: ['Alfa Slab One', 'Source Serif 4:wght@400;600'],
	light: {
		'--wb-font-heading': "'Alfa Slab One', Rockwell, 'Roboto Slab', Georgia, serif",
		'--wb-font-body': "'Source Serif 4', Georgia, serif",
		'--wb-bg': '#efe6d2',
		'--wb-surface': '#fbf6e9',
		'--wb-text': '#2a1d12',
		'--wb-muted': '#5e4a37',
		'--wb-accent': '#2d5a34',
		'--wb-accent-text': '#fbf6e9',
		'--wb-border': '#d3c5a6',
		'--wb-border-strong': '#7b6747',
		'--wb-radius': '4px',
		'--wb-focus': '#7a3b10',
		'--wb-success': '#2d5a34',
		'--wb-success-bg': '#e1efdc',
		'--wb-warning': '#7a4a00',
		'--wb-warning-bg': '#f8e6b8',
		'--wb-danger': '#9c2a1a',
		'--wb-danger-bg': '#f7dfd7',
		'--wb-danger-text': '#fbf6e9',
		'--wb-info': '#1f4e6b',
		'--wb-info-bg': '#dcebf2',
		'--wb-neutral-bg': '#e6dcc4'
	},
	dark: {
		'--wb-font-heading': "'Alfa Slab One', Rockwell, 'Roboto Slab', Georgia, serif",
		'--wb-font-body': "'Source Serif 4', Georgia, serif",
		'--wb-bg': '#2b1d12',
		'--wb-surface': '#3a2818',
		'--wb-text': '#f5e9cf',
		'--wb-muted': '#cdb998',
		'--wb-accent': '#e8c779',
		'--wb-accent-text': '#2b1d12',
		'--wb-border': '#56402a',
		'--wb-border-strong': '#a68c64',
		'--wb-radius': '4px',
		'--wb-focus': '#e8c779',
		'--wb-success': '#9bd39a',
		'--wb-success-bg': '#1f3320',
		'--wb-warning': '#f2c25a',
		'--wb-warning-bg': '#463211',
		'--wb-danger': '#ff9d8a',
		'--wb-danger-bg': '#4a2018',
		'--wb-danger-text': '#2b1d12',
		'--wb-info': '#8cc7e8',
		'--wb-info-bg': '#1d3342',
		'--wb-neutral-bg': '#46321f'
	},
	css: `
h1,
h2,
h3 {
	font-weight: 400;
	letter-spacing: 0.03em;
}
h1 {
	display: inline-block;
	padding: 0.35em 0.8em;
	background: #3b2616;
	color: #f5e9cf;
	border: 3px solid #f5e9cf;
	outline: 4px solid #3b2616;
	border-radius: 4px;
	box-shadow: 0 6px 0 rgba(0, 0, 0, 0.25);
}
.account-modal,
.account-menu-items,
.site-settings .setting,
.site-settings .invites {
	border: 2px solid var(--wb-border-strong);
	box-shadow: inset 0 0 0 3px var(--wb-surface), inset 0 0 0 4px var(--wb-border);
}
p {
	line-height: 1.65;
}
`
};

export default style;

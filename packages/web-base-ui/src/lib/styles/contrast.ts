// Contrast maths and the pairs every style must pass, in both themes.

export function lum(hex: string): number {
	const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
	return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}

export function ratio(a: string, b: string): number {
	const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x);
	return (hi + 0.05) / (lo + 0.05);
}

// [foreground token, background token, minimum ratio]: 4.5 for text, 3 for
// non-text (control outlines, focus rings). Token names are without `--wb-`.
export const PAIRS: [string, string, number][] = [
	['text', 'bg', 4.5],
	['text', 'surface', 4.5],
	['muted', 'bg', 4.5],
	['muted', 'surface', 4.5],
	['muted', 'neutral-bg', 4.5],
	['text', 'neutral-bg', 4.5],
	['accent', 'surface', 4.5],
	['accent', 'bg', 4.5],
	['accent', 'info-bg', 4.5],
	['accent-text', 'accent', 4.5],
	['danger-text', 'danger', 4.5],
	['info', 'info-bg', 4.5],
	['success', 'success-bg', 4.5],
	['success', 'surface', 4.5],
	['warning', 'warning-bg', 4.5],
	['danger', 'danger-bg', 4.5],
	['danger', 'surface', 4.5],
	['text', 'info-bg', 4.5],
	['text', 'success-bg', 4.5],
	['text', 'warning-bg', 4.5],
	['text', 'danger-bg', 4.5],
	['info-bg', 'info', 4.5],
	['success-bg', 'success', 4.5],
	['warning-bg', 'warning', 4.5],
	['danger-bg', 'danger', 4.5],
	['border-strong', 'surface', 3],
	['border-strong', 'bg', 3],
	['focus', 'surface', 3],
	['focus', 'bg', 3]
];

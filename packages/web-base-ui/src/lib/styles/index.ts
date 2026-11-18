// Every style a project can choose (the Copier question `style`), loaded in
// slug order. The default look is no style at all: a project that keeps it
// imports none of these files.
import type { StyleDef } from './types';
import bitmapDesktop from './data/bitmap-desktop';
import broadsheet from './data/broadsheet';
import confetti from './data/confetti';
import cutout from './data/cutout';
import decoMarquee from './data/deco-marquee';
import departuresBoard from './data/departures-board';
import jazzSleeve from './data/jazz-sleeve';
import journal from './data/journal';
import letterpress from './data/letterpress';
import midCenturyHouse from './data/mid-century-house';
import noir from './data/noir';
import paperback from './data/paperback';
import parkSign from './data/park-sign';
import pastelRegister from './data/pastel-register';
import phosphorTerminal from './data/phosphor-terminal';
import photocopyZine from './data/photocopy-zine';
import primaries from './data/primaries';
import printedBloom from './data/printed-bloom';
import rawConcrete from './data/raw-concrete';
import transitLine from './data/transit-line';

export type { StyleDef, StyleTokens } from './types';
export { REQUIRED_TOKENS, fontHref, fontSpecValid, resolveTokens, styleCss, cssErrors } from './css';
export { PAIRS, ratio, lum } from './contrast';

/** The answer that means "no style": the plain look the template ships. */
export const DEFAULT_STYLE = 'default';

export const styles: StyleDef[] = [
	bitmapDesktop,
	broadsheet,
	confetti,
	cutout,
	decoMarquee,
	departuresBoard,
	jazzSleeve,
	journal,
	letterpress,
	midCenturyHouse,
	noir,
	paperback,
	parkSign,
	pastelRegister,
	phosphorTerminal,
	photocopyZine,
	primaries,
	printedBloom,
	rawConcrete,
	transitLine
].sort((a, b) => (a.slug < b.slug ? -1 : 1));

/** The style for a slug, or undefined (which is the default look). */
export function findStyle(slug: string | null | undefined): StyleDef | undefined {
	return slug ? styles.find((s) => s.slug === slug) : undefined;
}

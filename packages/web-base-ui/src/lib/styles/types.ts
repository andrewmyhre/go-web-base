/** A design token: a custom property on :root, named `--wb-*`. */
export type StyleTokens = Partial<Record<`--wb-${string}`, string>>;

/** One selectable style for a generated project. */
export interface StyleDef {
	/** kebab-case, equal to the file name in data/ and to the Copier answer */
	slug: string;
	/** a neutral display name: no film, label, brand, place or person */
	name: string;
	/** one line on what the style draws from */
	description: string;
	/** Google Fonts css2 family specs, e.g. "Fraunces:opsz,wght@9..144,400;9..144,700" */
	fonts: string[];
	/** the tokens, per theme; a token given for one theme only is used for both */
	light: StyleTokens;
	dark: StyleTokens;
	/** extra rules, applied after the shared component rules; no @import, url() or @font-face */
	css?: string;
}

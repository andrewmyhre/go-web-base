# Styles

A generated project has one visual style, chosen when it is generated (the
Copier question `style`) and the same for everyone who uses the app: there is no
switcher. Twenty styles ship in `@andrewmyhre/web-base-ui`, plus `default`,
which is the plain look the template has always had and adds nothing.

Each style sets the colours (light and dark, following the system setting), the
radius and two Google Fonts, and adds a little CSS of its own (headings,
borders, shadows, a page texture). It reaches every component the package
renders (side menu, account menu and modal, avatar, buttons, the settings page
with its form controls and invite table) because they all read the same
`--wb-*` tokens. Every style meets 4.5:1 for text and 3:1 for control outlines
and focus rings in both themes; the package tests check the list below.

## The styles

Use the name in the first column as the `style` answer.

| `style` | Name | Inspiration |
|---|---|---|
| `default` | Default | The plain look the template ships. No stylesheet, no web fonts. |
| `bitmap-desktop` | Bitmap Desktop | 1980s black-and-white bitmap desktops: dithered backdrop, hard one-pixel shadows and pixel headings. |
| `broadsheet` | Broadsheet | A newspaper front page: high-contrast masthead headings, double column rules, newsprint cream and a book serif. |
| `confetti` | Confetti | 1980s Italian postmodern furniture: confetti patterns, chunky outlines, offset shadows and clashing pastels. |
| `cutout` | Cutout | Cut-paper poster graphics: flat vermilion blocks on cream, towering condensed capitals and thick black outlines. |
| `deco-marquee` | Deco Marquee | Art Deco (1925-1935) marquee lettering: stepped corners, thin gold double rules, gilt on black. |
| `departures-board` | Departures Board | Airport and rail departure boards: amber on black, tabular mono capitals, one row per item. |
| `jazz-sleeve` | Jazz Sleeve | 1960s jazz record sleeves: tall condensed capitals over a heavy ink bar on cream stock with a single blue. |
| `journal` | Journal Article | A scientific journal: condensed sans section heads, serif running text, hairline rules and one reference blue. |
| `letterpress` | Letterpress | Letterpress job printing: black and red ink on cream stock, double-rule borders and a fat didone display face. |
| `mid-century-house` | Mid-Century House | Mid-century modern houses: warm timber tones, one saturated accent, low wide geometric headings and a book serif. |
| `noir` | Noir Newsprint | Mid-century crime thriller posters: newsprint grey, deep black, high-contrast Didone capitals and one lipstick red. |
| `paperback` | Paperback | 1930s-60s paperback covers: coloured horizontal bands, a tracked humanist sans and cream stock. |
| `park-sign` | Park Sign | National park entrance signs: slab-serif cream letters on timber brown with forest green trail markers. |
| `pastel-register` | Pastel Register | Symmetrical pastel title cards: flat pink and plum, a geometric Futura-style face and a serif for reading. |
| `phosphor-terminal` | Phosphor Terminal | A 1970s ship-computer console: green phosphor on black, monospaced capitals, scan lines and bracket-framed cards. |
| `photocopy-zine` | Photocopy Zine | A 1990s photocopied zine: condensed capitals, hard offset shadows, cut-and-paste borders and a highlighter accent. |
| `primaries` | Primaries | Early-modernist poster geometry: flat primary colours, hard black bars and a geometric sans in capitals. |
| `printed-bloom` | Printed Bloom | 1960s Scandinavian printed textiles: one fearless colour, a faint dot print, soft serif headings and pebble-shaped cards. |
| `raw-concrete` | Raw Concrete | Brutalist web design and 1990s manual pages: monospace everywhere, heavy black borders, underlined links and one highlighter yellow. |
| `transit-line` | Transit Line | Metro wayfinding: bold line colours under headings, pill-shaped route markers and station-sign lettering. |

Fonts (loaded from Google Fonts when the style is on):

| `style` | Fonts |
|---|---|
| `bitmap-desktop` | Pixelify Sans, Work Sans |
| `broadsheet` | Playfair Display, Source Serif 4 |
| `confetti` | Lilita One, DM Sans |
| `cutout` | Anton, Libre Franklin |
| `deco-marquee` | Limelight, Crimson Pro |
| `departures-board` | Share Tech Mono, IBM Plex Sans |
| `jazz-sleeve` | Anton, Source Sans 3 |
| `journal` | IBM Plex Sans Condensed, IBM Plex Serif |
| `letterpress` | Abril Fatface, Libre Caslon Text |
| `mid-century-house` | Josefin Sans, Lora |
| `noir` | Bodoni Moda, Source Serif 4 |
| `paperback` | Josefin Sans, Crimson Pro |
| `park-sign` | Alfa Slab One, Source Serif 4 |
| `pastel-register` | Josefin Sans, Lora |
| `phosphor-terminal` | Share Tech Mono, IBM Plex Sans |
| `photocopy-zine` | Anton, Archivo |
| `primaries` | Jost, Public Sans |
| `printed-bloom` | Young Serif, Figtree |
| `raw-concrete` | IBM Plex Mono, IBM Plex Sans |
| `transit-line` | Overpass |

## Choosing and changing the style

At generation, answer the `style` question, or pass it:

```sh
uvx copier@9.19.1 copy gh:andrewmyhre/go-web-base my-app -d style=broadsheet
```

To change it later, update with a new answer (commit first):

```sh
uvx copier@9.19.1 update -d style=noir
```

The only thing the answer changes in the project is one import in
`ui/src/routes/+layout.svelte`, after `../app.css`:

```ts
import '@andrewmyhre/web-base-ui/styles/broadsheet.css';
```

So you can also edit that line by hand to another slug (and keep the answer in
`.copier-answers.yml` in step, or the next `copier update` puts it back). With
`default` there is no such line and the generated UI is the same as before
styles existed.

Fonts come from Google Fonts through an `@import` at the top of the stylesheet.
Each style names fallbacks, so the UI stays readable offline or when the font
request is blocked.

## The tokens

A style is a set of custom properties on `:root`, each `light-dark(light, dark)`:

| Token | Used for |
|---|---|
| `--wb-font-heading`, `--wb-font-body` | Font stacks |
| `--wb-bg`, `--wb-surface` | Page background; menus, cards, dialogs, inputs |
| `--wb-text`, `--wb-muted` | Body text; help text, table heads, the email line |
| `--wb-accent`, `--wb-accent-text` | Links, the current menu entry, primary buttons and the avatar placeholder, and the text on them |
| `--wb-border`, `--wb-border-strong` | Hairlines; control outlines (3:1 on the surface) |
| `--wb-radius` | Corner radius of dialogs, menus, cards (buttons and inputs use at most 12px of it) |
| `--wb-focus` | Focus ring (3:1 on the page and the surface) |
| `--wb-success`, `--wb-warning`, `--wb-danger`, `--wb-info` and their `-bg` tints | Status messages and the current menu entry; `--wb-danger-text` is the text on `--wb-danger` |
| `--wb-neutral-bg` | Hover backgrounds, inline code |
| `--wb-overlay` | The scrim behind the account dialog (optional) |

The shared rules that apply the tokens to the components live in
`packages/web-base-ui/src/lib/styles/base.ts`. A project's own pages can use the
same tokens (`color: var(--wb-muted)`) and so follow the chosen style.

## Adding or editing a style (in this repository)

1. Add `packages/web-base-ui/src/lib/styles/data/<slug>.ts` (copy a neighbour),
   import it in `index.ts`, add it to `copier.yml` and to the tables above. The
   name and description must be neutral: no film, record label, brand, place or
   person.
2. `cd packages/web-base-ui && pnpm run generate:styles` writes
   `src/lib/styles/css/<slug>.css`, the file projects import. Commit it.
3. `pnpm test` checks the contrast pairs, the fonts, the generated files, this
   page and `copier.yml` against the list.

// The rules every style shares: they paint each component the package renders
// (and the template's page chrome) from the --wb-* tokens. They use the same
// selectors as the template's app.css, so that, coming after it, they win on
// equal specificity. A style's own css comes after these.
//
// Keep every class the package's components emit in here: styles.test.ts scans
// the .svelte files and fails on one that is not.

/** Buttons: the .button class, and the plain button in the invites table. */
export const BUTTON = '.button,\n.invites button';
const BUTTON_HOVER = '.button:hover,\n.invites button:hover';

export const BASE_CSS = `:root {
	color-scheme: light dark;
	font-family: var(--wb-font-body);
}
html {
	background-color: var(--wb-bg);
	color: var(--wb-text);
}
body {
	background: transparent;
	color: var(--wb-text);
}
h1,
h2,
h3 {
	font-family: var(--wb-font-heading);
}
a {
	color: var(--wb-accent);
}
code {
	padding: 0 0.25em;
	border-radius: calc(var(--wb-radius) / 3);
	background: var(--wb-neutral-bg);
	color: var(--wb-text);
}
:focus-visible {
	outline: 3px solid var(--wb-focus);
	outline-offset: 2px;
}

/* SideMenu */
.menu {
	background: var(--wb-surface);
	border-right: 1px solid var(--wb-border);
	color: var(--wb-text);
}
.menu .group {
	margin: 0.75rem 0.5rem 0.25rem;
	color: var(--wb-muted);
	font-size: 0.8rem;
	font-weight: 600;
	letter-spacing: 0.06em;
	text-transform: uppercase;
}
.menu a {
	color: var(--wb-text);
	border-radius: min(var(--wb-radius), 8px);
}
.menu a:hover {
	background: var(--wb-neutral-bg);
}
.menu a[aria-current='page'],
.menu a[aria-current='page']:hover {
	background: var(--wb-info-bg);
	color: var(--wb-accent);
	font-weight: 600;
}
.menu a:focus-visible {
	outline: 3px solid var(--wb-focus);
	outline-offset: 2px;
}
@media (max-width: 640px) {
	.menu {
		border-right: none;
		border-bottom: 1px solid var(--wb-border);
	}
}

/* Page chrome */
.topbar {
	padding-bottom: 0.75rem;
	border-bottom: 1px solid var(--wb-border);
}

/* Avatar and UserBadge */
.avatar.placeholder {
	background: var(--wb-accent);
	color: var(--wb-accent-text);
}
.user .name,
.account-identity .name {
	color: var(--wb-text);
}

/* Buttons */
${BUTTON} {
	border: 1px solid var(--wb-border-strong);
	border-radius: min(var(--wb-radius), 12px);
	background: var(--wb-surface);
	color: var(--wb-text);
}
${BUTTON_HOVER} {
	background: var(--wb-neutral-bg);
}
.button[type='submit'] {
	background: var(--wb-accent);
	border-color: var(--wb-accent);
	color: var(--wb-accent-text);
}
.button[type='submit']:hover {
	background: var(--wb-accent);
	text-decoration: underline;
}
.button:disabled {
	opacity: 0.55;
	cursor: not-allowed;
}
.button:focus-visible,
.invites button:focus-visible,
.avatar-button:focus-visible,
.account-menu-items button:focus-visible,
.account-modal .button:focus-visible {
	outline: 3px solid var(--wb-focus);
	outline-offset: 2px;
}

/* AccountMenu */
.account-menu-items {
	border: 1px solid var(--wb-border-strong);
	border-radius: var(--wb-radius);
	background: var(--wb-surface);
	color: var(--wb-text);
}
.account-menu-items button {
	color: var(--wb-text);
}
.account-menu-items button:hover {
	background: var(--wb-neutral-bg);
}

/* AccountModal */
.account-modal {
	border: 1px solid var(--wb-border-strong);
	border-radius: var(--wb-radius);
	background: var(--wb-surface);
	color: var(--wb-text);
}
.account-modal::backdrop {
	background: var(--wb-overlay, rgba(0, 0, 0, 0.55));
}
.email {
	color: var(--wb-muted);
	opacity: 1;
}

/* SiteSettings */
.site-settings {
	display: grid;
	gap: 1rem;
	width: 100%;
	max-width: 44rem;
}
.site-settings .setting {
	display: grid;
	gap: 0.5rem;
	justify-items: start;
	padding: 1rem;
	border: 1px solid var(--wb-border);
	border-radius: var(--wb-radius);
	background: var(--wb-surface);
	color: var(--wb-text);
}
.help {
	margin: 0;
	color: var(--wb-muted);
}
.site-settings label {
	font-weight: 600;
}
.site-settings input,
.site-settings select {
	min-height: 44px;
	padding: 0.5rem 0.75rem;
	border: 1px solid var(--wb-border-strong);
	border-radius: min(var(--wb-radius), 12px);
	background: var(--wb-surface);
	color: var(--wb-text);
	font: inherit;
}
.site-settings .invite-form {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 0.5rem;
}
.status {
	margin: 0;
}
.status[role='alert'] {
	padding: 0.5rem 0.75rem;
	border: 1px solid var(--wb-danger);
	border-radius: min(var(--wb-radius), 12px);
	background: var(--wb-danger-bg);
	color: var(--wb-danger);
}
.status[role='status'] {
	padding: 0.5rem 0.75rem;
	border: 1px solid var(--wb-success);
	border-radius: min(var(--wb-radius), 12px);
	background: var(--wb-success-bg);
	color: var(--wb-success);
}
.invites {
	width: 100%;
	border-collapse: collapse;
	border: 1px solid var(--wb-border);
	background: var(--wb-surface);
	color: var(--wb-text);
}
.invites th {
	color: var(--wb-muted);
	text-align: left;
}
.invites th,
.invites td {
	padding: 0.5rem 0.75rem;
	border-bottom: 1px solid var(--wb-border);
}
.invites button {
	min-height: 36px;
	padding: 0.25rem 0.75rem;
	font: inherit;
	cursor: pointer;
}
.roles {
	color: var(--wb-text);
}
`;

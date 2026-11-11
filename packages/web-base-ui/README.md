# @andrewmyhre/web-base-ui

The reusable UI of go-web-base projects, as a Svelte 5 library. A generated
project depends on it instead of carrying copies, so a UI fix reaches it as a
version bump (through `copier update`, see below).

## What is in it

| Export | What it is |
|---|---|
| `AccountMenu`, `AccountModal`, `Avatar`, `SideMenu`, `UserBadge` | Components, from `@andrewmyhre/web-base-ui` |
| `createApiClient<paths>()`, `getMe`, `signOut`, `roleLabel`, `ApiError`, `SIGN_IN_PATH`, `LOGOUT_PATH`, types `Me`, `MeRole`, `BasePaths`, `ApiClient` | The API client helpers, from `@andrewmyhre/web-base-ui` or `@andrewmyhre/web-base-ui/api` |
| `strategyFor`, `isApiPath`, `API_PREFIXES` | The service worker policy, from `@andrewmyhre/web-base-ui/sw-policy` |

`package.json` `exports` has `svelte` and `types` conditions for the root
(components need the Svelte compiler) and `types` and `default` for `./api`
and `./sw-policy` (plain TypeScript). `svelte` `^5` and `@sveltejs/kit` `^2`
are peer dependencies (`SideMenu` reads `$app/state`). `openapi-fetch` is a
dependency.

## Using it

```ts
// src/lib/api/index.ts in the project
import { createApiClient } from '@andrewmyhre/web-base-ui/api';
import type { paths } from './openapi'; // generated from the project's own OpenAPI document

export const api = createApiClient<paths>();
```

`createApiClient<paths>` only accepts a `paths` type that has `GET /me`
answering a `Me` and `POST /auth/logout`, so a project whose API drifts from
what the helpers call fails type checking. Then `getMe(api)` and
`signOut(api)`.

```ts
// src/service-worker.ts
import { strategyFor } from '@andrewmyhre/web-base-ui/sw-policy';
strategyFor(request, origin, precached, ['/my-api-prefix']); // last argument: extra API paths
```

## Styling

The components use plain global class names (`.account-menu`, `.avatar`,
`.account-modal`, `.user`, `.menu` and friends) and ship no CSS. The project
styles them in its own `src/app.css`, which the template starts with. That is
deliberate: the look belongs to the project, and an unstyled component is
still usable. If the class names change in a release the release notes say
so.

## Developing

```sh
pnpm install
pnpm build      # svelte-package: src/lib -> dist/
pnpm test       # vitest, happy-dom
pnpm check      # svelte-check, warnings fail
```

Tests mock `$app/state`; `test/app-state.ts` stands in for it.

## Releasing

The committed version is `0.0.0`. `scripts/release.sh vX.Y.Z` (see the root
README) packs the package at `X.Y.Z` with `pnpm pack`, which gives
`andrewmyhre-web-base-ui-X.Y.Z.tgz`, and attaches it to the GitHub release
`ui/vX.Y.Z`. There is no npm registry and no credential: the template pins the
tarball's download URL in `ui/package.json`:

```
https://github.com/andrewmyhre/go-web-base/releases/download/ui%2FvX.Y.Z/andrewmyhre-web-base-ui-X.Y.Z.tgz
```

A project takes a new UI version by running `copier update`, which moves that
URL (and the lockfile). The UI version always equals the core and template
version.

Semver applies to the exports above. Before `v1` a minor version may break
them; the release notes say how.

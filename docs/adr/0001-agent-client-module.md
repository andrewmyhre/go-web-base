# ADR 0001: What the agent client module contains

Status: Proposed 2026-11-17. Approved by merging this PR. Decision record IMM-2294, first step of IMM-2081 (parent epic IMM-2082).

This ADR settles the module boundary before any file moves. It reads huddle-go at `origin/main` `12e63a6` (after IMM-2284, PR #182) and go-web-base at `dd2ffdf` (core `v0.5.0`). Figures are lines (`wc -l`) of non-test Go unless stated. Anything not run is marked **unverified**: nothing in this record was built or executed, it is read from source.

## 1. Verdict

**Proceed, narrower than IMM-2081 reads, and in an order that lets most of it start now.**

1. **The feature is `features/agent`, a Go module with Name `agent`.** It owns the conversation API and data, the harness hub, state machine and wire protocol, the vendorable Go client and the conversation runner. It owns **no** authorization, organizations, providers, projects, tokens, lab or Kubernetes code. Everything it needs from those it asks for through a small set of interfaces the app supplies (section 4).
2. **Ledger: its own, `goose_agent_version`, with no foreign keys to app tables.** Features migrate before the app, so the module cannot reference `projects` or `providers`. The module stores `project_id` and `provider_id` as plain `uuid` columns and **the app adds the foreign keys from its own ledger, which runs last** (section 3). huddle-go adopts the module without a data step: version 1 of the module is idempotent.
3. **Extraction does not wait for `app.NewRootCmd`, except where it must.** Hub, state, proto, client, UI package and the skeleton start now. The conversation routes wait for IMM-2287 (routes on core's router). Enabling the module in huddle-go waits for IMM-2288 (the root), because `Options.Features` is only read by `NewRootCmd` (section 5).
4. **Two things IMM-2081 lacks, and one of them blocks it.** Core has no way for a feature to start background work (`Installer` has no `Run`), and the hub and the conversation resume loop need one. That is an additive core change, `core/v0.6.0`, and it is a new first issue (section 5).
5. **Honest cost.** huddle-go is the only app that will host the module. role-up uses the client only, and it copies the client's files rather than importing them (section 2.4). So the server half of the module is a pilot of the feature model, as IMM-2081 says, not a response to a second host. The boundary below is drawn to keep the pilot cheap: where coupling to app concepts is heavy (activity listing, tokens, lab, providers), the code stays in huddle-go.

## 2. What moves and what stays

"Agent client" in IMM-2081 spans four things. This is the decision for each, with the size measured on huddle-go `12e63a6`.

### 2.1 Go server and data

| Piece | Lines | Decision | Why |
|---|---|---|---|
| Conversation routes and handlers: `conversations.go` 1540, `conversationpermissions.go` 175, `conversationquestions.go` 286 | 2,001 | **Moves** | The core of the feature |
| Runner: `conversationrun.go` 2242 | 2,242 | **Moves** | Drives turns against the hub |
| Attachments 858, idempotency 542, structured output 720 with `schemaregex.go` 309, unattended 184, `temperature.go` 124, `heldcalls.go` 243 | 2,980 | **Moves** | All conversation behaviour; the Go client's tests are their contract |
| Tables `conversations`, `conversation_messages`, `conversation_events`, `conversation_attachments` and the idempotency keys (migrations 000010, 000011, 000017, 000018, 000020 to 000023, 000025 to 000027, as the squashed baseline holds them); queries `conversations.sql`, `idempotency.sql` (470) | n/a | **Moves**, as `goose_agent_version` migrations | Section 3 |
| Harness hub `hub.go`, state machine `state.go`, `runtime.go` (the `RuntimeDriver` interface), local driver, proto and generated `harnessv1`, `docs/harness-contract.md` | about 2,950 plus generated | **Moves** | `harness/` imports nothing else of huddle-go (checked: the only `huddle-go/` imports under `harness/` are its own packages), so it lifts out whole |
| Kubernetes driver `harness/runtime/kube` | 929 | **Stays** | It encodes deploy policy: namespaces, network policies, project claims, private-egress lists, start deadline. About eight of huddle-go's config keys (`runtime_kubernetes_*`) feed it. The module sees only `runtime.RuntimeDriver` |
| Harness images `harnesses/claude` and their TypeScript | n/a | **Stays** | An image is a deployment artefact. The wire contract it implements moves; the cross-check is section 6.4 |
| Lab: `lab.go`, `labapi.go`, `labrun.go`, `labenv.go`, `labscrub.go`, `labtools.go` | 2,924 | **Stays** | An organization feature built on providers; conversations reach it only through the `Launcher` interface |
| Provider slots (`providerslots.go`) and provider egress | 320 | **Stays** | Shared by lab and conversations today (IMM-2017); the module takes `Slots` |
| Project tokens: `token.go`, `projecttokens.go`, `db/project_tokens.go`, migration 000012 | 646 | **Stays** | IMM-2296 lists them; that was wrong. A token is authentication (the `Middleware` hook) plus a role the app's Guard decides on, and its table has a foreign key to `projects`. The module learns only the caller's subject, `token:<name>`, which it already stores in `created_by` |
| Activity listing and counts: `activity.go` | 644 | **Stays** | Its three queries join `conversations` to `projects` and `organizations` and count per organization for the Admin. Moving it needs a directory interface for no second host. Revisit if a second host appears. The module exports the hub's live view the handlers read |
| Projects, providers, project choices, organizations, users, invitations, `authz.go`, guard | about 6,000 | **Stays** | App concepts (huddle-go ADR 0001, sections 2 and 3) |

So about 7,200 lines of server code plus the hub (about 2,950) and the client (4,294) move; lab, tokens, activity, providers and the Kubernetes driver stay.

### 2.2 The client

`client/` (4,294 lines, plus `docs/` and the `zaplog` subpackage) moves to **`features/agent/client`**, same package name, same exported API, as a package of the agent module and not a module of its own. Reasons:

- Its non-test files import only the standard library (its README states it and `grep 'huddle-go' client/*.go` over non-test files returns nothing). A separate module is not needed to keep `go get` light: Go's module graph pruning fetches only `go.mod` files for the rest of the agent module's requirements.
- Its tests import huddle-go's generated `harnessv1` for wire fixtures, which would be a cycle between two modules.
- A test in the module keeps the property that makes it vendorable: **a non-test file in `client/` may import the standard library and its own subpackages only.** `zaplog` is the one allowed exception, as today.

### 2.3 The UI package

A new `@andrewmyhre/web-base-agent-ui`, released as the GitHub release `agent-ui/vX.Y.Z` (section 6.3). It holds what is **pure or presentational**: `agentEvent.ts` (418 lines), `activity.ts` (488), `activityLayout.ts`, `activityPanel.ts`, `displayFilter.ts`, and the components `ConversationStream`, `PermissionPrompt`, `QuestionCarousel`, `AgentTree`, `StateIcon`, `ActivityPanel`, `ActivityList`, `ActivityCounts`, `StreamFilters`.

It must not import huddle-go's `$lib/api`, `userDirectory`, `phone` or `routing`, which today it does (measured: `PermissionPrompt`, `QuestionCarousel`, `StreamFilters`, `ActivityPanel` and `ActivityList` import them). The package defines the types it uses and takes the rest as props or a single `AgentUiContext` (an `api` object with `answerPermission`, `answerQuestion`, `getSettings`, `patchSettings`, a `displayName(sub)` function and href builders). **The pages stay in the app**, as IMM-2299 says, and so do `ProjectConversations`, `ActivityFullView` (it calls `goto` and `getMe`) and `ProjectTokens` (tokens stay). This is the largest rewrite in the epic, not a lift, and IMM-2299's done condition "no huddle-specific string remains" is the right test for it.

### 2.4 role-up

IMM-2298 says role-up's import must not break. **It does not import huddle-go at all.** role-up (IMM-2148, Done) holds a verbatim copy of `client/` at `internal/third_party/huddleclient` with the package clause rewritten, recorded in its `SOURCE.md` (tag `v0.2.0`, commit `545acad`) and refreshed by `scripts/sync-huddle-client.sh <tag>`. It copies because huddle-go is private and role-up's CI has no credential. So:

- No alias package or `replace` is needed. The one change to role-up is the script's source: the new path and the repository.
- go-web-base is public, so role-up **could** drop the copy and import `features/agent/client` directly. That is an option, not part of this epic. Do it only if the copy costs more than it saves.
- role-up's pin is huddle-go `v0.2.0`. The moved client must stay API-compatible with it, and the client's own tests are the check (`client/README.md` documents the IMM-2204 and IMM-2198 migrations; they stay).

## 3. The migration ledger

**Question.** Does the agent feature get its own goose ledger, given that its tables reference `projects` and `providers`, which live in the app, and features migrate before the app?

**Decision: yes, `goose_agent_version`, and no foreign key from a module table to an app table.**

The facts, read from source:

- Core runs `core`, then each feature in `Features` order, then `app` (`core/db/db.go` `MigrateWithConfig`; `core/app/feature.go` `Installer.Migrations`). huddle-go's migration `000010` makes `conversations.project_id` `REFERENCES projects (id) ON DELETE CASCADE` and `provider_id` `REFERENCES providers (id) ON DELETE SET NULL`. On a fresh database a feature ledger would run that before `projects` exists.
- Only `conversations` points at app tables. `conversation_messages`, `conversation_events`, `conversation_attachments` and the idempotency keys point only at `conversations`. `project_tokens` also points at `projects`, but it stays in the app (section 2.1).
- `provider_name` and `model` are stored on the row, so a conversation survives its provider being deleted; only `provider_id` is lost, which `SET NULL` expressed.
- huddle-go squashes to one baseline (IMM-2281, D2) and recreates dev and prod with no data kept. After the prod cutover prod will hold data, so adoption must not need a data step.

**The rule.** The module's tables carry `project_id uuid NOT NULL` and `provider_id uuid` as plain columns with indexes and no `REFERENCES`. **The app owns the foreign keys** and adds them in a migration of its own, which runs after the module's. The module documents the two constraints it expects (`conversations.project_id` to the app's projects with `ON DELETE CASCADE`, `conversations.provider_id` to the app's providers with `ON DELETE SET NULL`), names them (`conversations_project_id_fkey`, `conversations_provider_id_fkey`, which are also Postgres's default names), and ships a test helper that asserts them. An app that omits them still works: a dangling `provider_id` is resolved at use and an unknown project is refused by the app's guard before the handler runs; only the cascade is lost, and the module offers `Store.PurgeProject(ctx, projectID)` for an app that does not want the foreign key.

**Adopting it in huddle-go without a data step.** Module migration `00001` is written with `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` and equals huddle-go's agent tables as they stand in its baseline. Two paths, which must dump to the same schema:

| Database | What happens | Result |
|---|---|---|
| Existing (baseline already applied, tables and foreign keys present) | Module `00001` finds the tables, changes nothing and is recorded in `goose_agent_version`. The app's baseline is never re-run | Tables as before, foreign keys under the default names |
| Fresh | Module `00001` creates the tables without foreign keys. The app's baseline, edited in the same change to drop those tables, creates the rest. An app migration then adds the two constraints under the same names | Same schema |

The proof is IMM-2281's own: dump both and diff, expecting empty, and run it on both paths in IMM-2302. The one thing it asks of huddle-go is a one-time edit of an already applied baseline (goose does not check applied files, and it is only the fresh path that reads the edit). If IMM-2302 lands before the prod cutover there is no existing database and the edit is free, which is the reason section 5 prefers that order, but nothing forces it.

**Why not the alternatives.**

- *Tables stay in the app ledger, the module ships SQL files the app includes.* Keeps the foreign keys and needs no adoption, but the module is then not self-contained: its schema changes would arrive as files an app must copy into its own ledger, which is the copy-and-merge problem the feature model exists to remove, and `copier update` could not render a migration into a numbered folder.
- *Make core run the app first.* One option field would fix ordering, but it changes the order every existing app and feature has been written against (core, features, app), for one module's foreign key. The app-owned foreign key costs less.
- *A second ledger adoption copying `goose_app_version` rows into `goose_agent_version`.* The huddle-go ADR section 8 anticipated this. It needs a run-once step and a test, and the idempotent `00001` makes it unnecessary.

## 4. Coupling: how the module reaches the app

The module imports `core` and nothing from the app. The app passes an `agent.Options` to `agent.New`. Everything below is an interface or a function the app supplies; each is something a handler calls today as a method on a huddle-go type.

### 4.1 The interfaces the app supplies

| Name | Method shape | Replaces today |
|---|---|---|
| `Caller` | `func(ctx) (Caller, bool)` with `Caller{Sub string, Token bool}` | `SessionFromContext` plus the token identity `token:<name>` |
| `OwnOnly` | `func(ctx) bool` | `server.OwnOnly(ctx)`, set by the guard when only `ReadOwn` or `CreateOwn` passed |
| `Projects` | `Project(ctx, id) (Project, error)` with `Project{ID, OrganizationID, Slug}`; `ProvidersOf(ctx, projectID)`; `ProviderByName(ctx, orgID, name)`; `Choices(ctx, orgID)` | `GetProject`, `ListProvidersOfProject`, `GetProviderIDByName`, `ListProviderChoices` on `conversationQueries` |
| `Launcher` | `Plan(ctx, orgID, LaunchConfig) (LaunchPlan, error)` and `WithEgress(ctx, LaunchPlan) (LaunchPlan, error)`; `LaunchPlan` carries the harness kind, image, env, secret, provider id and name, and egress endpoints | `LabAPI.plan` and `withEgress`, which check harness, provider and model and build the env |
| `Slots` | `Acquire(ctx, providerID, limit) (release func(), err error)` | `providerSlots`, shared with the lab so a provider's limit counts both |
| `Driver` | `runtime.RuntimeDriver` | the Kubernetes or local driver chosen in `main.go` |
| `Config` | hub listen and advertise addresses, permission timeout, attachment origins and private CIDRs, the API path prefix | five of huddle-go's 34 config keys, plus the `/api/v1` prefix |

`LaunchPlan` and `Project` are plain structs in the module, so neither imports `sqlc` types; the app converts. This is the part most likely to grow when the code is actually moved, and IMM-2296 should treat each interface as a unit of review.

### 4.2 Authorization, without importing the app

- The module registers every route through `server.Register` with a permission scoped by `.In(server.ScopeProject)`, as core `v0.5.0` provides. The app's **Guard** decides it from the `{projectID}` route parameter, exactly as it decides huddle-go's own routes today. The module contains no authorization.
- The Guard tells the handler when the request passed only through an Own variant by setting a context value the app's `OwnOnly` reads. That is today's mechanism (`ownOnlyKey` in `server/context.go`), moved behind a function.
- Project tokens never reach the module as a type. The app's `Middleware` authenticates the bearer token and the Guard authorizes it; the handler sees `Caller{Sub: "token:ci", Token: true}`.

### 4.3 Permissions and resources joining the app's roles

The module declares three resources with `Installer.Resource`: `AgentConversations` (extra verb `ReadOwn`), `AgentMessages` (`ReadOwn`, `CreateOwn`) and `PermissionPrompts` (`Answer`), the names huddle-go uses today, so no role or stored `p` rule changes. It grants nothing itself. The app passes grants as **named bundles**, because the role graph is the app's:

```go
agent.New(agent.Options{
	Grants: map[string][]agent.Bundle{
		"Contributor":        {agent.Use},        // read, create, patch, answer
		"Agent Client":       {agent.Own},        // create, plus ReadOwn and CreateOwn
		"Project Agent Client": {agent.Wide},     // create and read every conversation of the project
	},
})
```

Each bundle expands to the permission strings huddle-go's `roles/roles.go` lists today (read from lines 139 to 141 and 213 to 222: `Use` is `AgentConversations: Read, Create`, `AgentMessages: Read, Create, Patch` and `PermissionPrompts: Answer`; `Own` and `Wide` are the two token roles' lists). core refuses a grant to a role that does not exist, naming the feature and role, so a typo fails at startup.

At IMM-2302 the three resources and these lines leave huddle-go's `roles/roles.go`; **the gate is the existing golden (`roles/testdata/before.json`) and the 172 `p` rules, which must come out identical**, with the module's grants merged in. Resources are not registered twice: core refuses a duplicate.

### 4.4 Paths, settings and lifecycle

- **API prefix.** Core's `APIPrefix` applies only to `/me`, `/auth/logout` and `/openapi.json`. Feature routes name their own paths, so `Config.Prefix` (default `""`; huddle-go passes `/api/v1`) is prepended by the module. Without it the module could not serve huddle-go's wire contract.
- **Settings.** The module declares its five settings through `Installer` as `<PREFIX>_AGENT_*`, like any feature, and `agent.Options` may override each with a value. huddle-go keeps its `HUDDLE_*` names (huddle-go ADR D4) by reading them in its `Options.Settings` and passing the values. The module's own flags then go unused in huddle-go, which is the price of D4; a new project gets the `AGENT_*` names.
- **Lifecycle.** The hub must listen before the first request and the runner must resume queued conversations after a restart. A feature has no hook for that. **Core gains `Installer.Run(func(ctx context.Context, d RunDeps) error)`**, with the semantics `Options.Run` already has (starts before the listener, cancelled on shutdown, fatal on error, waited for before the pool closes). It is additive, so a minor bump, `core/v0.6.0`. Without it the app would start the hub itself, and the Copier glue would no longer be one line.
- **Driver and secrets.** The app builds the `Driver` and the `Launcher`, so the credentials key, the provider secrets and the Kubernetes settings never enter the module.

### 4.5 What the module still needs from the database

It takes `Deps.Pool` and runs its own `sqlc` queries over its own tables. It never reads `projects` or `providers`: those come through `Projects`. This is why the activity queries stay in the app (they would be the one place the module joined an app table).

## 5. Sequencing against IMM-2080

**Question.** Should extraction wait until huddle-go runs on `app.NewRootCmd`?

**Answer: no for most of it, yes for the last step.** The revised huddle-go ADR (section 6.2, PR #183) puts the root last (IMM-2288) because the hooks only work inside `NewRootCmd`. Features are installed only from `Options.Features`, so the same holds: **a feature cannot be enabled in huddle-go before 2288.** But building the module needs no huddle-go change, and it should not wait.

| Step | Where | Waits for | Why |
|---|---|---|---|
| A. `Installer.Run`, core `v0.6.0` | go-web-base | nothing | New issue. Blocks the runner and the hub's start |
| B. IMM-2294 (this) | go-web-base | nothing | Docs only |
| C. IMM-2295 skeleton, with the interfaces of 4.1 and an empty migration | go-web-base | B | No code moves |
| D. IMM-2297 hub, state, proto, local driver | go-web-base | C | `harness/` imports nothing app-side, so it lifts out without waiting on any huddle-go change |
| E. IMM-2298 client | go-web-base | C | Parallel with D |
| F. IMM-2299 UI package | go-web-base | B | Independent of the Go work; needs 2.3's context API |
| G. IMM-2296 data, runner, routes | go-web-base | **IMM-2287**, A | Routes must be on core's `server.Register` before they move, or they are ported twice. The data layer (migration, queries, store) can start after C and ship first |
| H. IMM-2300 toggle and glue, IMM-2301 release | go-web-base | D, E, G | The template's `openapi.json` sections are the module's routes |
| I. IMM-2302a Go side in huddle-go | huddle-go | **IMM-2288**, H | By hand, with the same glue the template renders; deletes the local server, db, harness and client copies |
| J. IMM-2302b `copier update` and the UI package | huddle-go | **IMM-2293**, F | The answers file exists only after the prod cutover (D3) |

**Where this sits against the cutover.** Do step I before the prod cutover when the module is released in time, because both ledgers are then built from empty in prod and the baseline edit in section 3 is free. **Do not hold the cutover for it.** If I is not done when the dev soak ends, cut over without the module and adopt later; section 3's adoption is idempotent and proven on both paths, so it is safe either way. This keeps the huddle-go ADR's D3 (cutover diff is server only) intact.

One rule while D and E run: huddle-go's `harness/` and `client/` are copied while huddle-go keeps changing them. Take each snapshot at a recorded SHA, land it in one PR, and mirror any change to those directories in huddle-go to the module until I switches the imports. IMM-2297 and IMM-2298 should record the SHA they copied from.

## 6. Settled details

### 6.1 Naming

The module's name is `agent`: log scope `agent`, ledger `goose_agent_version`, settings `<PREFIX>_AGENT_*`, Go module `github.com/andrewmyhre/go-web-base/features/agent`. The Copier question follows `docs/consuming.md` (`feature_<name>`), so it is **`feature_agent`**, not `feature_agent_client` as IMM-2300 and IMM-2302 write. The server serves "conversations" while the client says "sessions", as the client README already explains; the module keeps the server's names on the wire.

### 6.2 What the template renders

For `feature_agent=true`: the `main.go` import and `Features` entry with the grants of 4.3, the `go.mod` require, the `deploy/dev` settings, the `/api` ingress and service worker prefixes (already present), the UI package dependency, `README.md` and `llms.txt` lines, and the module's routes in `ui/openapi.json` and `openapi.d.ts`. The agent routes are a large part of huddle-go's document, so the conditional sections in the template's `openapi.json` become long; if that is unwieldy IMM-2300 should render them from a file the module ships rather than inline. **Unverified:** the size, it was not generated.

### 6.3 Releases

`scripts/release.sh` has `FEATURES=(example)`; IMM-2301 adds `agent` and keeps core and every feature at one version, as `docs/consuming.md` says. The UI package needs its own release name: `ui/vX.Y.Z` is `web-base-ui`'s, so the agent package is released as the GitHub release **`agent-ui/vX.Y.Z`**, and the template pins both URLs.

### 6.4 The wire cross-check moves

Today the Go wire fixtures (`harness/hub/testdata/wire/*.bin`) and the TypeScript harness's output (`ts-*.bin`, written by `harnesses/claude/scripts/write-wire.ts`) are compared in one repository's CI. After the move go-web-base CI cannot see the harness. The Go fixtures and `docs/harness-contract.md` live with the proto in the module; the TypeScript side keeps its tests in huddle-go and reads the module's fixtures from the pinned module directory (`go list -m -f '{{.Dir}}'`). A proto change is then caught on huddle-go's bump PR instead of in the same PR. State this in the module's README; IMM-2297's done condition ("the cross-check passes") needs that wording. The proto source for TypeScript generation (`proto/buf.gen.ts.yaml`) reads from the module too.

### 6.5 Tests

The module's tests that need Postgres move with it and run in go-web-base CI against the same Postgres and Redis services the `core` step uses. The client's tests are the contract for the routes, as the huddle-go ADR already says. The runner tests are the largest set (2,242 lines of runner, plus its tests); **unverified:** how many depend on `sqlc` types the module will replace with its own.

## 7. The children of IMM-2081

All eight are right in intent and listed in order; five need a correction. Each gets a comment, and no description is edited. The corrections:

| Issue | Correction |
|---|---|
| IMM-2295 skeleton | Add the `agent.Options` interfaces of 4.1, the three resources and the grant bundles, and the `Installer.Run` dependency. `go.work` is not in the repo; a release workflow entry is `release.sh`'s `FEATURES` |
| IMM-2296 data, routes | **Too big for one issue** (7,200 lines of server plus 470 of SQL; a local run that edits nothing for long stops on `no file edited`). Split into (a) data layer: migrations, queries, store, adoption test; (b) runner, routes, permissions and questions; (c) attachments, idempotency, structured output, unattended, temperature. **Remove tokens** (they stay, 2.1). Not blocked by IMM-2280 only: also IMM-2287 and the `Installer.Run` issue. "as goose_agent_version migrations" gains: no foreign keys, `IF NOT EXISTS`, section 3 |
| IMM-2297 hub | `harness/` has no huddle-go imports, so it needs no unpicking. Add the wire cross-check wording of 6.4 and recording the source SHA |
| IMM-2298 client | **Role-up does not import it**, so drop the alias or `replace` (2.4). Add the stdlib-only test, `zaplog`, the docs folder, and the one change to role-up's `sync-huddle-client.sh` |
| IMM-2299 UI | It is a rewrite, not a lift: five components import huddle-go's `$lib/api`, `userDirectory`, `phone` and `routing`. Name the `AgentUiContext` of 2.3. Release name `agent-ui/vX.Y.Z` |
| IMM-2300 toggle | The question is `feature_agent`. The `openapi.json` size risk of 6.2. Needs the module's routes, so after G |
| IMM-2301 release | Add `agent` to `FEATURES`, the `agent-ui/v` release |
| IMM-2302 enable | Split: (a) Go side by hand after IMM-2288 and the release, golden `p` rules identical, the two-path schema dump; (b) `copier update` and the UI package after IMM-2293, because the answers file is created there. Toggle name `feature_agent` |

One issue is missing: **core `Installer.Run`** (section 4.4), a go-web-base issue under IMM-2081, estimate 3, blocking 2296(b).

## 8. Decisions

**None need Andrew's answer beyond approving this PR.** The forks the evidence settled, so that the record shows they were weighed:

- *Does the module own project tokens?* No. Evidence: the table references `projects`, the `Middleware` and Guard hooks already carry the logic, and the module needs only the subject string.
- *One ledger or two?* Own ledger, app-owned foreign keys, idempotent version 1 (section 3). The alternatives each fail on a stated fact.
- *Wait for the root?* Only the enabling step and the routes (section 5).
- *Before or after the prod cutover?* Before if the module is ready, never blocking it. This is a scheduling preference, and it is safe either way.
- *Should role-up import the client?* Not in this epic.

## 9. Alternatives rejected

- **Move everything under "agent" including lab, activity, tokens and the Kubernetes driver.** About 5,500 more lines (lab 2,924, tokens 646, activity 644, Kubernetes driver 929, slots 320), all coupled to providers, organizations or deploy policy, for a host that does not exist. The module would import the app, which is the thing the feature contract forbids.
- **Client and UI only, no server module.** Cheaper, and the only part with a second consumer today (role-up). Rejected because IMM-2081 and the epic exist to prove the whole feature model (module, UI package, toggle, update), and a model proven only on a client proves nothing about migrations, grants or routes. If the server work stalls on 4.1, this is the fallback: D, E and F stand alone.
- **Per-feature order in core (app before features).** Section 3.
- **Foreign keys in the module with a lazily created `projects` stub.** Would make two definitions of one table.

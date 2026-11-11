# go-web-base

A [Copier](https://copier.readthedocs.io) template for a Go API and a
SvelteKit PWA with Google sign-in and Casbin RBAC, generated from
`andrewmyhre/huddle-go` at `v0.1.0` (commit `6028ffc`).

## What a generated project has

- A Go API on chi and Huma. Every route goes through `server.Register` with a
  typed `server.Permission`, `server.Public` or `server.Authenticated`, and an
  unchecked route fails `Build`.
- Google OIDC sign-in: browser-bound state taken with `GETDEL`, PKCE, a nonce,
  the ID token verified with go-oidc, and sessions in Redis rotated at sign-in.
- Casbin RBAC with project domains plus `global`, the example role graph in
  `docs/roles.md` (six roles, edit them per project), an admin bootstrapped
  from config, and users with no binding refused at sign-in.
- `GET /me` and `POST /auth/logout`.
- A SvelteKit 2 / Svelte 5 PWA in `ui/`, whose API client is generated from the
  API's OpenAPI document. Drift fails CI.
- Postgres via pgx, sqlc and goose. Redis via go-redis.
- zap logging with scopes `api`, `rbac`, `queue` and `agent`, for Loki, with a
  Grafana dashboard in `deploy/grafana/`.
- A Woodpecker pipeline: tests, lint, UI checks, BuildKit image builds, and
  `deploy-dev` with a `/health` and `/` smoke check.

## Use

For how projects are meant to build on the base over time (versioned
packages, feature toggles, what belongs where), read
[docs/consuming.md](docs/consuming.md).

Copier is pinned to **9.19.1**. Run it through `uvx`; nothing needs installing
beyond [uv](https://docs.astral.sh/uv/).

```sh
uvx copier@9.19.1 copy gh:andrewmyhre/go-web-base my-app
```

Later, to merge template changes into the project (a three-way merge against
the answers in `.copier-answers.yml`; commit first):

```sh
cd my-app
uvx copier@9.19.1 update
```

### Questions

| Question | Default | Used for |
|---|---|---|
| `project_slug` | none | binary, images, Deployments, ServiceAccount, dashboard |
| `project_name` | slug in title case | UI title, PWA manifest, prose |
| `github_repo` | `andrewmyhre/<slug>` | references to the repo |
| `go_module` | `github.com/<github_repo>` | `go.mod` and every import |
| `env_prefix` | slug in upper case, `-` as `_` | every `PREFIX_*` variable |
| `dev_host` | `<slug>-dev.immutablesoftware.dev` | ingress, redirect URI, docs |
| `dev_namespace` | `<slug>-dev` | dev environment |
| `test_namespace` | `<slug>-test` | test Postgres and Redis |
| `registry` | `192.168.0.10:30500` | image names |
| `test_postgres_nodeport` | none | test Postgres; must be free on the cluster |
| `test_redis_nodeport` | none | test Redis; free, and not the Postgres port |
| `admin_sub` | empty | the global Admin's Google sub |

Two values are derived and never asked: the lower case of `env_prefix` names
the cookies (`<ident>_session`, `<ident>_oauth_state`), the Postgres user and
database, the Redis key prefixes and the root Go package; the first label of
`dev_host` is the Cloudflare subdomain.

The two NodePorts have no default, because a default would collide for the
second project that kept it. Find free ones with `kubectl get svc -A`. The
validator rejects ports outside 30000-32767, the same port twice, and the
ports in `taken_nodeports` in `copier.yml`: a snapshot of services on the
cluster that were not generated from this template. Never add a generated
project's ports to that list, because its `copier update` would then fail
validation.

## Per-project setup checklist

Nothing in a generated project works on the cluster until these are done.
Each one needs a human or an admin kubeconfig. `<slug>`, `<prefix>` and so on
are your answers.

1. **GitHub and Woodpecker.** Create the repo, push the generated project,
   and enable it in Woodpecker. Mark it trusted: `deploy-dev` sets
   `backend_options`.
2. **Google OAuth Web client.**
   - Authorized origin: `https://<dev_host>`.
   - Redirect URI: `https://<dev_host>/auth/google/callback`.
   - The OAuth consent screen's authorized domains must include the host's
     domain.
3. **Seal the secrets** with `kubeseal` (controller
   `sealed-secrets-controller` in `kube-system`). The commands are in the
   generated `deploy/test-services/README.md` and `deploy/dev/README.md`:
   - `test-services` (`password`) in `<test_namespace>`;
   - `dev-services` (`postgres-password`, `redis-password`) in
     `<dev_namespace>`;
   - `<slug>-google-oauth` (`client-id`, `client-secret`) in
     `<dev_namespace>`.

   The template never ships a SealedSecret: each is encrypted for one cluster
   and namespace.
4. **One-time applies** from `deploy/test-services` and `deploy/dev`: the
   namespaces, `rbac.yaml`, Postgres and Redis, then the first app rollout
   (`deploy/dev/README.md`, step 3).
5. **Woodpecker repo secrets** for the test services:
   `<prefix>_TEST_DATABASE_URL`, `<prefix>_TEST_REDIS_ADDR` and
   `<prefix>_TEST_REDIS_PASSWORD` (values in
   `deploy/test-services/README.md`). Add them to the right repo: huddle-go
   once had them on the wrong one, and the only symptom was CI stuck on
   pending.
6. **Cloudflare route, DNS and Access app.** Needs Andrew's yes. The default
   zone is `immutablesoftware.dev`:

   ```sh
   python3 ~/.claude-test/skills/cloudflare-zero-trust/scripts/cf_ops.py \
     add-service "<project_name> Dev" <dev_subdomain>
   ```

7. **The admin.** Set `<prefix>_ADMIN_SUB` in `deploy/dev/api.yaml` to the
   owner's Google sub (digits, never an email). If you left `admin_sub` empty,
   nobody can sign in yet, so `/me` cannot tell you. Sign in once anyway: the
   refusal logs an `rbac` deny line whose `subject` is the sub.

   ```sh
   kubectl -n <dev_namespace> logs deploy/<slug>-api | grep '"decision":"deny"'
   ```

## Developing the template

- `copier.yml` holds the questions. `template/` is what gets rendered.
- Only files where a value varies are templated, and only those carry the
  `.jinja` suffix. Paths are templated too (`cmd/[[ project_slug ]]/`).
- The delimiters are `[[ ]]`, `[[% %]]` and `[[# #]]`, not Jinja's defaults:
  Svelte's `{#if}` is a Jinja comment opener, and Go uses `[%q]`. A YAML flow
  list of one templated value needs quotes (`["[[ project_slug ]]"]`), or
  `[[[` confuses the lexer.
- The generated project's Postgres and Redis tests fail rather than skip when
  their env vars are unset. `<prefix>_SKIP_SERVICE_TESTS=1` is the one
  deliberate skip. Template CI uses it, because it has no test services.
  `pipeline_test.go` fails if a generated project's own pipeline ever sets it.

### Template CI

`.woodpecker.yml` renders `sample/` from the commit under test (with
`--vcs-ref HEAD`, since Copier otherwise renders the latest tag), then:

- sweeps it for `huddle` and for unrendered `[[ ]]` tags, with a control
  render using huddle answers that must bring the string back;
- runs `gofmt`, `go vet` and `go test` with the service tests skipped;
- runs the UI's `check:client`, `test`, `check` and `build`.

The sample's own image build and deploy steps never run: Woodpecker reads only
the root `.woodpecker.yml`.

To run the same checks locally:

```sh
uvx copier@9.19.1 copy --vcs-ref HEAD --defaults --trust . /tmp/sample \
  -d project_slug=sample-app -d github_repo=example/sample-app \
  -d test_postgres_nodeport=30601 -d test_redis_nodeport=30602
cd /tmp/sample
test -z "$(gofmt -l .)" && go vet ./... && SAMPLE_APP_SKIP_SERVICE_TESTS=1 go test ./...
cd ui && pnpm install --frozen-lockfile && pnpm run check:client && pnpm test && pnpm check && pnpm build
```

### Regression control

Rendering with huddle answers reproduces `huddle-go` `v0.1.0` except for
deliberate changes: the sealed secrets and `docs/plan-high-level.md` are
dropped, the Roles section moved to `docs/roles.md`, the skip switch was added,
the setup docs gained the sealing commands, and the old Linear issue
references were removed. huddle-go's NodePorts are on the taken list, so the
control uses others, and the port lines differ too.

```sh
git -C ~/projects/huddle-go archive v0.1.0 | tar -x -C /tmp/huddle-v0.1.0
uvx copier@9.19.1 copy --vcs-ref HEAD --defaults --trust . /tmp/huddle-render \
  -d project_slug=huddle-go -d project_name=Huddle -d go_module=huddle-go \
  -d env_prefix=HUDDLE -d dev_host=huddle-dev.immutablesoftware.dev \
  -d test_postgres_nodeport=30601 -d test_redis_nodeport=30602
diff -r /tmp/huddle-v0.1.0 /tmp/huddle-render
```

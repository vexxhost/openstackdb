# AGENTS.md

Guidance for AI coding agents working in this repository. Human-facing docs are in `README.md` (usage, filter semantics) and `API.md` (API reference).

## Project

`github.com/vexxhost/openstackdb` is a **read-only** Go library of typed queries against OpenStack service databases (MySQL/MariaDB). It was extracted from VEXXHOST's OpenStack Database Exporter, which is its main downstream consumer. Runtime packages must not depend on Prometheus or OpenStack API clients.

## Layout

| Path | What it is |
| --- | --- |
| `db.go`, `dsn.go`, `filters.go` | Root package: `Connect`/`ConnectContext`, oslo.db URL → go-sql-driver DSN, shared `Deleted` policy |
| `<service>/db/` | One package per service (cinder, glance, heat, ironic, keystone, magnum, manila, neutron). Nova is split into `nova/db/main` (cell DB) and `nova/db/api`. Octavia is `octavia/db/repositories`, Placement is `placement/objects` |
| `<pkg>/queries.sql` | sqlc query source |
| `<pkg>/queries.sql.go`, `models.go`, `db.go` | **sqlc-generated, do not edit by hand** |
| `<pkg>/filters.go` | Hand-written `...GetAllByFilters` methods (nova, cinder, glance) built on `internal/filter` |
| `internal/filter` | `Builder` for parameterized `WHERE` clauses from trusted column names |
| `schema/<service>/*.sql` | Minimal schema **test fixtures** embedded via `schema.Files`. They are not migrations and are also the sqlc schema input |
| `internal/testutil` | MariaDB testcontainers helpers (`NewMySQLContainer`, `SeedSQL`) |
| `tests/` | Integration tests (`//go:build integration`) |
| `.ai/` | Tool-agnostic agent workflows (see below) |

## Commands

```sh
go test -race ./...                                      # unit tests
go vet ./...
gofmt -l .                                               # must print nothing
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0     # pinned version, must match CI
sqlc generate                                            # regenerate after editing queries.sql or schema/
go test -race -tags integration -timeout 15m ./...       # integration tests; needs Docker
```

CI (`.github/workflows/ci.yaml`) runs all of the above. It fails if `sqlc generate` produces a diff, and it also runs the downstream exporter's integration tests against this module, so exported API changes can break CI even when this repo's tests pass.

Integration tests pass schema paths like `"../sql/nova/schema.sql"`. `testutil` maps any path containing `sql/` onto the embedded `schema/` files, so there is no `sql/` directory on disk.

## Conventions

- **Read-only.** Only `SELECT`. No writes, DDL, `FOR UPDATE`, or `multiStatements`.
- **New queries go in `queries.sql`.** Run `sqlc generate` and commit the generated output. If a new column or table is needed, add it to the matching `schema/<service>/schema.sql` fixture.
- **Naming follows upstream OpenStack DB APIs** where an operation corresponds (e.g. `InstanceGetAllByFilters`, `instance_extra_get_by_instance_uuid` → `InstanceExtraGetByInstanceUUID`). Keep existing exported names stable, or add a compatibility mapping in `API.md`.
- **Dynamic SQL only through `filter.Builder`.** Column names must be string literals; values always go through `?` placeholders. Never `fmt.Sprintf` caller data into SQL.
- **Filter semantics are a contract** (documented in `README.md`): fields combine with AND, slice values combine with OR, a `nil` slice means unrestricted, and a non-nil empty slice matches nothing. The zero `Deleted` value excludes soft-deleted rows. Lifetime windows must be both-zero or `start < end`. Invalid input returns an error and never widens the result.
- **Return raw data.** No unit conversion, rounding, aggregation, authorization, or cell discovery. Those belong to callers.
- **Tests.** Unit tests use `go-sqlmock` next to the code. Behavior against real SQL goes in `tests/` with `testutil.NewMySQLContainer` and `SeedSQL`.
- **Formatting and commits.** Run `gofmt`. Commit subjects are short, imperative, and sentence case (e.g. "Let Cinder active volume queries choose their index").
- **Docs.** Update `README.md` / `API.md` when exported behavior changes.

## Security

Filter values may come from end users of the calling service. Treat them as untrusted. The oslo.db connection string is trusted operator configuration, but it holds credentials: never echo it, or the DSN built from it, in errors or logs.

- `docs/explanation/security-model.md`: assets, trust boundaries, threat model, and each control mapped to its code and tests.
- `docs/reference/security-allowances.md`: the register of deliberate allowances (AL), privileges (PR), controls owned outside the library (EX), residual risks (R), and fixed findings (SR). Open findings are never committed; they stay in `local/` until the fix ships. Check any security change against it.
- `.ai/context/security.md`: security policy for agents, including the actions that need explicit confirmation.
- `local/` is gitignored and must never be committed. Full security review reports live in `local/security-review/<date>/`.

## Agent workflows (`.ai/`)

Tool-agnostic instructions any agent can follow:

- `.ai/context/security.md`: security policy for agents.
- `.ai/security-review.md`: security review of the current branch's changes, or a baseline review of the whole repository. It reads the register and its own instructions from the base ref, so a branch can't pre-allow its own findings. In Claude Code, `/security-review` runs this project skill (`.claude/skills/security-review/`), which replaces the built-in `/security-review` here.
- `.ai/security-review-false-positives.md`: rules for verifying review findings.

Tool-specific wrappers (e.g. `.claude/skills/`) should point at these files rather than duplicate them.

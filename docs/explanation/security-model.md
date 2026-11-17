# Security model

What `openstackdb` protects, from whom, and which mechanism is responsible for each guarantee. Each control is mapped to where it is implemented and how it is tested. Everything the library permits on purpose, leaves to callers, or accepts as a risk has an ID in [Security allowances and residual risks](../reference/security-allowances.md).

## The short version

`openstackdb` turns typed Go calls into `SELECT` statements against OpenStack service databases. It holds no credentials of its own, runs no server, and makes no authorization decisions. Everything on this page follows from that:

- **Filter values are data, never SQL.** Values that may come from end users of the calling service only ever reach the database as bound parameters.
- **Asking for less never returns more.** An empty filter matches nothing, soft-deleted and archived rows appear only when asked for, and an invalid option is an error, never a wider query.
- **Read-only by construction.** The query set contains only `SELECT`. Read-only access at the database is the caller's job: the account they connect with.
- **Secrets stay where they are.** No query selects password hashes, credentials, private keys, user data, or driver secrets, and the connection string's password never appears in an error.
- **Callers own everything around the query.** That covers credentials, TLS, pools, deadlines, cell and region selection, and deciding who may see which project.

## Assets

| Asset | Why it matters |
| --- | --- |
| Tenant isolation in results | Callers scope reads by project. A filter that silently widens returns another tenant's resources to whoever the caller serves |
| Integrity of the OpenStack databases | These are the live control-plane databases of every service. Any write path, or an injected statement, could corrupt the cloud |
| The database credential | It usually grants read access to a whole service database, Keystone included |
| Secret columns in those databases | Keystone password hashes and credential blobs, Nova `user_data` and admin passwords in system metadata, Ironic `driver_info` (BMC credentials), Heat templates and parameters, Magnum and Octavia certificate material |

## Trust boundaries

| Input | Trust | Reasoning |
| --- | --- | --- |
| Filter values: IDs, project IDs, hosts, states, statuses, time windows | **Untrusted** | Callers such as exporters and APIs forward them from their own users |
| Option enums: `Deleted`, `ArchiveMode`, booleans | Untrusted, validated | Out-of-range values return an error |
| Column and table names | Trusted | String literals in the package. Nothing exported can supply one |
| The oslo.db connection string | Trusted operator configuration | It already holds the credential. Whoever controls it controls the connection |
| `*sql.DB` / `*sql.Tx` passed by the caller | Trusted | The caller chose its account, isolation level, and TLS |
| Database contents | Trusted as data | Rows are returned, never interpreted as code or SQL |

## Threat model

### End user of a calling service

This is someone who controls filter values, such as a project ID in an exporter's or API's request.

- **SQL injection.** sqlc-generated queries bind every argument. Hand-built filter queries go through `internal/filter.Builder`, which takes column names from string literals and appends values only as `?` placeholders. Table names in archive modes are picked from a fixed pair. Tests: `TestCombinedFilters`, which binds `b' OR 1=1 --` as a value, and the integration test `TestInstanceFilters`, which shows `p1' OR 1=1 --` matches no rows.
- **Widening results.** A non-nil empty slice adds `1 = 0`. An unknown `Deleted` or `ArchiveMode` value is an error. A lifetime window must be both-zero or `start < end`. All clauses are joined with `AND`, and the only `OR` is parenthesized inside the lifetime clause. Tests: `TestNilAndDeleted`, `TestLifetime`, `TestInvalidDeletionPolicy` (nova, cinder, glance), `TestInvalidArchiveMode`, and integration tests `TestInstanceFilters` and `TestInstanceHistoryAndMetadata`.

### Compromised or careless caller

The library does not authorize (AL-1). A caller can query any project, and with `IncludeDeleted` / `IncludeArchived` it can read soft-deleted and shadow-table rows. What a caller cannot get through the library is a write or a secret column, because neither exists in the query set. A caller holding the credential can bypass the library entirely (PR-1).

### Network attacker between library and database

The library does not configure TLS (AL-2). Unless the connection string sets `tls=…`, or the caller's own `*sql.DB` uses TLS, the login handshake and every result cross the network in plaintext.

### Operator misconfiguration

Driver parameters in the connection string's query are passed to go-sql-driver unchanged (AL-3), including ones that weaken the connection, such as `allowAllFiles`, `allowCleartextPasswords`, and `tls=skip-verify`. An account with more than `SELECT` is not detected (EX-1).

## The controls

| Control | Implementation | Tests |
| --- | --- | --- |
| Values as parameters only | `internal/filter/filter.go` (`Strings`, `Lifetime`); sqlc in `queries.sql` | `internal/filter/filter_test.go`, `tests/filters_test.go` |
| Trusted identifiers only | Column and table literals in `*/db/**/filters.go`, `nova/db/main/metadata.go` | Code review; `.ai/security-review.md` checks every change |
| Empty filter matches nothing | `Builder.Strings` | `TestCombinedFilters`, `TestInstanceFilters`, `TestInstanceHistoryAndMetadata` |
| Soft-deleted excluded by default; unknown policy rejected | `Builder.Deleted`, zero value `ExcludeDeleted` (`filters.go`) | `TestNilAndDeleted`, `TestInvalidDeletionPolicy` |
| Archive access explicit; unknown mode rejected | `InstanceGetAllByFilters` switch (`nova/db/main/filters.go`) | `TestInvalidArchiveMode`, `TestInstanceHistoryAndMetadata` |
| Lifetime window both-or-neither, ordered | `Builder.Lifetime` | `TestLifetime` |
| Read-only query set | `queries.sql` files and hand-written `SELECT`s only | CI `sqlc generate` + `git diff --exit-code`; security review |
| No secret columns | Explicit projections, no `SELECT *` | Security review |
| Credential not echoed | `ParseOsloDBConnectionString` drops `url.Parse` errors and echoes only the scheme | `TestMalformedURLDoesNotExposeCredentials`, `TestConnectInvalidURLs` |
| Bounded pool by default | `ConnectContext`: 5 open, 2 idle, negative values rejected | `TestConnectInvalidOptions` |

## Fail-closed behavior

| Condition | Result |
| --- | --- |
| Non-nil empty filter slice | No rows |
| Unknown `Deleted` / `ArchiveMode` value | Error |
| Half-open or inverted lifetime window | Error |
| Requested archive table missing | Error from the database; no partial substitute |
| More than one `InstanceExtraOptions` | Error |
| Unparseable connection string, wrong scheme, missing host or database | Error without the credential |

## Review history

| Date | Scope | Outcome |
| --- | --- | --- |
| 2026-11-17 | Baseline review of the whole repository, using `.ai/security-review.md` | Report kept out of the repository until any fixes ship |

# Security allowances and residual risks

A register of what `openstackdb` permits on purpose, what it leaves to callers and operators, and the risks it accepts. It is written for security reviewers comparing findings against intended behavior. A finding that matches an entry here is known and deliberate, and the entry says why and what bounds it. A finding that contradicts an entry is a defect.

The reasoning behind each entry is in the [Security model](../explanation/security-model.md).

## Deliberate allowances

Behavior that may look like a weakness in a review but is intended.

| ID | Allowance | Why | What bounds it |
| --- | --- | --- | --- |
| AL-1 | **No authorization.** Any caller can read any project, and soft-deleted or archived rows on request | The library is a query layer. Keystone policy and per-request identity belong to the calling service | Filters are honored exactly and never widen. Only `SELECT`s, and no secret columns |
| AL-2 | **No TLS unless asked for.** The DSN carries only what the connection string says | TLS material and policy are site configuration | `tls=true` / `tls=<name>` in the connection string, or a caller-built `*sql.DB` |
| AL-3 | **Connection-string driver parameters pass through unchanged**, including `allowAllFiles`, `allowCleartextPasswords`, `tls=skip-verify`, `multiStatements`, and unknown keys (sent to the server as `SET key = value`) | The connection string is trusted operator configuration and already holds the credential | None are set by default. The library adds only `parseTime=true` |
| AL-4 | **Unbounded result sets and `IN` lists.** No `LIMIT`, no cap on filter length | Callers such as exporters need complete listings. Denial of service is out of scope | Bounded pool (5 open by default). The caller's context deadline cancels the query |
| AL-5 | **Archive union without deduplication.** `IncludeArchived` returns primary and shadow rows, each with an `Archived` flag | Faithful to the data. Merging is the caller's decision | Archive access is never the default. Use `WithTx` for a consistent snapshot |
| AL-6 | **Raw JSON is returned uninterpreted** (Nova `instance_extra.flavor`) | The library does not parse versioned Nova objects | Flavor data holds no secrets. Callers must treat it as untrusted structured data |

## Privileges the library holds

| ID | Holder | Holds | Note |
| --- | --- | --- | --- |
| PR-1 | The process using the library | Whatever database account the connection string or `*sql.DB` names | The library runs only `SELECT`s, but it cannot narrow the account. A compromised caller holding the credential bypasses every control on this page |

## Owned outside the library

Controls the library depends on but does not implement. Each needs an owner at the site.

| ID | Control | Why it matters | Library's part |
| --- | --- | --- | --- |
| EX-1 | **A `SELECT`-only database account**, ideally limited to the tables the caller uses and excluding secret-bearing tables | It is the only real read-only boundary, and it limits what a leaked credential exposes | README recommends it. Not checked at connect time |
| EX-2 | **Credential storage and rotation** | The connection string holds the password | Never logs or echoes it. Callers own the pool and must close it |
| EX-3 | **TLS to the database** | Results include tenant IDs, names, and hosts | Passes `tls=` through (AL-2) |
| EX-4 | **Authorization of the caller's own users** | AL-1 | Exact filter semantics, so a correct caller-side scope is enforced exactly |
| EX-5 | **Cell and region selection** | Querying the wrong cell database returns another region's tenants | None. The caller picks the database |
| EX-6 | **Database timestamps in UTC** | Lifetime windows assume UTC | Converts inputs to UTC |

## Residual risks

Accepted, with what would retire each.

| ID | Risk | Bounded by | Retired by |
| --- | --- | --- | --- |
| R-1 | **Read-only is a property of the query set, not of the connection.** A future change, a hand edit to generated code, or a caller's own use of the `DBTX` could write | CI fails on generated-code drift. The security review flags any non-`SELECT`. EX-1 | Opening sessions with `SET SESSION TRANSACTION READ ONLY` in `ConnectContext` |
| R-2 | **The secret-column guarantee depends on review.** Nothing mechanical stops a new query from selecting `password` or `driver_info` | Explicit projections, no `SELECT *`, and the security review's sensitive-column check | A CI check against a denylist of secret columns per service |
| R-3 | **The database name is URL-decoded into the DSN**, so `%3F` and `%2F` in it become DSN syntax | The connection string is trusted (AL-3), and someone who can edit it can add parameters directly | Escaping or rejecting `?`, `/`, and `&` in the database name |

## Fixed findings

Findings are listed here only once their fix has shipped. Open findings stay out of this
public repository: the full report lives in `local/security-review/<date>/` (gitignored),
and a finding in upstream code is reported privately (to VEXXHOST, or through a draft
repository security advisory) before it is disclosed.

| ID | Severity | Finding | Fixed in |
| --- | --- | --- | --- |
| — | — | None yet | — |

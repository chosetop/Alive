# Alive Backend

生产拓扑、配置、日志、备份恢复和发布 smoke test 见 [`docs/deployment.md`](../docs/deployment.md)。

HTTP API for Alive, a personal record site. Go + Gin + PostgreSQL.

This stage contains infrastructure only: configuration, database pool,
middleware, error handling, response envelope and health endpoints. There is no
business feature yet, by design.

## Requirements

| Tool | Version | Needed for |
|---|---|---|
| Go | 1.25+ | building and running |
| PostgreSQL | 15+ | the database |
| golang-migrate | any | applying migrations |
| sqlc | 1.27+ | generating query code (no effect yet) |

```bash
brew install golang-migrate sqlc
```

`go install` is the alternative if Homebrew is not in use.

### Module proxy

`proxy.golang.org` was unreachable from the machine this was set up on, so the
`Makefile` defaults `GOPROXY` to `https://goproxy.cn,direct`. Override it when
the default proxy works:

```bash
make run GOPROXY=https://proxy.golang.org,direct
```

## Setup

### 1. Configuration

```bash
cp .env.example .env
```

Adjust `DATABASE_URL` if the database is not on `127.0.0.1:5432`.

The process reads the environment, not the file. Load it into the shell:

```bash
set -a; source .env; set +a
```

`make` targets load `.env` on their own, so the manual `source` is only needed
when running `go run` directly.

#### CORS differs between environments

admin is served from `/admin` on the site's own domain, so in production
frontend, admin and the API share one origin and no CORS exchange happens.
Production therefore sets an empty list:

```bash
APP_ENV=production
CORS_ALLOWED_ORIGINS=
```

Development is the opposite case: the Nuxt (3000) and Vite (5173) dev servers
are separate origins from the API on 8080, so origins are required and startup
fails without them.

Setting the variable to empty is not the same as leaving it out. Empty means
"allow nothing"; absent restores the development default. Every other variable
treats empty as absent, which would make the production setting impossible to
express.

### 2. PostgreSQL

Two options. Both give the same result; pick whichever fits the machine.

**Option A: Homebrew, runs as a background service**

```bash
brew install postgresql@16
brew services start postgresql@16

createuser -s alive
createdb -O alive alive
psql -d alive -c "ALTER USER alive WITH PASSWORD 'alive';"
```

**Option B: Docker, one throwaway container**

No Dockerfile or compose file is committed in this stage; this is a single
command run by hand.

```bash
docker run -d --name alive-postgres \
  -e POSTGRES_USER=alive \
  -e POSTGRES_PASSWORD=alive \
  -e POSTGRES_DB=alive \
  -p 5432:5432 \
  -v alive-pgdata:/var/lib/postgresql/data \
  postgres:16-alpine
```

The named volume keeps data across container restarts. Without it, removing the
container discards the database.

Confirm it answers:

```bash
psql "postgres://alive:alive@127.0.0.1:5432/alive?sslmode=disable" -c "select version();"
```

### 3. Migrations

```bash
make migrate-up
make migrate-version   # expect: 2
```

Migration `000001` creates no table. It installs the `citext` extension and the
`set_updated_at()` trigger function, and it gives the chain a first link so
`schema_migrations` exists.

Migration `000002` creates `users` and `sessions`.

### 4. Create the owner account

```bash
make user-create name=owner
```

The password is prompted for, twice, with echo disabled. It is never accepted as
a flag: `argv` is written to shell history and is visible in `ps` to every user
on the machine, and a password that has reached either is no longer a secret.
Passing `--password` is refused with that explanation rather than ignored.

For a script, `--password-stdin` reads it from a pipe:

```bash
printf '%s' "$PASSWORD" | go run ./cmd/cli user:create --username owner --password-stdin
```

That is opt-in by flag, never a fallback. Silently accepting piped input would
mean a mistyped command creates an account whose password came from whatever
happened to be on stdin.

### 5. Run

```bash
make config-check   # validate configuration without connecting
make run
```

`config-check` is worth running first: it reports what the process parsed from
the environment and fails on a bad value without touching the database.

Expected startup output:

```
level=INFO msg="connected to database" max_open_conns=25
level=INFO msg="http server listening" addr=127.0.0.1:8080 env=development
```

## Verifying /health

Two endpoints, answering two different questions.

**Liveness. Never touches the database.**

```bash
curl -i http://127.0.0.1:8080/health
```

```
HTTP/1.1 200 OK
X-Request-ID: 4f3c...
Content-Type: application/json; charset=utf-8

{"data":{"status":"ok"}}
```

**Readiness. Pings the database.**

```bash
curl -s http://127.0.0.1:8080/health/ready | jq
```

```json
{
  "data": {
    "status": "ok",
    "database": { "status": "ok", "latency": "412.5µs" },
    "pool": { "total_conns": 5, "idle_conns": 5, "acquired_conns": 0, "max_conns": 25 }
  }
}
```

Stop the database and call it again to see the failure path:

```bash
docker stop alive-postgres    # or: brew services stop postgresql@16
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/health/ready
# 503
```

`/health` still returns 200 while the database is down. That is the intended
behaviour: the process is alive, it just cannot serve data. A liveness probe
that fails on a database blip makes an orchestrator restart a healthy process,
which does not bring the database back.

**Error envelope**

```bash
curl -s http://127.0.0.1:8080/nope | jq
```

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "no route matches this path",
    "request_id": "4f3c..."
  }
}
```

## Verifying auth

Three endpoints. The session lives in the database; the browser holds only a
random token in a `HttpOnly` cookie.

```
POST /api/v1/auth/login    public, rate limited
POST /api/v1/auth/logout   public, idempotent
GET  /api/v1/me            requires a session
```

**1. Unauthenticated access is refused**

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/api/v1/me
# 401
```

**2. A wrong password and an unknown user give the same answer**

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"owner","password":"wrong"}'

curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"nobody","password":"wrong"}'
```

Both return 401 `INVALID_CREDENTIALS` with identical bodies apart from the
request id. Distinguishing them would turn the endpoint into a way to discover
which usernames exist. The service also hashes a dummy password when the account
is absent, so the two paths take the same time.

**3. Login sets the cookie**

```bash
curl -i -c /tmp/alive-cookies.txt -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"owner","password":"<real>"}'
```

```
HTTP/1.1 200 OK
Set-Cookie: alive_session=...; Path=/api/v1; Max-Age=604800; HttpOnly; SameSite=Strict
{"data":{"id":1,"username":"owner","role":"owner","display_name":""}}
```

The token is in the cookie only, never in the body. `Secure` is absent because
development runs on http://localhost; production refuses to start without it.

**4. The cookie authenticates**

```bash
curl -s -b /tmp/alive-cookies.txt http://127.0.0.1:8080/api/v1/me | jq
```

**5. The database holds no replayable credential**

```bash
psql -d alive -c "SELECT id, user_id, length(token_hash) AS bytes FROM sessions;"
# bytes = 32, a SHA-256 digest of the cookie value
psql -d alive -c "SELECT username, left(password_hash, 32) FROM users;"
# $argon2id$v=19$m=65536,t=3,p=2$...
```

A stolen `sessions` row cannot be turned back into a cookie: the column holds the
digest, and the token itself was never stored.

**6. Logout is immediate and idempotent**

```bash
curl -s -o /dev/null -w '%{http_code}\n' -b /tmp/alive-cookies.txt \
  -X POST http://127.0.0.1:8080/api/v1/auth/logout    # 204
psql -d alive -c "SELECT count(*) FROM sessions;"     # 0
curl -s -o /dev/null -w '%{http_code}\n' -b /tmp/alive-cookies.txt \
  http://127.0.0.1:8080/api/v1/me                     # 401
```

Calling logout again also returns 204. The client's goal is to end up logged out,
and when it already is, that goal is met.

**7. Login is rate limited**

```bash
for i in $(seq 1 7); do
  printf '%d: ' "$i"
  curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' -d '{"username":"owner","password":"wrong"}'
done
# 1..5: 401
# 6..7: 429   with Retry-After
```

Five attempts per minute per address, counted in this process's memory. The
refusal happens before the handler, so a limited request costs about 50 µs
instead of the ~200 ms an Argon2id hash takes. That matters: without it, one HTTP
request buys 200 ms of the server's CPU.

Set `RATE_LIMIT_ENABLED=false` to turn it off for a script that logs in in a
loop. Production refuses to start with it off.

**Behind a reverse proxy, configure `TRUSTED_PROXIES` first.** The router trusts
only the configured proxy IPs/CIDRs, so `c.ClientIP()` uses `X-Forwarded-For` only
when the socket peer is one of those proxies. Direct local development trusts no
proxy headers. An incomplete or overly broad production list is a deployment
configuration error; see `docs/deployment.md`.

**8. Expired sessions are refused and can be swept**

```bash
psql -d alive -c "UPDATE sessions SET expires_at = now() - interval '1 hour';"
curl -s -o /dev/null -w '%{http_code}\n' -b /tmp/alive-cookies.txt \
  http://127.0.0.1:8080/api/v1/me       # 401
make session-prune                       # removed 1 expired session(s)
```

`session:prune` reclaims rows. It closes no hole: an expired session is already
refused at authentication.

A session is extended on use once it is past halfway through its lifetime, and
the response then carries a fresh `Set-Cookie` with the same token and a new
`Max-Age`. Renewing on every request would turn each read into a write.
The session also has a 30-day absolute maximum age (`SESSION_ABSOLUTE_LIFETIME`);
sliding renewal never crosses that boundary.

## Layout

```
cmd/
  server/          HTTP entry point: load config, assemble, serve
  cli/             operator commands that must not be reachable over HTTP
internal/
  config/          environment parsing and validation
  apperr/          the one application error type and its HTTP mapping
  httpx/           response envelope, request id context helpers
  postgres/        connection pool and its health check
    sqlcgen/       generated from sql/queries, committed to git
  middleware/      request id, logger, recovery, CORS, login rate limit
  router/          every route in the API, in one file
  health/          health endpoints
  auth/            authentication domain: no HTTP, no Gin
  authhttp/        the HTTP adapter for auth, the only place that imports Gin
migrations/        golang-migrate SQL pairs
sql/queries/       sqlc input
```

`auth` and `authhttp` are two packages on purpose. Go dependencies are per
package, not per file: a `handler.go` inside `auth` importing Gin would make the
whole domain package depend on Gin, so `cmd/cli` would link a web framework to
create a user. Splitting them puts the boundary under the compiler:

```bash
go list -deps ./cmd/cli       | grep -c gin-gonic   # 0
go list -deps ./internal/auth | grep -c gin-gonic   # 0
```

`health` keeps its handler in-package because nothing but HTTP uses it. `auth` has
another caller, the CLI.

Remaining business domains are absent until the stage that implements them. Each
will be one directory holding its handler, service, repository and models
together:

| Package | Purpose |
|---|---|
| `entry/` | the one content entity, `type` telling a journal from a book |
| `taxonomy/` | categories and tags |
| `media/` | media records and upload credentials |
| `storage/` | object storage behind an interface; Aliyun OSS is the implementation |

## Tests

```bash
make test               # everything that needs no database
make test-db-create     # once: create and migrate alive_test
make test-integration   # everything, including the repository tests
```

`make test` passes without a database. The repository tests skip themselves when
`TEST_DATABASE_URL` is unset, so a green run there does **not** mean the storage
layer was exercised. `make test-integration` is what runs them.

**Why a separate database.** The repository tests call `TRUNCATE users CASCADE`,
and `CASCADE` follows every foreign key pointing at `users`. Aimed at the
development database that deletes real content, and it does so silently: the
tests still pass, so nothing reports the loss.

Two things prevent it. The Makefile points `TEST_DATABASE_URL` at `alive_test`,
and the tests themselves refuse any database whose name does not end in `_test`:

```
refusing to run against database "alive": these tests TRUNCATE users CASCADE,
which would delete real data.
```

That check fails the run rather than skipping it. Skipping would let a wrong DSN
pass as a green test run, which is the failure this exists to catch. The guard
lives in the test rather than only in the Makefile because the Makefile only
governs one command — exporting `TEST_DATABASE_URL` by hand bypasses it, and that
is exactly when a mistake is most likely.

## Conventions

- No global database handle and no global config. Both are built in `main` and
  passed explicitly. What is not passed to a component cannot be used by it.
- `context.Context` is threaded from the HTTP request down. Timeouts and
  cancellation therefore work end to end.
- Code below the HTTP boundary does not know HTTP exists. A service returns
  `apperr.NotFound(...)`; the HTTP layer maps it to 404.
- An error's `cause` reaches the log, never the response body. Driver errors
  name tables and columns.
- Every response body has exactly one top-level key: `data` or `error`.
- Middleware carries no business logic.
- Secrets are never printed and never taken from `argv`. `config:check` reports
  the DSN's length, not the DSN; `user:create` prompts for a password rather than
  accepting a flag.
- Interfaces are declared where they are consumed, not beside the type that
  satisfies them. `auth.Store` lives in `service.go`, with a
  `var _ Store = (*Repository)(nil)` to keep the compiler checking.

## Make targets

```bash
make help
```

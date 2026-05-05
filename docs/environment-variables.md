# Environment variables

Sarga reads its config from `SARGA_<SECTION>_<KEY>` environment variables. In Docker, set them in `.env`. On bare metal, use a systemd `EnvironmentFile=` or export them directly. A `config.yaml` is also supported (`./sarga serve --config /path/to/config.yaml`); env vars override values found in YAML.

All variables are optional — defaults work for the standard Docker install.

---

## Server

| Variable | Default | Description |
|---|---|---|
| `SARGA_SERVER_ADDR` | `:8080` | Listen address. Use `127.0.0.1:8080` to bind to loopback only. |

## Logging

| Variable | Default | Description |
|---|---|---|
| `SARGA_LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `SARGA_LOG_FORMAT` | `text` | One of `text` (human-readable, dev) or `json` (structured, prod). |

## Database

| Variable | Default | Description |
|---|---|---|
| `SARGA_DATABASE_HOST` | `postgres` | Postgres hostname. The default matches the service name in our `compose.yaml`. Set to `localhost` for bare-metal installs, or to your external Postgres host. |
| `SARGA_DATABASE_PORT` | `5432` | Postgres port. |
| `SARGA_DATABASE_USER` | `sarga` | Postgres role name. |
| `SARGA_DATABASE_NAME` | `sarga` | Postgres database name. |
| `SARGA_DATABASE_PASSWORD` | `sarga` | Postgres role password. **Change before exposing publicly.** |
| `SARGA_DATABASE_SSLMODE` | `disable` | One of `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`. Use `require` or stronger when reaching Postgres over a network. |
| `SARGA_DATABASE_MAXCONNS` | `0` | Max pool connections. `0` means auto (`2 × GOMAXPROCS`). |
| `SARGA_DATABASE_AUTOMIGRATE` | `true` | Apply pending schema migrations on `sarga serve` startup. Set to `false` if you run multiple sarga instances and prefer to apply migrations explicitly via `sarga migrate up` from one of them. |

## Auth

| Variable | Default | Description |
|---|---|---|
| `SARGA_AUTH_JWT_SECRET` | _(auto-generated)_ | JWT signing secret. Leave unset and sarga generates a 64-byte secret on first boot, persisted at `/data/secrets/jwt.key` (mode `0600`, never logged, never in DB backups). Set explicitly only when injecting from an external secrets manager (Docker secrets, Vault, AWS KMS, etc.) — must be at least 32 bytes. |

---

## System env vars sarga honours

These aren't `SARGA_*` knobs — they're standard POSIX/Go env vars the runtime picks up automatically when set in the container.

| Variable | Effect |
|---|---|
| `TZ` | Sets the container timezone (e.g. `America/New_York`, `Europe/London`, `Asia/Kolkata`). Affects log timestamp display. Default is UTC. Per-user timezones for scrobbles / year-in-music are stored on the user record and unaffected by this. |

## Naming convention

Env var names map directly to nested config keys:

```
SARGA_<SECTION>_<KEY>          →  section.key
SARGA_<SECTION>_<SUB>_<KEY>    →  section.sub.key
```

Examples:
- `SARGA_LOG_LEVEL` → `log.level`
- `SARGA_DATABASE_AUTOMIGRATE` → `database.automigrate`
- `SARGA_AUTH_JWT_SECRET` → `auth.jwt.secret`

Underscores in env names become dots in the config path — so the binary's struct fields can never themselves contain underscores. (That's why the field is `automigrate`, not `auto_migrate`.)

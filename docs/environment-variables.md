# Environment variables

Sarga reads its config from `SARGA_<SECTION>_<KEY>` environment variables. In Docker, set them in `.env`. On bare metal, use a systemd `EnvironmentFile=` or export them directly. A `config.yaml` is also supported (`./sarga serve --config /path/to/config.yaml`); env vars override values found in YAML.

All variables are optional — defaults work for the standard Docker install and for `./sarga serve` on a developer machine.

## Naming convention

Env var names map directly to nested config keys:

```
SARGA_<SECTION>_<KEY>          →  section.key
SARGA_<SECTION>_<SUB>_<KEY>    →  section.sub.key
```

The algorithm: strip the `SARGA_` prefix, lowercase, replace each `_` with `.`. Examples:

- `SARGA_LOG_LEVEL` → `log.level`
- `SARGA_DATABASE_AUTOMIGRATE` → `database.automigrate`
- `SARGA_SECRETS_JWT` → `secrets.jwt`

Because underscores in env names become dots in the config path, struct field names can never contain underscores themselves — that's why the field is `automigrate`, not `auto_migrate`.

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
| `SARGA_DATABASE_PATH` | _(empty — defaults to `<data-dir>/sarga.db`)_ | Path to the SQLite database file. Leave unset to use the default location inside the data directory. Set explicitly only when you need the database on a separate volume or path. |
| `SARGA_DATABASE_AUTOMIGRATE` | `true` | Apply pending schema migrations on `sarga serve` startup. Set to `false` to apply migrations explicitly via `sarga migrate up`. |

## Secrets

Server-wide cryptographic secrets. Both auto-generate on first boot if unset; set via env when injecting from Docker secrets, Vault, k8s secrets, AWS KMS, etc.

| Variable | Default | Description |
|---|---|---|
| `SARGA_SECRETS_JWT` | _(auto-generated)_ | JWT signing secret. Leave unset and sarga generates a 64-byte secret on first boot, persisted at `<data-dir>/secrets/jwt.key` (mode `0600`, never logged, never in DB backups). Must be at least 32 bytes when set. |
| `SARGA_SECRETS_MASTER` | _(auto-generated)_ | 32-byte master key that encrypts all other secrets stored in the database (provider API keys, OIDC client secrets, etc.) via HKDF-derived per-purpose subkeys. Encoded as hex (64 chars) or base64 (44 chars padded, or 43 unpadded). When set, no file is read or written. File fallback: `<data-dir>/secrets/master.key` (mode `0600`, 32 raw bytes, auto-generated). |

**Master key rotation is destructive:** changing it (env or file) invalidates every value encrypted under the old key. Export decrypted secrets first, replace the key, then re-enter them via the admin UI.

## Data directory

| Variable | Default | Description |
|---|---|---|
| `SARGA_DATA_DIR` | _(platform-dependent, see below)_ | Where Sarga keeps its working files: `sarga.db` (the database), `secrets/` (auto-generated keys), `certs/` (TLS material), `cache/`, `tmp/`. The path is resolved to absolute on startup; relative paths are relative to the process working directory.<br><br>**Default by platform:**<br>• **Container (Docker/Podman):** `/data` — auto-detected at startup, no env var needed. Pair with the `./data:/data` volume in `deploy/compose.yaml`.<br>• **macOS:** `~/Library/Application Support/sarga`.<br>• **Linux / BSD:** `$XDG_DATA_HOME/sarga` if set, else `~/.local/share/sarga`.<br>• **Windows:** `%LOCALAPPDATA%\sarga` (falls back to `%APPDATA%\sarga`).<br><br>For systemd, override to `/var/lib/sarga` and pair with `StateDirectory=sarga`. |

---

## Metadata providers

Metadata provider API keys are **not env-configurable**. They're entered at runtime via the admin UI and stored encrypted in the `server_settings` database table (encrypted under the master key, see above).

Rationale: provider keys are rotated independently of deploys, often added after the operator discovers a provider in the UI, and don't belong in the same tier as bind-address or log-level config. MusicBrainz, TheAudioDB (free tier), Cover Art Archive, and LRCLib require no keys at all.

| Provider | Get a key |
|---|---|
| Last.fm | [last.fm/api](https://www.last.fm/api/account/create) — artist bios, genre tags |
| Fanart.tv | [fanart.tv](https://fanart.tv/get-an-api-key/) — HD artist logos, clearart, backgrounds |
| AcoustID | [acoustid.org](https://acoustid.org/login) — fingerprint submission (lookups work key-less) |
| Discogs | [discogs.com/settings/developers](https://www.discogs.com/settings/developers) — metadata + cover art (strong for niche releases) |

## System env vars sarga honours

These aren't `SARGA_*` knobs — they're standard POSIX/Go env vars the runtime picks up automatically when set in the container.

| Variable | Effect |
|---|---|
| `TZ` | Sets the container timezone (e.g. `America/New_York`, `Europe/London`, `Asia/Kolkata`). Affects log timestamp display. Default is UTC. Per-user timezones for scrobbles / year-in-music are stored on the user record and unaffected by this. |

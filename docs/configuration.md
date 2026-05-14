# Configuring Sarga

This page explains **how** you configure Sarga — the layers, the precedence rules, and the differences between Docker and bare-metal installs. For the full list of every knob you can set, see [`environment-variables.md`](environment-variables.md).

---

## The three tiers of state

Sarga splits configuration into three tiers. They have different homes and different rules:

| Tier | What lives here | Mutable while running? | How you change it |
|---|---|---|---|
| **Secrets** | Cryptographic keys (`jwt.key`, future `master.key`) | No — restart to rotate | Auto-generated files in `<data-dir>/secrets/`. Don't edit by hand. |
| **Config** | Server tunables: listen address, DB password, log level, data dir | No — restart to apply | Defaults → `config.yaml` → `SARGA_*` env vars |
| **Settings** | Operator preferences: registration on/off, SMTP host, etc. | Yes — live | SQLite `server_settings` table, edited via the admin UI (not implemented yet) |

This page is about **the Config tier** — the server-level knobs you set before launching Sarga.

---

## Three channels, fixed precedence

The Config tier reads from exactly three places, in this order:

```
built-in defaults  <  config.yaml  <  SARGA_* env vars
   (lowest priority)                       (highest)
```

That's the whole story. There are **no CLI flags for config values** — only `--config` to point at the YAML file itself. There are no other layers.

| Channel | Where | When it wins |
|---|---|---|
| Built-in defaults | Baked into the Sarga binary | When neither YAML nor env sets the value |
| `config.yaml` | Path passed to `--config` (default: `./config.yaml`) | When env doesn't set the value |
| `SARGA_*` env vars | Process environment | Always — env wins over YAML |

Naming convention: `SARGA_<SECTION>_<KEY>` maps to `section.key`. Underscores become dots. So `SARGA_LOG_LEVEL` sets `log.level`, and `SARGA_SECRETS_JWT` sets `secrets.jwt`.

---

## Docker vs bare metal: same channels, different delivery

The three channels above are identical on both platforms. What changes is *how env vars reach the process*:

| Channel | Docker (compose) | Bare metal |
|---|---|---|
| Defaults | Baked into the image | Baked into the binary |
| `config.yaml` | Mount it: `volumes: ["./config.yaml:/etc/sarga/config.yaml:ro"]` and pass `command: ["serve", "--config", "/etc/sarga/config.yaml"]`. Most Docker users skip this and use env-only. | `./sarga serve` looks for `./config.yaml` by default. Pass `--config /etc/sarga/config.yaml` to override. |
| `SARGA_*` env vars | `env_file: .env` (the bundled `compose.yaml` does this), **or** an `environment:` block inline, **or** `docker run -e SARGA_FOO=bar`. | Shell `export SARGA_FOO=bar`, **or** systemd `EnvironmentFile=/etc/sarga/sarga.env`, **or** `Environment=SARGA_FOO=bar` lines in the unit file. |

Whichever delivery mechanism you use, **inside the process the env vars look identical**. Sarga can't tell the difference between `docker run -e` and `systemctl set-environment`.

---

## Recommended setups

### Docker (most users)

1. Copy `.env.example` to `.env` next to your `compose.yaml`.
2. `docker compose up -d`.

You don't need a `config.yaml` for a typical install. The bundled defaults plus your `.env` cover everything.

### Bare-metal / systemd

The clean split: durable non-secret settings in YAML, secrets in env.

1. **Durable settings** — drop a `config.yaml` at `/etc/sarga/config.yaml`:
   ```yaml
   server:
     addr: ":8080"
   data:
     dir: /var/lib/sarga
   log:
     level: info
     format: json
   ```
   Commit this to your ops repo. It contains nothing sensitive.

2. **Secrets** — for a standard install there are none to inject; Sarga auto-generates `jwt.key` and `master.key` on first boot. If you're integrating with an external secrets manager, put overrides in a systemd `EnvironmentFile` (mode `0600`):
   ```ini
   # /etc/sarga/sarga.env  (0600, chown sarga:sarga)
   SARGA_SECRETS_JWT=<inject from vault/KMS if needed>
   SARGA_SECRETS_MASTER=<hex or base64, 32 bytes>
   ```

3. **Wire it up in the unit:**
   ```ini
   # /etc/systemd/system/sarga.service
   [Service]
   User=sarga
   Group=sarga
   StateDirectory=sarga                       # creates /var/lib/sarga (0700)
   EnvironmentFile=/etc/sarga/sarga.env
   ExecStart=/usr/local/bin/sarga serve --config /etc/sarga/config.yaml
   Restart=on-failure
   ```

This gives you:
- Non-secrets you can `cat /etc/sarga/config.yaml` for, share screenshots of, commit, etc.
- Secrets that live in one `0600` file you can audit.
- A standard FHS layout (`/etc/`, `/var/lib/`, `/usr/local/bin/`).

### Bare-metal, hand-launched (development / quick test)

```bash
./sarga serve
```

Defaults handle everything else. `./data` appears next to the binary on first run.

---

## Quick rules of thumb

- **Secrets** (passwords, tokens) → env vars, never YAML.
- **Non-secrets** (addresses, log level, data dir) → YAML if you have many; env if you have few.
- **Per-host overrides** of a shared YAML → env.
- **Auto-generated keys** (`jwt.key`, future `master.key`) → leave them alone. Don't put them in env or YAML unless you're integrating with an external secrets manager.
- **Restart Sarga to apply any Config change.** No hot-reload for the Config tier.

---

## Common confusions

**"Should I use env or YAML?"**
Either works. Most Docker users go env-only. Most systemd users use both: YAML for the boring stuff, env for secrets.

**"Why doesn't `--data-dir` work as a flag?"**
There's no flag for individual config values, by design. Set `SARGA_DATA_DIR` (env) or `data.dir` (YAML) instead. The only flag is `--config`, which points at the YAML file itself.

**"Where does `SARGA_*` get parsed?"**
Inside the Sarga process, by koanf. The shell, Docker, or systemd just delivers the env var into the process; Sarga reads it from there. No external tool is involved.

**"Can I have multiple `config.yaml` files?"**
No. One file per Sarga instance, pointed at by `--config`. If you need environment-specific overrides, set them via env vars on top of one shared YAML.

**"What about `server_settings` in the database?"**
That's the third tier (mutable settings, edited via admin UI). It's separate from the Config tier and not implemented yet. When it lands, it'll cover things you'd want to change without restarting — registration toggle, SMTP host, etc.

---

## See also

- [`environment-variables.md`](environment-variables.md) — every knob, with defaults and descriptions
- [`backup-and-portability.md`](backup-and-portability.md) — what to back up
- [`security.md`](security.md) — security posture and recommendations

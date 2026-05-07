# Security in Sarga

Sarga is built to live on the open internet. Self-hosted music servers are usually exposed (you want to stream from your phone on cellular), and they hold a lot about you: your music taste, when you listen, who has access, and — eventually — third-party service tokens like Last.fm. So security isn't a "Phase 9 hardening pass" for us. It's a day-one default.

This document covers three things:

1. **What's already in the codebase** — the protections that ship today.
2. **What's planned** — what we're adding next, and why.
3. **What you, the operator, should do** — practical settings that make your install meaningfully safer.

Last reviewed: 2026-05-06.

---

## What we've already built in

### Authentication & passwords

| Protection | What it means | Where it lives |
|---|---|---|
| **Argon2id password hashing** with OWASP 2024 parameters (m=64 MiB, t=3, p=4) | Even if someone steals your database, cracking a single password takes months on a GPU. The parameters are tuned to OWASP's current recommendation. | `internal/domain/auth/password.go` |
| **Constant-time password compare** | `crypto/subtle.ConstantTimeCompare` — an attacker can't measure how long verification takes to learn anything about the stored hash. | same |
| **Password input bounded at 1024 bytes** | Prevents a malicious request with a 10 MB "password" from pinning the CPU on argon2. | same |
| **Automatic rehash on login** | If we tighten argon2 parameters in a future release, your stored hash gets silently upgraded the next time you log in. No flag day. | `NeedsRehash()` |
| **Refresh tokens are 32 random bytes, stored as SHA-256 hash** | The plaintext only ever lives in your browser's `HttpOnly` cookie. The database holds a hash, so a leak doesn't hand attackers usable tokens. | `internal/domain/auth/refresh.go` |
| **Refresh token rotation** | Each refresh issues a new token and invalidates the previous one (linked via `parent_id`). Reusing a rotated token is detectable. | migration `0001_init.sql` |
| **Username and email forced lowercase at the DB level** | CHECK constraints make `Alice` and `alice` literally the same row — no Unicode-confusable login bypasses. | same migration |
| **JWT signing secret auto-generated on first boot** | `data/secrets/jwt.key` (0600 perms, atomic write). You don't need to think about it; the file appears with 32 cryptographically random bytes the first time Sarga starts. | `internal/domain/auth/jwt.go` |
| **Loose-perm refusal** | If `jwt.key` ends up world- or group-readable (e.g. someone `chmod`d it), Sarga refuses to start rather than load it. | `internal/domain/auth/jwt.go:readSecret` |

### Network defenses

| Protection | What it means |
|---|---|
| **Five timeouts on the HTTP server** | `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, plus a per-request `chi.Timeout`. A slow-loris attacker can't tie up your server with a half-open connection. |
| **Body size cap** | `MaxBytesReader` stops oversized request bodies from filling memory. Mounted on every API route. |
| **Max header bytes (1 MiB)** | Same idea, applied to headers. |
| **Security headers on every response** | `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Resource-Policy: same-origin`. Browsers won't render Sarga in an iframe, won't leak referrer URLs, and won't share resources cross-origin. |
| **Recoverer middleware** | A panic in one request returns a 500 to that one client; it doesn't crash the server. |
| **`RealIP` middleware** | Logs and rate limits use the real client IP when behind a trusted reverse proxy, not the proxy's address. |
| **`RequestID` middleware** | Every request gets a unique ID in logs, so you can trace exactly what happened during an incident without grepping by timestamp. |

### Authorization

| Protection | What it means |
|---|---|
| **`RequireAuth` middleware** | Mounted in front of every protected route. Parses `Authorization: Bearer <jwt>`, validates the signature and expiry, and pins the user ID into the request context. Failed validation = 401 immediately. |
| **`RequireAdmin` middleware** | Composed on top of `RequireAuth` for admin-only endpoints. |
| **DB-level role check** | The `library_access.role` column has a CHECK constraint limiting it to `viewer`/`contributor`/`admin`. Even a SQL injection that tried to grant a bogus role would fail at the database. |

### Data directory layout

| Directory | Mode | Why |
|---|---|---|
| `data/` (root) | 0700 | Owner-only access |
| `data/secrets/` | 0700 | Holds keys that gate everything |
| `data/certs/` | 0700 | TLS material |
| `data/cache/` | 0755 | Non-sensitive; readable for caching tooling |

Directory creation is atomic and idempotent — Sarga never trusts that the parent process set the mode correctly; it checks every boot.

### Build & supply chain

| Protection | What it means |
|---|---|
| **`CGO_ENABLED=0` static binary** | No dynamic linker surprises, no glibc CVEs creeping in via the runtime. |
| **Non-root container (uid 65532)** | If someone gets RCE inside the container, they don't get container-root. |
| **Alpine 3.23 base** | Small attack surface, fast security updates. |

---

## What's coming next

These are tracked in `PLAN.md`. We're calling them out here so you know what to expect.

### Phase 1 (in progress)

| Item | Why |
|---|---|
| **Config redaction in logs** (`Config.LogValue()`) | Stop `auth.jwt.secret` from being printable via `slog.Info("config", "c", cfg)`. Defence against accidental log dumps in support tickets. |
| **OIDC (OpenID Connect) login** | Self-host alongside Authentik / Keycloak / Pocket ID and let your existing identity provider handle MFA, device management, and federated login. Three lines of YAML to wire up. |
| **OIDC identity claim binding** | The `oidc_identities` table already exists; we'll enforce that an issuer + subject pair maps to exactly one user, so you can't be silently re-linked to someone else's account. |

### Phase 3

| Item | Why |
|---|---|
| **Master key + envelope encryption for external tokens** | Last.fm session keys, ListenBrainz tokens, future SMTP passwords, etc. — all encrypted at rest with AES-256-GCM. The master key auto-generates to `data/secrets/master.key` (same pattern as `jwt.key`). |
| **HKDF-derived per-purpose subkeys** | Master key is never used directly. Each consumer (`scrobble-session-key`, `oidc-client-secret`, etc.) gets its own subkey via HKDF-SHA256. A bug in one consumer's nonce handling can't compromise the others. This is the same pattern AWS Encryption SDK and Rails Active Record Encryption use. |
| **Listening event privacy controls** | Per-user toggle for "scrobble out to Last.fm/ListenBrainz" + bulk-delete + full export. |

### Phase 5+

| Item | Why |
|---|---|
| **Rate limiting** | In-process token bucket (`golang.org/x/time/rate`) on auth endpoints first, then expanded. No Redis dependency. |
| **Filesystem path validation for libraries** | `filepath.Clean` + prefix check on every library path; refuse symlinks that escape the configured root; refuse `..` in any user-supplied path. Already partly enforced; will be made systematic. |
| **TLS via autocert** | `golang.org/x/crypto/acme/autocert` so you don't need Caddy/Traefik/nginx to get HTTPS. Three lines of config and Let's Encrypt does the rest. |

### Continuous

| Item | Why |
|---|---|
| **Dependabot** | Weekly Go module + Docker base image updates as PRs. |
| **`govulncheck` in CI** | Fails the build if any imported package has a published Go vulnerability. |
| **`gosec`** in golangci-lint | Catches common Go security smells (hardcoded creds, weak crypto, unsafe SQL) at PR time. |
| **`syft` SBOM + `cosign` keyless signing on releases** | Every published binary and container has a verifiable software bill of materials and a signature you can verify with `cosign verify`. |
| **End-of-phase security review** | At every phase boundary we ask: "what new attack surface did this introduce, and is it hardened?" — not as a checkbox, but as the gate before merging the phase. |

---

## What you, the operator, should do

Sarga gives you secure defaults out of the box, but a few choices are yours to make.

### The five-minute hardening checklist

If you do nothing else, do these:

1. **Put Sarga behind HTTPS.** Either let Sarga handle it directly (autocert, Phase 5) or front it with Caddy (`caddy reverse-proxy` is one line), Traefik, or a tunnel like Tailscale/Cloudflare.
2. **Back up the `data/` directory regularly.** It contains both the database (`sarga.db`) and `secrets/`. Losing `secrets/` will eventually mean unrecoverable data once Phase 3 ships. See [`backup-and-portability.md`](backup-and-portability.md).
3. **Verify the data directory is 0700.** Sarga enforces this on startup, but confirm if you've moved it or restored from backup. The SQLite file inherits the directory's owner — no other process on the host should be able to read it.
4. **Make a regular user for daily use; reserve the admin account for admin tasks.** Same reasoning as `sudo`: an XSS in a hypothetical future plugin can't escalate to admin if you're not logged in as one.

### The thirty-minute hardening checklist

For a more locked-down deployment:

| Action | How | Why |
|---|---|---|
| **Use OIDC for login** (when Phase 1 lands) instead of local passwords | Point Sarga at Authentik / Keycloak / Pocket ID | Centralizes MFA, device management, audit logs |
| **Run Sarga behind a reverse proxy with rate limits** | Caddy `rate_limit`, Traefik plugins, or Cloudflare | Defence in depth before our in-process limiter ships |
| **Restrict who can reach `/api/v1/admin/*`** at the proxy | Allow-list your home IP, or require Tailscale/WireGuard | Belt and braces with `RequireAdmin` |
| **Encrypt your offsite backups** | `age -r age1...` or restic with a passphrase | Backups are exfiltrated more often than running services |
| **Lock down the host** | Unattended-upgrades, firewall (only 80/443 exposed), SSH key auth only | Standard server hygiene; the weakest link is usually the OS, not the app |
| **Separate the Sarga container's network** from other services | `networks:` in compose | Limits blast radius of a hypothetical RCE |

### Things to *not* do

| Don't | Why |
|---|---|
| Bake `data/secrets/`, `.env`, or `config.yaml` into a Docker image | Anyone who pulls the image gets your keys |
| Commit `secrets/` or `.env` to git, even a private repo | Git history is forever; private repos become public via mistakes and account takeovers |
| Paste the contents of `secrets/` or your DB password into a forum / Discord / GitHub issue when asking for help | Same reason. Use the `<redacted>` log markers Sarga produces — those are the safe version |
| Disable HTTPS "just for testing" on a server reachable from the internet | The phone you tested with becomes the device that scraped your library a week later |
| Run Sarga as root | The Docker image already runs as uid 65532; if you build your own, keep it that way |
| Skip the `secrets/` backup because "it's just session keys" | Once Phase 3 lands, this folder gates access to all encrypted columns. Treat it like an SSH private key |

### When something goes wrong

Suspected breach? Do this in order:

1. **Stop Sarga.** `docker compose stop sarga` (or `systemctl stop sarga`).
2. **Snapshot the database and `data/secrets/`** with the same backup commands as a normal backup, but to a separate directory you'll keep for forensics.
3. **Rotate the JWT secret**: delete `data/secrets/jwt.key` and restart. This invalidates all active sessions; everyone gets logged out.
4. **Force password resets** for all users (admin UI, when shipped, or `UPDATE users SET password_changed_at = datetime('now')` and have them re-auth).
5. **Audit `listening_events`** for unfamiliar play history and `library_access` for unexpected grants.
7. **Update Sarga to the latest release** before bringing the service back up — if the breach was via a known CVE, the patched version closes it.

If you find a vulnerability *in Sarga itself*, please report it privately rather than opening a public issue. (Reporting channel will be added to the README before v1.0.)

---

## Threat model in one paragraph

We assume your Sarga instance is reachable from the open internet, that some users will pick weak passwords, that your backups will sometimes leak, and that one day someone will find a SQL injection or RCE before we do. Defences are layered so that no single failure compromises everything: passwords are hashed with argon2id (DB leak doesn't reveal credentials), refresh tokens are hashed (DB leak doesn't grant sessions), session tokens are short-lived (15 min) and rotated, future external-service tokens are encrypted with a master key kept in a separate file (DB leak doesn't reveal Last.fm keys), the master key has a per-purpose subkey via HKDF (one consumer's bug doesn't compromise the others), the secrets folder has 0600 perms (filesystem read by another tenant doesn't grant access), and the build pipeline signs and SBOM-tracks every release (a compromised distribution channel is detectable). You can break any one layer and the others still hold.

We can't defend against: a fully compromised host (root reads everything), a malicious operator (you have admin access by design), or a sufficiently determined nation-state actor with physical access to your server. For that threat model, you want a different product.

---

## Further reading

- [`docs/backup-and-portability.md`](backup-and-portability.md) — what to back up and why
- [`docs/environment-variables.md`](environment-variables.md) — every config knob, with security notes
- `CLAUDE.md` (in the repo root) — internal security posture for contributors
- `PLAN.md` (in the repo root) — the full multi-phase roadmap, including security-related items

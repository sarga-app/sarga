# Backing up Sarga (and moving to a new server)

This guide gets you a working backup in about 5 minutes, and shows you how to restore it on a new machine when you need to.

## TL;DR

You need to back up **two things**:

1. **Your Postgres database** — your users, libraries, listening history, playlists.
2. **Sarga's `secrets/` folder** — the keys that make your saved data readable.

That's it. Music files aren't included (Sarga never modifies them — your existing music backup is fine).

> **Most important rule:** back up #1 and #2 *together*, on the same schedule. Restoring one without the other will break your install.

Pick your install type below and follow the steps.

---

## I run Sarga in Docker

Your data lives in two Docker volumes that were created when you ran `docker compose up`:

- `sarga-pgdata` — the database
- `sarga-data` — Sarga's working files (secrets, certs, cache)

### One-time: a backup script you can run any day

Save this as `backup-sarga.sh` somewhere safe and run it whenever you want a backup (or put it in cron — see below).

```bash
#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-$HOME/sarga-backups}"
DATE=$(date +%F)
mkdir -p "$OUT"

echo "→ Dumping database..."
docker exec sarga-postgres pg_dump -U sarga -Fc sarga > "$OUT/db-$DATE.dump"

echo "→ Archiving secrets..."
docker run --rm \
  -v sarga-data:/src:ro \
  -v "$OUT":/out \
  alpine tar czf "/out/secrets-$DATE.tar.gz" -C /src secrets

echo "→ Copying .env..."
cp .env "$OUT/env-$DATE.backup" 2>/dev/null || echo "  (no .env in current dir, skipping)"

echo "✓ Done. Files in $OUT:"
ls -lh "$OUT"/*-"$DATE".*
```

Run it:

```bash
chmod +x backup-sarga.sh
./backup-sarga.sh
```

You'll get three files dated today:

```
db-2026-05-06.dump        # the database
secrets-2026-05-06.tar.gz # the keys
env-2026-05-06.backup     # your config
```

Copy these three files **together** to wherever you keep backups (external drive, S3, Backblaze, another machine — see the Offsite section below).

### Restoring on a new machine

You're setting up a fresh machine and want your old Sarga back.

```bash
# 1. Install Docker + Sarga the normal way (clone repo or copy compose.yaml).

# 2. Restore your .env file FIRST. The DB password in it must match
#    what you backed up.
cp env-2026-05-06.backup .env

# 3. Bring up just Postgres.
docker compose up -d postgres

# 4. Wait ~5 seconds for it to be ready, then restore the database.
docker exec -i sarga-postgres \
  pg_restore -U sarga -d sarga --clean --if-exists < db-2026-05-06.dump

# 5. Restore secrets into the data volume.
docker run --rm \
  -v sarga-data:/dst \
  -v "$PWD":/in \
  alpine tar xzf "/in/secrets-2026-05-06.tar.gz" -C /dst

# 6. Start Sarga.
docker compose up -d sarga
```

Open the URL in a browser — you should be back exactly where you left off.

---

## I run the Sarga binary directly

Your data lives in:

- A Postgres database somewhere (local, remote, doesn't matter — you set this in your config)
- A `data/` folder created **next to wherever you launched Sarga from**. It's always `./data` relative to the working directory when you ran `./sarga serve`. There is no config option to move it — if you want it elsewhere, run Sarga from that directory (or symlink).

### One-time: a backup script you can run any day

Save this as `backup-sarga.sh` **next to your Sarga binary** (so `./data` resolves correctly):

```bash
#!/usr/bin/env bash
set -euo pipefail

# Run this from the same directory you launch Sarga from.
# That's where the ./data folder lives.

OUT="${1:-$HOME/sarga-backups}"
DATE=$(date +%F)
mkdir -p "$OUT"

echo "→ Dumping database..."
pg_dump -h "${SARGA_DATABASE_HOST:-localhost}" \
        -U "${SARGA_DATABASE_USER:-sarga}" \
        -Fc "${SARGA_DATABASE_NAME:-sarga}" > "$OUT/db-$DATE.dump"

echo "→ Archiving secrets..."
tar czf "$OUT/secrets-$DATE.tar.gz" -C ./data secrets

echo "→ Copying config..."
cp config.yaml "$OUT/config-$DATE.yaml" 2>/dev/null || echo "  (no config.yaml found, skipping)"

echo "✓ Done. Files in $OUT:"
ls -lh "$OUT"/*-"$DATE".*
```

Run it:

```bash
chmod +x backup-sarga.sh
./backup-sarga.sh
```

You'll get three files dated today. Copy them somewhere safe.

### Restoring on a new machine

```bash
# 1. Install Postgres and create the sarga role + database.
sudo -u postgres createuser sarga
sudo -u postgres createdb -O sarga sarga
sudo -u postgres psql -c "ALTER USER sarga WITH PASSWORD '<your-old-password>';"

# 2. Restore the database.
pg_restore -h localhost -U sarga -d sarga --clean --if-exists db-2026-05-06.dump

# 3. Restore secrets.
mkdir -p ./data
tar xzf secrets-2026-05-06.tar.gz -C ./data

# 4. Restore config.
cp config-2026-05-06.yaml ./config.yaml

# 5. Start Sarga.
./sarga serve --config ./config.yaml
```

---

## Moving to a new server (zero-downtime-ish)

Same recipe as restoring, but done while the old server is still up:

1. **On the old server:** stop only Sarga (leave Postgres running): `docker compose stop sarga` (or `systemctl stop sarga`). This pauses writes so your snapshot is consistent.
2. **On the old server:** run your backup script.
3. **Copy the three files** to the new server (`scp`, `rsync`, USB drive, whatever).
4. **On the new server:** follow the restore steps above.
5. **Update your DNS or reverse proxy** to point at the new server.
6. **On the old server:** once you've verified the new one works, shut it down.

You don't need to copy the `cache/` folder — Sarga rebuilds it automatically. You *can* copy `certs/` if you want to avoid Sarga re-fetching its TLS certificate on first boot, but it's optional.

---

## Automating it (cron)

To run the backup every night at 3am and keep the last 14 days:

```bash
# crontab -e
0 3 * * * /path/to/backup-sarga.sh && find ~/sarga-backups -mtime +14 -delete
```

---

## Storing backups offsite

A backup that lives next to the server isn't really a backup. Pick one:

- **Easiest:** copy the daily files to a cloud bucket (`rclone copy ~/sarga-backups remote:sarga`). Works with Backblaze B2, S3, Google Drive, OneDrive, etc.
- **More secure:** encrypt before uploading using [age](https://age-encryption.org):
  ```bash
  age -r age1... < db-2026-05-06.dump > db-2026-05-06.dump.age
  ```
- **Most thorough:** use [restic](https://restic.net) or [borg](https://www.borgbackup.org) — they handle encryption, deduplication, and remote repos in one tool.

Whatever you pick, **test the restore at least once.** An untested backup is a wish, not a backup.

---

## The one warning to take seriously

Sarga's `secrets/` folder is small (a few files of a few hundred bytes each), but it's the most important thing to back up. Here's what each file does and what happens if you lose it:

| File | What it does | If you lose it |
|---|---|---|
| `secrets/jwt.key` | Signs login session tokens | Everyone gets logged out and has to log in again. Annoying, but recoverable. |
| `secrets/master.key` *(coming soon, Phase 3)* | Encrypts saved Last.fm / ListenBrainz keys and other external service tokens | Those saved tokens become **permanently unreadable**. You'll have to re-link those services. |

If you delete these files and restart Sarga, **it will happily generate new ones** — and now your old database refers to keys that no longer exist. Always restore from backup rather than let Sarga regenerate.

Treat `secrets/` like an SSH private key:

- Never commit it to git
- Never bake it into a Docker image
- Never paste it in a forum or chat
- Keep at least one copy that isn't on the same machine as Sarga

---

## What's *not* covered by these backups

| Thing | Why not | What to do |
|---|---|---|
| Your music files | Sarga never modifies them | Use your existing media/NAS backup |
| Reverse proxy config (Caddy / Traefik / nginx) | Lives outside Sarga | Back up alongside, separately |
| Operator notes (passwords, OIDC client secrets) | Not in any one place | Keep a password manager entry titled "Sarga: <hostname>" |

---

## Got stuck?

If a restore doesn't work, the most common issue is a **database password mismatch** between the backup and the new install. Open `.env` (Docker) or `config.yaml` (binary) on the new machine and make sure `SARGA_DATABASE_PASSWORD` is exactly what it was on the old machine.

If you need help, open a discussion at [github.com/sarga-app/sarga/discussions](https://github.com/sarga-app/sarga/discussions) and include:
- Whether you're on Docker or binary
- Which step failed
- The error message (with passwords/keys redacted, please)

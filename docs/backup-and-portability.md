# Backing up Sarga (and moving to a new server)

This guide gets you a working backup in about 5 minutes, and shows you how to restore it on a new machine when you need to.

## TL;DR

You need to back up **one thing**: Sarga's data directory.

It contains everything you can't reconstruct:

- `sarga.db` — your users, libraries, listening history, playlists.
- `secrets/` — the keys that protect your data.

That's it. Music files aren't included (Sarga never modifies them — your existing music backup is fine).

> **Most important rule:** always keep `sarga.db` and `secrets/` together in the same backup. Restoring one without the other will break your install.

Pick your install type below and follow the steps.

---

## I run Sarga in Docker

Your data lives in one Docker volume that was created when you ran `docker compose up`:

- `sarga-data` — the database, secrets, certs, and cache.

### One-time: a backup script you can run any day

Save this as `backup-sarga.sh` somewhere safe and run it whenever you want a backup (or put it in cron — see below).

```bash
#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-$HOME/sarga-backups}"
DATE=$(date +%F)
mkdir -p "$OUT"

echo "→ Stopping sarga (briefly)..."
docker compose stop sarga

echo "→ Archiving data volume..."
docker run --rm \
  -v sarga-data:/src:ro \
  -v "$OUT":/out \
  alpine tar czf "/out/sarga-data-$DATE.tar.gz" -C /src .

echo "→ Starting sarga..."
docker compose start sarga

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

You'll get two files dated today:

```
sarga-data-2026-05-06.tar.gz  # database + secrets + certs
env-2026-05-06.backup         # your config
```

Copy these two files **together** to wherever you keep backups (external drive, S3, Backblaze, another machine — see the Offsite section below).

> **Downtime:** stopping sarga takes a second or two; the volume archive usually completes in under a minute. If you can't afford even that, see the [Online backup](#online-backup-no-downtime) section below.

### Restoring on a new machine

You're setting up a fresh machine and want your old Sarga back.

```bash
# 1. Install Docker + Sarga the normal way (clone repo or copy compose.yaml).

# 2. Restore your .env file FIRST.
cp env-2026-05-06.backup .env

# 3. Restore the data volume.
docker run --rm \
  -v sarga-data:/dst \
  -v "$PWD":/in \
  alpine tar xzf "/in/sarga-data-2026-05-06.tar.gz" -C /dst

# 4. Start Sarga.
docker compose up -d sarga
```

Open the URL in a browser — you should be back exactly where you left off.

### Online backup (no downtime)

If you can't pause sarga, use SQLite's built-in online backup:

```bash
docker exec sarga sqlite3 /data/sarga.db ".backup /data/tmp/sarga-online-backup.db"
docker run --rm \
  -v sarga-data:/src:ro \
  -v "$OUT":/out \
  alpine tar czf "/out/sarga-data-$DATE.tar.gz" \
    -C /src tmp/sarga-online-backup.db secrets
docker exec sarga rm /data/tmp/sarga-online-backup.db
```

`.backup` is WAL-aware and produces a consistent snapshot without stopping the server.

> Requires `sqlite3` in the Sarga container image. This will be wrapped as `sarga backup` in a future release.

---

## I run the Sarga binary directly

Your data lives in one folder controlled by `SARGA_DATA_DIR` (or `data.dir` in `config.yaml`). Defaults to `./data` next to the binary. systemd users typically set this to `/var/lib/sarga`.

### One-time: a backup script you can run any day

Save this as `backup-sarga.sh`. Set `SARGA_DATA_DIR` to match your install if you've moved it:

```bash
#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${SARGA_DATA_DIR:-./data}"
OUT="${1:-$HOME/sarga-backups}"
DATE=$(date +%F)
mkdir -p "$OUT"

echo "→ Stopping sarga..."
systemctl stop sarga 2>/dev/null || true   # adjust to your init system

echo "→ Archiving data dir..."
tar czf "$OUT/sarga-data-$DATE.tar.gz" -C "$(dirname "$DATA_DIR")" "$(basename "$DATA_DIR")"

echo "→ Starting sarga..."
systemctl start sarga 2>/dev/null || true

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

You'll get two files dated today. Copy them somewhere safe.

### Restoring on a new machine

```bash
# 1. Install the Sarga binary.

# 2. Restore the data dir.
tar xzf sarga-data-2026-05-06.tar.gz -C ./   # extracts ./data/

# 3. Restore config.
cp config-2026-05-06.yaml ./config.yaml

# 4. Start Sarga.
./sarga serve --config ./config.yaml
```

---

## Moving to a new server (zero-downtime-ish)

Same recipe as restoring, but done while the old server is still up:

1. **On the old server:** stop Sarga: `docker compose stop sarga` (or `systemctl stop sarga`). This pauses writes so your snapshot is consistent.
2. **On the old server:** run your backup script.
3. **Copy the files** to the new server (`scp`, `rsync`, USB drive, whatever).
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
  age -r age1... < sarga-data-2026-05-06.tar.gz > sarga-data-2026-05-06.tar.gz.age
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
| Operator notes (OIDC client secrets, etc.) | Not in any one place | Keep a password manager entry titled "Sarga: <hostname>" |

---

## Got stuck?

If a restore doesn't work, the most common issue is a **mismatched `secrets/` folder**. Make sure you restored the `secrets/` directory from the same backup archive as `sarga.db` — they must be from the same point in time.

If you need help, open a discussion at [github.com/sarga-app/sarga/discussions](https://github.com/sarga-app/sarga/discussions) and include:
- Whether you're on Docker or binary
- Which step failed
- The error message (with any keys or tokens redacted, please)

# Phase 2 Scanner Benchmark

Recorded numbers for the Phase 2 scanner architecture (folder-unit pipeline, content-hash move detection, scoped GC, in-tx cursor, in-memory live state). This is the Phase 2 acceptance gate per `PLAN.md`.

## Test environment

| | |
|---|---|
| Host | Apple M5, 16 GB RAM, macOS 26.4.1 |
| Container | `sarga:dev` built from this repo's `deploy/Dockerfile`, arm64 alpine 3.23 base, `CGO_ENABLED=0` |
| Bind mounts | `dev/data` (sqlite) and `/Users/rohan/Music/Library/Sorted/FLAC` → `/mnt/music` (read-only library) — Docker Desktop VirtioFS |
| Library | 1424 audio files, 51.05 GB on disk, FLAC-heavy. 429 folders, 413 releases, 200 artists |
| Sarga | commit `50a074d` (Phase 2 scanner code-complete) |

## First scan (cold DB, all 1424 files imported)

```
scan: started     library_id=… run_id=… paths=1 cursor=""
scan: pipeline    folders_with_work=429 prepare_wall=0s total_wall=14.53s
scan: gc          releases=0 release_groups=0 artists=0 elapsed=1ms
scan: completed   total=1424 added=1424 updated=0 unchanged=0 moved=0 removed=0
                  errors=0 walk=160ms pipeline=14.53s missing=3ms gc=1ms
```

| Metric | Value |
|---|---|
| Wall time | **14.69 s** |
| Throughput | 97 files/s, 3.5 GB/s |
| Files imported | 1424 |
| Errors | **0** (vs 27 on pre-fix code — the `track_files.is_preferred` partial-unique constraint that multi-edition folders tripped is now respected) |
| Phase: walk (reconciler) | 160 ms |
| Phase: pipeline (hash + tags + folder tx × 429) | 14.53 s |
| Phase: missing | 3 ms |
| Phase: scoped GC | 1 ms |

## Rescan (warm DB, every file stat-clean)

```
scan: completed   total=1424 added=0 updated=0 unchanged=1424 moved=0 removed=0
                  errors=0 walk=73ms pipeline=0s missing=4ms gc=2ms
```

| Metric | Value |
|---|---|
| Wall time | **82 ms** |
| Throughput | 17,000 files/s (effectively `stat` rate) |
| Files imported | 0 (all unchanged) |
| Phase: walk | 73 ms |
| Phase: pipeline | 0 s (no folder had any Added/Changed files; pipeline workers never spawned) |

The stat-clean path opens **zero transactions**, runs **zero TagLib reads**, and computes **zero xxh3 hashes**. The full cost is `os.Stat` on each file plus one SELECT to load the `track_files` snapshot.

## Phase 2 acceptance gate checklist

| Requirement (PLAN.md row 2) | Status |
|---|---|
| Drop a folder of FLACs → multi-file tracks grouped from day one | ✅ folder-unit pipeline; one tx per folder; `track_files` deduped to single `tracks` row via MBID |
| Multi-edition coexistence | ✅ `track_files.is_preferred` partial-unique respected; first file wins, others import as non-preferred |
| Kill the server mid-scan; restart; resume from where it left off | ✅ in-tx cursor in `scan_runs.last_completed_folder`; on boot `resumeInterruptedScans` calls `Scanner.Resume()` per library, picks up by skipping `folder.Dir <= cursor` |
| Library queryable throughout | ✅ writes are per-folder tx (commit point every ~30 ms); reads use the reader pool, never block on writers in WAL mode |
| Move detection preserves annotations | ✅ `content_hash` lookup against missing rows in the EventAdded branch (`tryRepathByHash`) repath instead of insert; track_id stays stable across renames |
| Drive-mount detection | ✅ `os.Stat` of every `library_paths.path` at scan start; abort with `library_unreachable` if any fails |
| User-initiated cancel | ✅ `POST /api/v1/libraries/{id}/scan/cancel` cancels the doScan context; `status='cancelled'`; cursor preserved for resume |
| Plex-style "currently scanning X" | ✅ in-memory `Scanner.LiveState(runID)` exposed via `GET /scan-runs/{id}` when `status='running'`; updated per folder commit; zero extra DB writes |
| Recorded benchmark | ✅ this file |

## Comparison vs Navidrome 0.61.2 (same machine, host-native, smaller library)

From the user's Navidrome first-scan log captured earlier: 639 files / Phase 1 pipeline wall time 2.10 s → 304 files/s host-native.

Sarga in this benchmark: 1424 files / 14.7 s container = 97 files/s container.

The 3× per-file gap comes from two sources:

1. **xxh3-128 hash of every byte** (Sarga structural; Navidrome doesn't hash). For 51 GB read across NVMe + VirtioFS at ~3.5 GB/s, that's ~14 s of pure I/O — the dominant cost.
2. **VirtioFS bind-mount overhead** on macOS Docker. The standalone Go benchmark on the same library, same code, host-native (no container, direct FS access): **7.96 s for 1424 files = 179 files/s**. That's ~half the container time.

Projected host-native rate: ~180 files/s. Projected to 50K files: ~5 min first scan, then sub-second rescans for typical (few-hundred-files-changed) deltas.

## What this doesn't measure (deferred)

| Tier | Files | Status |
|---|---|---|
| 10K | ~10 albums × 12 tracks × 100 artists | Not measured — needs synthetic library |
| 100K | ~100 albums × 10 tracks × 1000 artists | Not measured — same |
| 500K | full reference tier | Not measured — same |
| Pi Zero 2 W | small library + ARMv8 + slow SD | Not measured |

This benchmark is one *real-world* point on a typical self-hoster's library, sized between the 1K and 10K tier. Synthetic-library benchmarks at higher tiers land alongside Phase 3 (streaming work, which has its own throughput targets).

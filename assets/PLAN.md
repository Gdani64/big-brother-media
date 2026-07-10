# POC: `big-brother` — event-driven media auto-linker

## Context

Downloads land in one folder on a Synology volume; they need to end up under the
right media library folder (`Movies`, `TV Shows`, `Music`, …) so Plex/File Station
see them. Symlinks are unreliable on Synology's native Plex package (File Station
hides them; Package Center sandboxing breaks cross-share targets), so the tool
**hardlinks** instead — same inode, shows up everywhere, no path stored, zero extra
disk. Because a TV show is a *folder*, and directories can't be hardlinked, the tool
recreates the directory tree and hardlinks each file inside it (`cp -al` semantics).

Detection of "what kind of media is this" is delegated to **Claude Haiku 4.5**, fed
only the item's name + the tree of inner filenames (no file-content reads).

The whole thing runs as a long-lived container watching the downloads dir via
**inotify (event-based, not polling)** so it idles at ~0% CPU — the stated
requirement. It's a POC: single static Go binary, one small image.

Module is already initialised as `github.com/Gdani64/big-brother` (Go 1.23.5).
Current `cmd/main.go` is a hello-world stub to be replaced.

## Decisions (confirmed)

- **Target categories**: the existing top-level dirs under `/volume1/Media` —
  `Movies`, `TV Shows`, `Music`, `Books`, `Games`, `Others`, `Archive`. Discovered
  dynamically at startup (excluding system/ignore dirs) and passed to Haiku as the
  allowed set, so adding a library folder needs no code change.
- **Classify on names only** — item name + capped inner filename tree.
- **Low confidence / unknown** → hardlink into `/volume1/Media/_unsorted/<item>`
  (created if missing) so it leaves downloads but is flagged for review.
- **Watch root**: `/volume1/Media/downloads` (create this folder on the NAS).

## Architecture

```
downloads/  ──inotify CREATE──▶  watcher ──settle timer──▶  pipeline
                                                              │
                                        ┌─────────────────────┼─────────────────────┐
                                        ▼                     ▼                     ▼
                                   classifier            linker (hardlink)      state store
                                   (Haiku 4.5)           cp -al semantics       (dedupe)
                                        │                     │
                                   category ────────────▶ /volume1/Media/<Category>/<item>
```

Idle path is pure inotify — no goroutine spins, no polling loop. Work happens only
when the kernel delivers a filesystem event.

### Project layout

```
big-brother/
  cmd/main.go                  # wire config → watcher → pipeline; signal handling
  internal/config/config.go    # env-driven config + category discovery
  internal/watcher/watcher.go  # fsnotify wrapper + per-item settle/debounce
  internal/classify/haiku.go   # Anthropic Go SDK call, JSON-schema result
  internal/linker/linker.go    # recursive hardlink (os.Link), EXDEV handling
  internal/pipeline/pipeline.go# orchestration: settle→classify→link→record
  internal/state/state.go      # tiny JSON "already processed" set
  Dockerfile
  docker-compose.yml           # Container Manager "Project"
  README.md                    # Synology test procedure (below)
```

## Component details

### `internal/config` — internal/config/config.go
Read from env (with defaults):
| Env | Default | Purpose |
|---|---|---|
| `MEDIA_ROOT` | `/volume1/Media` | library root |
| `DOWNLOADS_DIR` | `$MEDIA_ROOT/downloads` | watched folder |
| `UNSORTED_DIR` | `$MEDIA_ROOT/_unsorted` | low-confidence sink |
| `SETTLE_SECONDS` | `15` | quiet period before acting |
| `CATEGORIES` | *(auto)* | comma list; if empty, discover dirs |
| `ANTHROPIC_API_KEY` | — | required |
| `DRY_RUN` | `false` | log actions, link nothing |

`DiscoverCategories()` lists `MEDIA_ROOT`, keeps directories, drops the ignore set
(`@eaDir`, `#recycle`, `.DS_Store`, `downloads`, `_unsorted`, `Test`, anything
starting with `.` or `#`). Ignore set is shared with the walker.

### `internal/watcher` — internal/watcher/watcher.go
- `github.com/fsnotify/fsnotify` watch on `DOWNLOADS_DIR` (not recursive by default).
- On a `CREATE` for a **top-level** entry, register it as a pending item and start a
  `SETTLE_SECONDS` timer. If it's a directory, add recursive watches on its subtree
  so writes *into* it (new episode files, extraction) reset the timer. Any event
  under the item → reset timer.
- When an item's timer fires with no intervening events → emit it on a channel to the
  pipeline as "stable". Then drop the item's inner watches (keep footprint low).
- Skip ignore-set names and partial-download suffixes: `.part`, `.!qB`, `.aria2`,
  `.tmp`, `~`.

> **inotify gotchas to bake in:**
> - Events *do* fire for bind-mounted local dirs on Synology (SMB/rsync/host writes
>   hit the same kernel). Works.
> - Watch limit: large trees can exceed `fs.inotify.max_user_watches`. Note in README
>   how to raise it; handle fsnotify's watch-add errors by falling back to a single
>   settle timer (don't crash).
> - fsnotify has no native recursive watch — add/remove watches per subdir as `CREATE`
>   dir events arrive.

### `internal/classify` — internal/classify/haiku.go
- SDK: `github.com/anthropics/anthropic-sdk-go` (`go get`), model **`claude-haiku-4-5`**
  (constant `anthropic.ModelClaudeHaiku4_5_20251001`, or the bare string).
- Build a compact prompt: the item name + a **capped** walk (e.g. first 40 filenames,
  max depth 3) of inner names. Names only — never read file bytes.
- Constrain output with **structured outputs** (Haiku 4.5 supports it): JSON schema
  `{category: enum[<discovered categories> + "unknown"], confidence: number}`. Parse
  with `json.Unmarshal`; treat `unknown` or `confidence < 0.5` as low-confidence →
  `_unsorted`.
- `max_tokens` small (~256); no thinking config needed on Haiku. Handle the SDK error
  chain (`errors.As` → `*anthropic.Error`, branch on `StatusCode`); on API failure,
  fall back to `_unsorted` rather than dropping the item.

### `internal/linker` — internal/linker/linker.go
- `HardlinkTree(src, dst)`: `filepath.WalkDir(src)`; for each dir `os.MkdirAll(dstPath)`,
  for each file `os.Link(srcFile, dstFile)`. Skip ignore-set entries.
- If `dstFile` exists, skip (idempotent re-runs). If `os.Link` returns `EXDEV`
  (cross-device), log a clear error — src and dst must be on the same volume; do **not**
  fall back to copy in the POC.
- Single-file downloads: just `os.Link` the file into the category dir.
- Respect `DRY_RUN`.

### `internal/state` — internal/state/state.go
- JSON file at `$MEDIA_ROOT/.big-brother-state.json` mapping processed item name →
  {category, linkedAt}. Prevents re-processing on restart / duplicate events.
- Loaded at startup; the **startup reconcile** step lists `DOWNLOADS_DIR`, and any
  item not in state (and not obviously partial) is pushed through the pipeline once —
  so the tool "catches up" on whatever was already sitting there.

### `internal/pipeline` — internal/pipeline/pipeline.go
`Process(item)`: already-in-state? skip. Else classify → resolve target dir
(category or `_unsorted`) → `HardlinkTree` → record in state → structured log line.
Serialize processing (one worker) — throughput isn't the concern, footprint is.

### `cmd/main.go` — cmd/main.go
Load config, discover categories, open state, run startup reconcile, start watcher,
select-loop over the stable-item channel + `SIGINT/SIGTERM` for clean shutdown.

## Docker image & Synology setup

### Dockerfile (multi-stage, static)
```dockerfile
FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /big-brother ./cmd

FROM gcr.io/distroless/static-debian12
COPY --from=build /big-brother /big-brother
ENTRYPOINT ["/big-brother"]
```
`distroless/static` ships CA certs (needed for HTTPS to `api.anthropic.com`) and is
tiny. `CGO_ENABLED=0` → fully static binary.

### docker-compose.yml (Container Manager Project)
```yaml
services:
  big-brother:
    image: big-brother:latest
    container_name: big-brother
    restart: unless-stopped
    user: "0:0"            # POC: root, so it can write into the library shares
    environment:
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
      MEDIA_ROOT: /volume1/Media
      SETTLE_SECONDS: "15"
    volumes:
      - /volume1/Media:/volume1/Media   # SINGLE mount: container path == host path
```

> **Why one mount at the same path:** keeps `downloads` and every category on the
> *same device* inside the container (so `os.Link` succeeds), and makes container
> paths identical to host paths so logs/state are unambiguous. Do **not** mount
> `downloads` and each library folder as separate `-v` entries.
>
> **Permissions:** hardlinking needs write access to the library shares. POC runs as
> root (`user: "0:0"`). To harden later, run as the share-owner UID/GID
> (`user: "1026:100"`-style) and grant that user the shares in DSM.

## Synology test procedure (put in README.md)

1. **Enable SSH** (Control Panel → Terminal & SNMP → Enable SSH), then
   `ssh <you>@<nas>`; `sudo -i` for root.
2. **Prep dirs**: `mkdir -p /volume1/Media/downloads /volume1/Media/_unsorted`.
3. **Get the image onto the NAS** — either:
   - build on a dev box → `docker save big-brother:latest | gzip > bb.tgz`, copy to
     NAS, `docker load < bb.tgz`; **or**
   - over SSH on the NAS: `docker build -t big-brother:latest .`
4. **Create the Project** in Container Manager (Project → Create, point at the
   `docker-compose.yml`), or `docker compose up -d` over SSH. Put `ANTHROPIC_API_KEY`
   in a `.env` next to the compose file.
5. **Watch logs**: `docker logs -f big-brother`.
6. **Drop a test item**: copy a sample show folder into downloads, e.g.
   `cp -al "/volume1/Media/TV Shows/Frasier" /volume1/Media/downloads/Frasier.Test`
   (a hardlinked copy is enough to exercise it without new data).
7. **Expect**: after ~15s quiet, a log line `classified=TV Shows` and the tree
   appears at `/volume1/Media/TV Shows/Frasier.Test/`.
8. **Confirm real hardlinks (shared inodes, link count ≥ 2)**:
   ```
   ls -li /volume1/Media/downloads/Frasier.Test/*.mkv
   ls -li "/volume1/Media/TV Shows/Frasier.Test/"*.mkv
   ```
   inode numbers (first column) must match. Verify it shows up in File Station and
   Plex (it will — it's a normal file, not a symlink).
9. **Idempotency / restart**: `docker restart big-brother` → it must **not** re-link
   (state file covers it), and startup reconcile must pick up anything dropped while
   it was down.
10. **Low-confidence path**: drop a folder with an opaque name (e.g. `xzy-2024-rip/`)
    and confirm it lands in `_unsorted`.

If inotify seems to miss events on a huge tree, raise the watch limit:
`echo 'fs.inotify.max_user_watches=204800' >> /etc/sysctl.conf && sysctl -p`.

## Verification (end-to-end, before calling it done)

- **Happy path**: drop `Frasier.Test` → hardlinked into `TV Shows`, inodes match,
  visible in File Station.
- **Movie single-file**: drop one `.mkv` movie file → linked into `Movies`.
- **Low confidence**: opaque-named folder → `_unsorted`.
- **No-op idle**: leave running 5 min with no drops → container CPU ~0% (`docker
  stats`), no log spam (proves event-driven, not polling).
- **Restart safety**: restart → no duplicate links; catch-up reconcile works.
- **Same-volume guard**: (optional) point `DOWNLOADS_DIR` at a different volume and
  confirm a clear `EXDEV` error instead of a silent copy.

## Out of scope (POC)

Copy-on-EXDEV fallback, symlink mode, web UI, multi-worker throughput, per-file
re-detection, DSM permission automation, pushing to a registry.

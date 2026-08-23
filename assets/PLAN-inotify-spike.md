# Spike: inotify event trace during a Download Station transfer

## Objective

Before committing to the settle-timer heuristic, observe the **raw inotify event
stream** while Download Station downloads a real file, and find out whether there's
a **deterministic "done" signal** we can key on instead of a quiet-period timer.

The prime suspects we're hunting for:
- **`CLOSE_WRITE`** — fired when a writable file descriptor is closed. If DS opens the
  file, fills it, and closes it **once** at the end → that single `CLOSE_WRITE` is a
  gold-standard completion signal.
- **`MOVED_TO` / `MOVED_FROM`** — reveals if DS downloads to a temp name/location and
  **renames on completion**. A `MOVED_TO` into the folder = definitive done.
- **temp-name pattern** — any suffix (`.part`, hidden name, etc.) that appears during
  download and disappears/renames at the end.

> **Why raw inotify, not fsnotify/Go:** Go's `fsnotify` **collapses events and does not
> surface `IN_CLOSE_WRITE`** — the very signal we most want to check. So the spike uses
> `inotifywait` (from `inotify-tools`), which prints every raw op. Zero code to write.

## The whole artifact (minimal)

**Dockerfile**
```dockerfile
FROM alpine:3.20
RUN apk add --no-cache inotify-tools
# -m monitor (don't exit), -r recursive, print time | events | full path
ENTRYPOINT ["inotifywait","-m","-r","--timefmt","%F %T.%N","--format","%T | %e | %w%f","/watch"]
```

**docker-compose.yml**
```yaml
services:
  inotify-spike:
    build: .
    container_name: inotify-spike
    volumes:
      - /volume1/Media/downloads:/watch   # point DS to download here
```

Bring it up over SSH: `docker compose up -d --build`
Capture the trace: `docker logs -f inotify-spike | tee spike.log`

## Run the experiment

1. Make sure DS's destination is `/volume1/Media/downloads` (the `/watch` mount).
2. Start `docker logs -f inotify-spike | tee spike.log`.
3. In Download Station, add **one** download and let it run **to completion, plus
   ~30s after** (to catch any final rename and the Synology indexer touching `@eaDir`).
4. Repeat for **two protocols if you can** — a **BitTorrent** task and a plain
   **HTTP/direct** download — their write/rename behavior differs.

## What to look for in `spike.log` (the actual deliverable)

Walk the trace and answer:

| Question | If yes → detection signal |
|---|---|
| Is there exactly **one `CLOSE_WRITE`** per file, at the end? | Use `CLOSE_WRITE` as done-signal (best case) |
| Does a **`MOVED_TO`** land in the folder at the end? | Use rename-in as done-signal (also definitive) |
| Is there a **temp name / suffix** that renames away on finish? | Gate on "no temp files present" |
| Does the file appear **full-size immediately** then many `MODIFY`s? | Preallocated → size-stability is useless; don't use it |
| After completion, does **`@eaDir`** get written? | Weak secondary hint (indexer runs post-complete) — mostly just noise to ignore |
| What's the **longest gap between `MODIFY`s** mid-download? | Sizes `SETTLE_SECONDS` if we fall back to the timer |

Also note the practical stuff:
- Does DS **create the folder first, then files inside** (so we catch a dir `CREATE`
  then need recursive watches)?
- Any `CLOSE_WRITE,ISDIR` or dir-level events worth using as a boundary?

## Decision / next step

- **If `CLOSE_WRITE` or `MOVED_TO` is clean and once-per-item** → drop the settle-timer
  as the primary mechanism and key `big-brother` on that op (raw inotify via
  `golang.org/x/sys/unix`, since fsnotify can't see `CLOSE_WRITE`). Update PLAN.md.
- **If it's messy** (chunked closes, in-place writes, no rename) → keep the settle
  timer, but use the measured MODIFY-gap distribution to size `SETTLE_SECONDS`, and
  keep the partial-suffix gate.

## Caveats

- inotify fires for the bind-mounted local dir on the same kernel — works from the
  container, same as the real tool will.
- `inotifywait -m -r` sets up watches on existing subdirs at start; modern
  `inotify-tools` auto-watch newly-created subdirs, but if a fast folder-create races,
  you may miss a first event or two — fine for a spike.
- Optional noise cut once you've seen it once: add
  `--exclude '(@eaDir|#recycle|\.DS_Store)'` to the ENTRYPOINT. Leave it **off** for
  the first run so you can see whether `@eaDir` is a usable signal.

# AGENTS.md

Terminal anime streaming client (Go, Cobra + Bubble Tea v2). Module: `github.com/hishantik/anilix`, Go 1.26.

## Commands

```sh
go vet ./...            # CI runs this first
go test -short ./...    # CI runs this (skips network integration tests)
go build .              # CI builds with no -o flag
go build -o anilix .    # local build
```

- Match CI order: `vet -> test -short -> build`.
- Integration tests are real-network tests gated by `t.Skip` when `testing.Short()`; they are NOT named-tagged or build-tagged. `go test ./...` without `-short` hits live APIs (hianime, Jikan, AniList, extractors). Run one suite: `go test ./provider/hianime -run TestIntegration -v`.
- Unit tests are fast and offline; they cover `source`, `provider/jikan`, `provider/hianime`, `player` only.

## Termux/PRoot build gotcha

In PRoot environments (including Termux), `go build` inside the repo can fail with filesystem locking errors. Workaround: copy the tree elsewhere and build there:

```sh
mkdir -p /tmp/anilix-build && cp -r . /tmp/anilix-build/ && cd /tmp/anilix-build && go build -o ~/anilix .
```

## Architecture (non-obvious parts)

- Full writeup: `docs/architechture.md` — updated for the hianime provider, but still **partially stale**: it omits `auth/`, `update/`, and `cmd/anilist.go`, and no longer documents the removed AllAnime provider or the `AnimeLinker` (the SQLite ID cache it mentions never existed in code).
- **Design system**: `docs/design-system.md` is the spec. `tui/theme/` holds the tokens (colour roles, type scale, spacing scale, elevation, icons, capability detection); `tui/ui/` holds the components (button, chip, badge, card, list/menu item, tab, dialog, input field, info row, empty/error/loading states) and the layout helpers (`BreakpointOf`, `ListWidth`, `PreviewWidth`, `OverlayWidth`). `tui/shell.go` is the application shell (app bar, content region, footer) and `tui/nav.go` the navigation drawer. Screens must not hardcode colours or build their own lipgloss styles — compose `ui` components with `theme` roles. `anilix theme` (or `Ctrl+G` in the app) renders the whole system. The pre-redesign palette is gone: there is no `Theme` variable left in `tui`.
- Everything hangs off the `source.Source` interface (`source/source.go`): `Search`/`SeasonsOf`/`EpisodesOf`/`StreamsOf`.
- **Hianime (`provider/hianime`) is the only streaming provider** (scrapes hianime.at, like current ani-cli v5.x). AllAnime was removed from the codebase entirely: its `episode()` API requires an `aaReq` AES-GCM crypto token that is only obtainable inside a browser (Turnstile-protected SSR) — ani-cli, AniPlay, and anilix all abandoned it. Don't try to bring it back without dynamic key extraction, which upstream gave up on.
- **Latency budget**: search results appear in ~1s (hianime only); AniList ID + metadata resolution runs in the background (~2s, one batched request) and must never block the result list. Playback needs ~2s: episode list → servers → embed → master playlist, i.e. three sequential HTTP round trips (~0.8s each) — inherent to the provider, not to our code.
- **The hianime episode list of a long series is >1MB** and is needed twice per episode view (episode number → internal data-id), so `provider/hianime` caches episode lists (30 min) and server lists (10 min) in the provider instance. The provider is a singleton in the TUI, so the cache is shared for the session.
- **Jikan's API goes down for hours at a time** (`api.jikan.moe` unreachable while `jikan.moe` still serves the site). The client keeps a 5-minute circuit breaker (`JikanClient.IsDown()`), so failing calls are skipped instead of stalling every search. AniList is the metadata provider precisely because it stays reachable.
- Jikan and AniList are metadata-only. **AniList is the primary metadata provider**: hianime gives no AniList ID, so `tui/commands.go` resolves titles through `anilist.Client.ResolveByTitles` (one request, aliased `Page` fields per title) which fills both `AniListID` and `MALID` and re-enables the batched metadata path (`GetAnimeBatch`). Matching is exact-after-normalization (Unicode-aware: "Journey’s End" = "Journey's End"), never fuzzy, because a near-match attaches the wrong anime.
- `Anime.ShowID` is the streaming-provider show ID (hianime's numeric id); `Anime.MALID` is also recovered from hianime's embed URL (`/mal/<id>/`) when episodes are fetched, which powers AniSkip and Jikan episode titles.
- **hianime's search page has several unrelated sections** (carousels for top/latest anime). Only the results grid — `<div class="flw-item">` blocks, show id on the inner `film-poster-ahref` anchor's `data-id` — holds real hits. The carousels use `film-poster item-qtip` markup; zipping those ids with the titles silently pairs every show with One Piece's id.
- **Not every hianime entry has a MAL id**: embeds are either `/stream/mal/<malid>/...` (MAL id) or `/stream/anime/<internalid>/...` (hianime-internal, no MAL id). The latter still plays fine — it just can't offer AniSkip or ID-based metadata.
- **Server names on hianime are unstable** (`ZokoAnime`, `HD-1`, …) while the embed host is stable. Rank servers by embed host, not name, and fall back across sub/dub: some episodes only have one translation.
- Hianime playback needs Referer (`NeedsReferrer: true`) and on Android goes through `player/proxy.go`, which does raw UDP DNS to 8.8.8.8.
- Providers and extractors self-register via `init()` + `Register()` — new ones need only a file with an `init()`, no registry edits.
- Version lives in `version/version.go`; releases inject it via ldflags `-X github.com/hishantik/anilix/version.Version` (`.goreleaser.yml`). Local builds show `dev`. `docs/architechture.md` cites a `cmd.version` path that doesn't exist.

## Conventions

- **HTTP goes through the `curl` package** (`curl.Get`/`curl.Post`), which shells out to the curl binary — intentional for PRoot/Termux DNS/TLS compatibility. Use it for new provider/extractor/API code, not `net/http`. (Exceptions already exist: player proxy, m3u8 parser.)
- **Streaming CDN fetches must use `curl.GetResolved`** (added in `curl/dns.go`): many devices run DNS-level content filters that blocklist streaming hosts (e.g. `hls.dramahot.top`) with a NextDNS blockpage; plain `curl.Get` then fails with exit 60 or returns blockpage HTML. `GetResolved` pins the IP via raw UDP DNS to 8.8.8.8/1.1.1.1 and falls back to system resolution elsewhere. The hianime provider uses it for every request; playback on Android already bypasses DNS via `player/proxy.go`.
- Bubble Tea v2 import paths are `charm.land/bubbletea/v2` etc. (not `github.com/charmbracelet/...`).
- Config via Viper at `~/.anilix/anilix.toml`; env overrides use `ANILIX_` prefix (e.g. `ANILIX_PLAYER=vlc`).
- CLI: `anilix` (TUI, default), `anilix sources`, `anilix version`, `anilix anilist login|logout|status`.

## Version Control — Strict Rules

These rules are **non-negotiable**. Violations caused the loss of 7 commits in a previous session.

### Commit and push after every phase

After completing **each phase** of any multi-phase task, immediately commit and push before starting the next phase. A phase is not complete until the remote has it.

```sh
git add -A && git status --short
git commit -m "$(cat <<'EOF'
type: concise commit message describing the phase

What changed and why it matters.
EOF
)"
git push
```

### Never run destructive commands without verifying the path first

Never use `rm -rf` with a path that contains `~` or `$HOME` inside a PRoot/Termux environment, because `~` resolves to the actual home directory. Always use absolute paths.

```sh
# WRONG — deletes ~/anilix instead of the intended temp dir:
rm -rf /tmp/anilix-build ~/anilix

# RIGHT — absolute paths, never globbed:
rm -rf /data/data/com.termux/files/usr/tmp/anilix-build
```

### Keep the remote as the source of truth

Every meaningful change must be pushed before the session ends. A commit that exists only locally is not a commit — it is a draft. If the remote does not have it, it is not done.

### Build verification before push

Before pushing, verify the build succeeds in PRoot to catch filesystem-lock issues early:

```sh
mkdir -p /data/data/com.termux/files/usr/tmp/anilix-build && \
  cp -r . /data/data/com.termux/files/usr/tmp/anilix-build/ && \
  cd /data/data/com.termux/files/usr/tmp/anilix-build && \
  go build -o /data/data/com.termux/files/usr/tmp/anilix-bin . && \
  rm -rf /data/data/com.termux/files/usr/tmp/anilix-build /data/data/com.termux/files/usr/tmp/anilix-bin
```

## Release

GoReleaser on `v*` tags (`.github/workflows/release.yml`): desktop (linux/darwin/windows amd64+arm64) plus a separate termux/android arm64 build, `CGO_ENABLED=0`.

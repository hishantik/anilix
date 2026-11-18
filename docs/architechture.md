# Anilix - Project Documentation

A terminal anime streaming client written in Go, styled with Material Design 3. Search anime, browse episodes, and stream directly from your terminal.

---

## Table of Contents

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Architecture](#architecture)
- [Core Interfaces](#core-interfaces)
- [Package Deep Dive](#package-deep-dive)
  - [source](#source)
  - [cmd](#cmd)
  - [config](#config)
  - [tui](#tui)
  - [provider/jikan](#providerjikan)
  - [provider/anilist](#provideranilist)
  - [provider/hianime](#providerhianime)
  - [extractor](#extractor)
  - [player](#player)
  - [aniskip](#aniskip)
  - [curl](#curl)
- [Data Flow](#data-flow)
- [Key Design Patterns](#key-design-patterns)
- [Configuration](#configuration)
- [Build & Run](#build--run)
- [Testing](#testing)
- [Release & Distribution](#release--distribution)
- [Platform Support](#platform-support)

---

## Overview

Anilix is a Go CLI/TUI application that lets users search for anime, browse episodes, and stream video directly from the terminal. It scrapes streaming from hianime.at and enriches results with metadata from Jikan (MyAnimeList API), AniList and Kitsu.

### Key Features

- **Interactive TUI** - Two-column search layout with instant metadata preview, built on Bubble Tea v2
- **Batch Metadata** - Fetches all search result metadata in one batched call, with Jikan and Kitsu fallbacks for entries AniList does not match
- **Multi-provider Search** - AniList + Kitsu ID resolution (one request each), Jikan for MAL-only entries, unified results
- **Quality Selection** - Cycle through presets: best, 1080p, 720p, 480p, 360p, auto
- **Sub/Dub Toggle** - Switch between subtitled and dubbed versions
- **Auto-Skip Intro/Outro** - AniSkip integration with mpv Lua script
- **Recent Searches** - Persists last 10 searches for quick access
- **Multi-host Extraction** - Parallel stream extraction from multiple hosts with fallback
- **Android Proxy** - Local HTTP proxy for seamless playback on Termux/PRoot
- **ID Mapping** - MAL IDs recovered from the hianime embed URL, enabling AniSkip and metadata

---

## Tech Stack

| Technology | Purpose |
|------------|---------|
| **Go 1.26** | Language and runtime |
| [Cobra](https://github.com/spf13/cobra) v1.10 | CLI command framework |
| [Viper](https://github.com/spf13/viper) v1.21 | Configuration management (TOML) |
| [Bubbletea v2](https://github.com/charmbracelet/bubbletea) | TUI framework (Elm architecture) |
| [Bubbles v2](https://github.com/charmbracelet/bubbles) | TUI components (list, textinput, spinner, progress, help) |
| [Lipgloss v2](https://github.com/charmbracelet/lipgloss) | Terminal styling and layout |
| [GoReleaser](https://github.com/goreleaser/goreleaser) | Cross-platform release builds |
| `exec.Command("curl")` | HTTP requests via curl binary (PRoot compatibility) |
| `crypto/aes` | AES-256-CTR stream payload decryption |
| `crypto/cipher` | AES-GCM token / payload decryption |

---

## Project Structure

```
anilix/
├── main.go                          # Entry point - delegates to cmd.Execute()
├── cmd/
│   ├── root.go                      # Root command + tui subcommand
│   ├── sources.go                   # "sources" subcommand (list providers)
│   └── version.go                   # "version" subcommand
├── config/
│   └── config.go                    # Viper-based config at ~/.anilix/anilix.toml
├── source/
│   ├── source.go                    # Source interface + Season type
│   ├── anime.go                     # Anime data model
│   ├── episode.go                   # Episode data model
│   ├── episode_test.go              # Episode unit tests
│   └── stream.go                    # Stream data model
├── tui/
│   ├── models.go                    # State machine (SearchModel, state types)
│   ├── search.go                    # SearchModel - main TUI model with Init/Update
│   ├── views.go                     # View rendering for all states
│   ├── shell.go                     # Application shell (app bar, content region, footer)
│   ├── nav.go                       # Navigation drawer and its destinations
│   ├── commands.go                  # Tea commands (async operations)
│   ├── messages.go                  # Tea message types
│   ├── items.go                     # List rows and the result list delegate
│   ├── keymap.go                    # Key binding definitions
│   ├── furniture.go                 # Screen furniture shared by the views
│   ├── themeview.go                 # The design system showcase
│   ├── recent.go                    # Recent search persistence
│   ├── theme/                       # Design tokens: roles, type, spacing, shape, icons
│   └── ui/                          # Components: buttons, chips, lists, dialogs, fields
├── provider/
│   ├── init.go                      # Provider registration note
│   ├── provider.go                  # Provider registry
│   ├── jikan/
│   │   ├── jikan.go                 # JikanProvider (Source impl - search/metadata)
│   │   ├── client.go                # Jikan REST API client
│   │   ├── client_test.go           # Client unit tests
│   │   ├── jikan_test.go            # Provider unit tests
│   │   ├── types.go                 # Jikan API response types
│   │   ├── init.go                  # Jikan provider registration
│   │   └── integration_test.go      # Network integration tests
│   ├── anilist/
│   │   ├── anilist.go               # AniListProvider (Source impl - metadata)
│   │   ├── client.go                # AniList GraphQL client
│   │   ├── types.go                 # AniList GraphQL types
│   │   └── integration_test.go      # Network integration tests
│   └── hianime/
│       ├── hianime.go               # Provider (Source impl - search/episodes/streams)
│       ├── client.go                # hianime.at scraping client
│       ├── blob.go                  # Embed config deobfuscation (base64 + XOR)
│       ├── timeouts.go              # Per-operation timeouts
│       ├── init.go                  # Hianime provider registration
│       ├── hianime_test.go          # Parser unit tests
│       └── integration_test.go      # Network integration tests
├── extractor/
│   ├── extractor.go                 # Extractor interface + registry + Resolve()
│   ├── init.go                      # Extractor registration (init)
│   ├── aes.go                       # AES-256-CTR decryption helper
│   ├── hianime.go                   # HiAnime extractor
│   ├── filemoon.go                  # FileMoon extractor
│   ├── wixmp.go                     # Wixmp extractor
│   ├── youtube.go                   # YouTube extractor
│   ├── mp4upload.go                 # Mp4Upload extractor
│   ├── vidguard.go                  # VidGuard extractor
│   ├── streamwish.go                # StreamWish extractor
│   ├── sharepoint.go                # SharePoint extractor
│   ├── generic.go                   # Generic m3u8/mp4 URL handler
│   ├── m3u8.go                      # M3U8 playlist parsing
│   └── extractor_integration_test.go # Network integration tests
├── player/
│   ├── play.go                      # Player struct + Launch() + args builders
│   ├── detect.go                    # Player detection (mpv, vlc, iina, Android)
│   ├── proxy.go                     # Local HTTP proxy for Android playback
│   ├── detect_test.go               # Detection unit tests
│   ├── play_test.go                 # Play unit tests
│   └── ani-skip.lua                 # Embedded mpv Lua script for auto-skip
├── aniskip/
│   └── client.go                    # AniSkip API client (intro/outro times)
├── curl/
│   ├── curl.go                      # HTTP client via curl binary
│   └── dns.go                       # DNS-pinned fetch (GetResolved) for filtered CDNs
├── assets/                          # SVG logo
├── docs/
│   └── architechture.md             # This document
├── go.mod                           # Module definition
├── go.sum                           # Dependency checksums
├── .goreleaser.yml                  # GoReleaser config for cross-platform builds
├── install.sh                       # Linux/macOS/Termux installer
├── install.bat                      # Windows installer
├── LICENSE                          # MIT License
└── README.md                        # User-facing documentation
```

---

## Architecture

```
                        +-------------------+
                        |    TUI / CLI      |
                        |  (Bubbletea v2)   |
                        +---------+---------+
                                  |
                                  v
                    +---------------------------+
                    |    Source Interface        |
                    |  (source/source.go)        |
                    |                            |
                    |  Search()                  |
                    |  SeasonsOf()               |
                    |  EpisodesOf()              |
                    |  StreamsOf()               |
                    +---------------------------+
                      |            |           |
              +-------+    +------+------+    +----------+
              v              v              v
     +----------------+ +------------+ +------------------+
     | JikanProvider  | | AniList    | | HianimeProvider  |
     | (metadata)     | | (metadata) | | (search +        |
     | REST API       | | GraphQL    | |  episodes +      |
     |                | |            | |  streams)        |
     +-------+--------+ +------------+ +--------+---------+
             |                                   |
             +-----------------------------------+
                         |
                         v
              +---------------------+
              |  hianime.at scrape  |
              |  episodes + servers |
              |  → embed → m3u8     |
              +---------------------+
                         |
                         v
              +---------------------+
              |  Stream Extractors  |
              | (parallel, priority)|
              | hianime > filemoon  |
              | > wixmp > youtube   |
              +---------------------+
                         |
                         v
              +---------------------+
              |   Player Launcher   |
              | mpv/vlc/iina/      |
              | mpv-android/vlc-    |
              | android             |
              +---------------------+
```

---

## Core Interfaces

### `source.Source` (`source/source.go`)

The central abstraction for all anime data providers:

```go
type Source interface {
    Name() string
    ID() string
    Search(query string) ([]*Anime, error)
    SeasonsOf(anime *Anime) ([]Season, error)
    EpisodesOf(anime *Anime, season int) ([]*Episode, error)
    StreamsOf(episode *Episode) ([]*Stream, error)
}
```

Three implementations exist:
- **JikanProvider** - Implements `Search` (metadata). `SeasonsOf`, `EpisodesOf`, `StreamsOf` return empty (not its role).
- **AniListProvider** - Implements `Search` (metadata). Same as Jikan, other methods return empty.
- **HianimeProvider** - Implements all methods. Primary source for search, episodes and streams.

### `extractor.Extractor` (`extractor/extractor.go`)

Resolves playable stream URLs from embed/hosting pages:

```go
type Extractor interface {
    Name() string
    CanHandle(url string) bool
    Extract(ctx context.Context, url, referer string) ([]*source.Stream, error)
    ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error)
}
```

Eight implementations: hianime, filemoon, wixmp, youtube, mp4upload, vidguard, streamwish, sharepoint. Registered via `init()` + `Register()`.

---

## Package Deep Dive

### `source`

Core data models shared across all packages.

**Types:**
- `Anime` - Anime title with metadata: Name, URL, Cover, Year, Genres, Status, MALID, AniListID, ShowID, EpisodeCount, Type, Rating, Score, Rank, Popularity
- `Episode` - Episode with Number (float64), URL, Title, Synopsis, Aired date, Score, Filler/Recap flags, Duration, Season, and back-reference to Anime
- `Stream` - Playable stream URL with Provider name, Quality, URL, Referer, NeedsReferrer flag
- `Season` - Season number and name

### `cmd`

Cobra CLI commands.

| Command | Description |
|---------|-------------|
| `anilix` | Root command, launches TUI (default) |
| `anilix tui` | Explicit TUI launch |
| `anilix version` | Print version info |
| `anilix sources` | List registered providers |

The `rootCmd` defaults to running `tui.RunSearch()` when no subcommand is given.

### `config`

Viper-based configuration at `~/.anilix/anilix.toml`.

**Defaults:**
| Key | Default | Description |
|-----|---------|-------------|
| `player` | `mpv` | Media player |
| `quality` | `auto` | Stream quality preset |
| `source` | (empty) | Preferred stream source |
| `history.enabled` | `true` | Watch history |
| `aniskip.enabled` | `true` | Auto-skip intros/outros |

**Functions:** `Get()`, `GetString()`, `GetBool()`, `Set()` (auto-saves), `Save()`, `HistoryPath()`

Environment variables supported with `ANILIX_` prefix (e.g., `ANILIX_PLAYER=vlc`).

### `tui`

Bubble Tea v2 terminal UI with an Elm architecture state machine.

**States:**
1. `searchState` - Search bar + two-column results (list left, metadata preview right) + recent searches
2. `detailState` - Two-column detail view (metadata/cover left, episode list right)
3. `confirmQuitState` - Quit confirmation dialog
4. `settingsState` - Settings popup (quality cycling, aniskip toggle)

**Key components of `SearchModel`:**
- `textInput` - Search input styled from the design system
- `searchList` / `episodeList` - Bubbles list components
- `loading` - Spinner for async operations
`progress` - Progress bar with smooth animation
- `metadataCache` - Map[int]*MetadataPanel for instant navigation
- `episodeTitlesCache` - Map for episode title lookups
- `episodeMetadataCache` - Map for Jikan episode metadata
- `recentSearches` - Persisted recent search queries

**Async operations (tea.Cmd):**
- `doSearch(query)` - Search hianime, enrich MAL IDs with one Jikan lookup
- `fetchMetadataBatch(results)` - Batch-fetch AniList metadata for all results
- `fetchEpisodes(anime)` - Fetch episodes for selected anime
- `fetchEpisodeMetadata()` - Fetch Jikan episode metadata on navigation
- `playEpisode(anime, episodeNum)` - Resolve streams and launch player

**Layout:**
- Search state: 55% left (list), 45% right (metadata preview). Collapses to single column on narrow terminals (<80 cols).
- Detail state: 35% left (cover/metadata), 65% right (title/synopsis/episodes). Same responsive behavior.

**Keybindings:**
| Key | Action |
|-----|--------|
| `j`/`k`/arrows | Navigate lists |
| `Enter` | Search / Select / Confirm |
| `Esc` | Back / Cancel |
| `/` | Focus search bar |
| `Ctrl+T` | Toggle sub/dub |
| `Ctrl+S` | Open settings popup |
| `Ctrl+G` | Open the design system showcase |
| `Ctrl+C` | Quit |

**Design system:**
The visual foundation lives in two packages, documented in
[docs/design-system.md](design-system.md):

- `tui/theme` - semantic colour roles, the typography and spacing scales, the
  elevation model, the icon set and the terminal capability detection
  (`theme.Profile`, `theme.HasColor`, `theme.HasUnicode`).
- `tui/ui` - the component library: buttons, chips, badges, cards, list and
  menu items, tabs, dialogs, input fields, metadata rows, empty/error/loading
  states, and the layout helpers (`ui.BreakpointOf`, `ui.ListWidth`,
  `ui.OverlayWidth`).
- `tui/shell.go` and `tui/nav.go` - the frame of the application: the app bar,
  the content region, the footer and the navigation drawer.

Screens must not hardcode colours or build their own styles: they compose
`ui` components with `theme` roles. The pre-redesign palette is gone; every
screen of the application draws itself from the tokens.

Run `anilix theme` to print the whole system, or press `Ctrl+G` inside the
app.

### `provider/jikan`

REST API client for the [Jikan](https://api.jikan.moe/v4) (MyAnimeList unofficial API).

**JikanClient** - Makes HTTP requests via `curl` package to `https://api.jikan.moe/v4`:
- `SearchAnime(ctx, query)` - Search anime by title
- `GetAnimeFull(malID)` - Full anime details including synopsis
- `GetAnimeEpisodes(malID)` - Episode list for an anime

**JikanProvider** - Implements `source.Source`. Maps Jikan API responses to `source.Anime` with English title preference (falls back to default title).

### `provider/anilist`

GraphQL client for the [AniList](https://anilist.co) API. Used for batch metadata enrichment.

**Client** - Makes GraphQL POST requests via `curl` package:
- `SearchAnime(ctx, query, limit)` - Search by title
- `GetMediaBatch(ctx, malIDs)` - Batch fetch metadata by MAL IDs (single API call)

**AniListProvider** - Implements `source.Source`. Maps AniList responses to `source.Anime`. Primarily used for metadata, not streaming.

### `provider/hianime`

Scraping client for [hianime.at](https://hianime.at), the provider current ani-cli uses. This is the core content provider.

**Client** - Scrapes hianime.at through the `curl` package (`curl.GetResolved`, which pins DNS so device-level DNS filters cannot serve a blockpage instead of the stream CDN):
- `Search(ctx, query)` - Scrape `/search?keyword=` for show id, slug, title, poster, type
- `GetEpisodes(ctx, showID)` - `/api/theme/episode/list/<id>` → (episode number, internal data-id) pairs
- `GetEmbed(ctx, epDataID, translation)` - `/api/theme/episode/servers?episodeId=` → ZokoAnime embed URL (base64 `data-hash`)
- `GetStreamConfig(ctx, embedURL)` - Fetch embed page, decode `window.__P` (base64 + XOR `otaku-embed-v1`) → m3u8 src + subtitles
- `GetVariants(ctx, masterURL, referer)` - Parse master HLS playlist into quality variants

**Provider** - Implements `source.Source`:
- `Search()` - Scrape search results; MAL IDs are filled by the TUI via a single Jikan title match
- `EpisodesOf()` - Episode list; opportunistically recovers `MALID` from the embed URL (`/mal/<id>/`)
- `StreamsOf()` - Episode → data-id → embed → m3u8 master playlist → quality variants (sorted best first, with Referer set)

**Notes:**
- The servers API needs hianime's internal `data-id`, not the episode number; it is resolved from the episode list.
- The embed URL carries the MAL ID, which is why AniSkip and Jikan metadata keep working.
- Playback requires a Referer header (`NeedsReferrer: true`); on Android it is rewritten by `player/proxy.go`.

### `extractor`

Stream URL extractors for various hosting providers. Each extractor handles a specific host.

**Registry:** Extractors register via `init()` + `Register()`. `Resolve(url)` finds the first matching extractor.

**Priority order** (controlled by `ProviderPriority` map):
1. hianime (priority 1)
2. filemoon (priority 2)
3. wixmp (priority 3)
4. youtube (priority 4)

**Extractors:**

| Extractor | File | Handles |
|-----------|------|---------|
| HiAnime | `hianime.go` | hianime.to embed pages |
| FileMoon | `filemoon.go` | filemoon.sx embed pages |
| Wixmp | `wixmp.go` | wixmp CDN URLs |
| YouTube | `youtube.go` | YouTube video URLs |
| Mp4Upload | `mp4upload.go` | mp4upload.com embeds |
| VidGuard | `vidguard.go` | vidguard.to embeds |
| StreamWish | `streamwish.go` | streamwish embeds |
| SharePoint | `sharepoint.go` | SharePoint video URLs |
| Generic | `generic.go` | Direct m3u8/mp4 URLs |
| M3U8 | `m3u8.go` | M3U8 playlist parsing |

**Helpers:**
- `NoSubtitlesExtractor` - Embed wrapper for extractors that don't support subtitles
- `aes.go` - AES-256-CTR decryption helper used by some extractors

### `player`

Media player detection, launching, and Android proxy support.

**Player types:** mpv, vlc, iina (macOS only), mpv-android, vlc-android

**Detection (`detect.go`):**
- Checks `player --version` for desktop players
- Checks `pm list packages` for Android players
- Priority: mpv > vlc > iina (macOS only)

**Launch (`play.go`):**
- Builds player-specific arguments (title, subtitles, referrer, skip times)
- For Android: starts local HTTP proxy, replaces URL with localhost, launches via `am start`
- Skip script: embedded `ani-skip.lua` written to `~/.anilix/ani-skip.lua`

**Android Proxy (`proxy.go`):**
- Starts local HTTP server on random port
- Proxies remote video URLs through localhost
- Rewrites m3u8 playlists to route segments through proxy
- Custom DNS resolution via raw UDP to 8.8.8.8 (bypasses Termux DNS issues)
- 2s playlist cache TTL, 30 min server lifetime
- Content-Type sniffing for video formats

### `aniskip`

Client for the [AniSkip](https://api.aniskip.com) API.

- `GetSkipTimes(malID, episodeNum)` - Fetches intro (op) and outro (ed) skip intervals
- `FormatForScriptOpts(intervals)` - Formats for mpv script-opts: `op:87.5-118.2,ed:1340.0-1370.5`

### `curl`

HTTP client wrapper that shells out to the `curl` binary instead of using `net/http`.

**Functions:**
- `Get(ctx, url, headers)` - GET request
- `Post(ctx, url, headers, body)` - POST request
- `PostRaw(ctx, url, headers, body)` - POST with raw bytes (for GraphQL)
- `GetResolved(ctx, url, headers)` - GET with DNS pinned via raw UDP to 8.8.8.8/1.1.1.1 (bypasses filtered/broken system DNS)
- `ParseJSONBody(data)` - Extract JSON from `{"data": ...}` wrapper

This exists for PRoot/Termux compatibility where Go's `net/http` can have issues with DNS and TLS.

---

## Data Flow

### Search Flow
```
1. User types query in TUI search bar
2. TUI calls doSearch(query) → tea.Cmd
3. HianimeProvider.Search(query) → hianime.at search page scrape
4. One Jikan lookup for the same query → titles matched to fill MAL IDs
5. Results sorted by match score; TUI displays them in two-column layout
6. Batch: AniListClient.GetMediaBatch(malIDs) → metadata for all results
7. Metadata cached in metadataCache for instant j/k navigation
```

### Episode Flow
```
1. User selects anime → Enter
2. HianimeProvider.EpisodesOf(anime)
   a. Scrape /api/theme/episode/list/<showID> → episode numbers + data-ids
   b. If anime.MALID is still 0: read it from the embed URL (/mal/<id>/)
3. TUI displays episode list
4. User navigates → Jikan episode metadata fetched on debounce (150ms)
```

### Playback Flow
```
1. User selects episode → Enter
2. HianimeProvider.StreamsOf(episode)
   a. Episode number → data-id (from the episode list)
   b. Servers API → ZokoAnime embed URL (base64 data-hash)
   c. Embed page → window.__P → base64 + XOR decode → m3u8 master URL
   d. Master playlist → quality variants, sorted best first (Referer set)
3. AniSkip.GetSkipTimes(malID, episodeNum) → skip intervals
4. Player.Launch(stream.URL, Options{Title, Referrer, Subtitles, SkipTimes})
   a. Desktop: exec player binary with args
   b. Android: StartProxy() → am start with local URL
```

---

## Key Design Patterns

### Provider/Extractor Registration
Both providers and extractors use Go's `init()` function + a `Register()` function to register themselves at program startup. This allows adding new providers/extractors by simply creating a new file with an `init()` function.

```go
// In provider/jikan/init.go
func init() {
    provider.Register(&provider.Provider{ID: "jikan", Name: "Jikan", CreateSource: ...})
}

// In extractor/hianime.go
func init() {
    extractor.Register(&HiAnimeExtractor{})
}
```

### HTTP via curl
All HTTP is done through `exec.Command("curl", ...)` rather than `net/http`. This is intentional for PRoot/Termux environments where Go's built-in HTTP client can have DNS and TLS issues.

### Parallel Stream Extraction
The extractor registry resolves embed/hosting pages to direct m3u8/mp4 URLs:
- Extractors are tried in priority order per source URL
- First successful result returned (with 500ms grace for additional results)
- Per-extractor timeout of 5 seconds
- Total extraction timeout of 10 seconds

### State Machine TUI
The TUI follows Bubble Tea's Elm architecture with explicit state transitions:
```
searchState → detailState → (play episode)
    ↑              |
    +--- Esc ------+
    |
    v
confirmQuitState
    |
    v
settingsState
```

### Batch Metadata
AniList's `GetMediaBatch` API fetches metadata for all search results in a single call. This enables instant j/k navigation without per-item network requests. Individual Jikan fetches are used as fallback.

### Android Proxy
On Android/Termux, players launched via `am start` from PRoot can't directly fetch remote URLs. The proxy:
1. Starts a local HTTP server on `127.0.0.1:0` (random port)
2. Fetches remote content and streams it to the player
3. Rewrites m3u8 playlists so segment URLs route through the proxy
4. Uses raw UDP DNS queries to 8.8.8.8 (bypasses Termux `[::1]:53` issue)

---

## Configuration

Config file: `~/.anilix/anilix.toml`

```toml
player = "mpv"        # mpv, vlc, iina, mpv-android, vlc-android
quality = "auto"      # best, 1080p, 720p, 480p, 360p, auto
source = ""           # preferred stream source (empty = auto)

[history]
enabled = true        # enable watch history

[aniskip]
enabled = true        # auto-skip intros/outros
```

Settings can also be changed at runtime via the `Ctrl+S` popup menu in the TUI.

Environment variables override config file values with `ANILIX_` prefix:
```bash
ANILIX_PLAYER=vlc ANILIX_QUALITY=720p anilix
```

### Data Files

| Path | Purpose |
|------|---------|
| `~/.anilix/anilix.toml` | Configuration file |
| `~/.anilix/history.json` | Recent search history (last 10) |
| `~/.anilix/ani-skip.lua` | mpv Lua script for auto-skip |

---

## Build & Run

### Standard Build

```bash
go build -o anilix .
```

### PRoot/Termux Build (required for Termux/PRoot environments)

```bash
mkdir -p /tmp/anilix-build && cp -r . /tmp/anilix-build/ && cd /tmp/anilix-build && go build -o /storage/self/primary/github/Anilix/anilix .
```

### Run

```bash
# Launch TUI (default)
./anilix

# Explicit subcommands
./anilix tui
./anilix version
./anilix sources
```

### Quick Install

```bash
# Linux / macOS / Termux
curl -fsSL https://raw.githubusercontent.com/hishantik/anilix/main/install.sh | sh

# Windows (PowerShell)
curl -fsSL https://raw.githubusercontent.com/hishantik/anilix/main/install.bat -o install.bat && .\install.bat

# Go Install
go install github.com/hishantik/anilix@latest
```

---

## Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./source -v
go test ./provider/jikan -v
go test ./provider/hianime -v
go test ./extractor -v
go test ./player -v

# Run integration tests (require network, skip with -short)
go test ./provider/hianime -v -run TestIntegration -short=false
go test ./provider/jikan -v -run TestIntegration -short=false
go test ./extractor -v -run TestIntegration -short=false
go test ./provider/anilist -v -run TestIntegration -short=false
```

Integration tests are gated behind `-short=false` flag and require network access.

---

## Release & Distribution

GoReleaser (`.goreleaser.yml`) builds for:

| Build ID | OS | Arch |
|----------|----|------|
| desktop | linux, darwin, windows | amd64, arm64 |
| termux | android | arm64 |

Version is injected via ldflags: `-X github.com/hishantik/anilix/version.Version={{ .Version }}`

Archives: `.tar.gz` (Linux/macOS/Termux), `.zip` (Windows).

---

## Platform Support

| Platform | Supported Players |
|----------|-------------------|
| Linux | mpv, vlc |
| macOS | mpv, vlc, iina |
| Windows | mpv, vlc |
| Android (Termux) | mpv-android, vlc-android |

Android playback uses a local HTTP proxy to bridge the gap between PRoot and Android's media players.

---

## License

This project is licensed under the [MIT License](../LICENSE).

Copyright (c) 2026 Hishantik

Inspired by [ani-cli](https://github.com/pystardust/ani-cli).

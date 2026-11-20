# Anilix UI Audit — Phase 0

**Scope:** read-only inspection of `github.com/hishantik/anilix` at commit `0e91938` (branch `ui-improvements`).
**No source code was modified.** Every claim below is referenced by `file:line`.

---

## 1. Existing architecture overview

### 1.1 Stack

| Dependency | Version | Role |
|---|---|---|
| Go | 1.26 (`go.mod:3`) | runtime |
| `charm.land/bubbletea/v2` | v2.0.6 | TUI framework (Elm architecture) |
| `charm.land/lipgloss/v2` | v2.0.3 | styling / layout |
| `charm.land/bubbles/v2` | v2.1.0 | `list`, `textinput`, `help`, `spinner`, `progress`, `key` |
| `github.com/spf13/cobra` | v1.10.2 | CLI commands |
| `github.com/spf13/viper` | v1.21.0 | config (`~/.anilix/anilix.toml`) |
| `golang.org/x/image` | v0.41.0 | CatmullRom scaling for cover art |

Note: Bubble Tea **v2** — the API differs from v1 (`tea.View`, `tea.NewView(string)`, `tea.Raw()`).

### 1.2 Layering

```
cmd/ (cobra)          → tui.RunSearch() → returns *source.SelectionResult
tui/ (bubbletea)       → the only consumer of providers; owns all state
  ├─ provider/hianime  → the single streaming provider (search/episodes/streams)
  ├─ provider/anilist  → metadata (GraphQL) + tracking (mutations)
  ├─ provider/jikan    → metadata fallback + episode titles/per-episode metadata
  ├─ player/           → player detection, arg building, Android local proxy
  ├─ aniskip/          → intro/outro intervals
  ├─ auth/             → AniList OAuth2 (fixed callback port 9999)
  ├─ update/           → GitHub release check + self-update
  ├─ config/           → viper persistence
  └─ curl/             → HTTP via the `curl` binary (+ DNS-pinned GetResolved)
source/                → provider-agnostic data model (Anime/Episode/Stream/Season)
extractor/             → embed-page → stream resolvers — **currently dead code**
```

### 1.3 Application model

One model struct, one `Update` switch, one `View` function.

- `SearchModel` — `tui/search.go:44-81`. Holds *everything*: screen state, both lists, metadata caches, provider clients, settings, tracking, kitty image id, width/height.
- `Init()` returns `nil` (`tui/search.go:153-155`) — no startup command.
- `Update(msg)` — `tui/search.go:157-793`. A single flat `switch msg := msg.(type)` handling: `tea.WindowSizeMsg`, `spinner.TickMsg`, `progressTickMsg`, `tea.KeyMsg`, and 16 app messages.
- `View()` — `tui/views.go:15-38`. Returns `tea.NewView(m.renderChrome(content))` with `v.AltScreen = true`.
- Screens are a flat enum, not a stack (`tui/models.go:11-17`):
  `searchState`, `detailState`, `confirmQuitState`, `settingsState`, `anilistLoginState`.
- `RunSearch()` — `tui/search.go:908-918`. `tea.NewProgram(model).Run()`, returns `model.selectedResult`.

### 1.4 Message contracts (`tui/messages.go`)

`SearchResultsMsg` · `IDsResolvedMsg` · `MALIDsResolvedMsg` · `SearchErrorMsg` · `MetadataFetchTriggered` · `metadataDebounceMsg` · `episodeMetadataDebounceMsg` · `MetadataLoadedMsg` · `MetadataBatchLoadedMsg` · `EpisodesLoadedMsg` · `EpisodeMetadataFetchTriggered` · `EpisodeMetadataLoadedMsg` · `PlayStreamMsg` · `TUIErrorMsg` · `progressTickMsg` · `CoverImageLoadedMsg` · `TrackingUpdateMsg` · `TrackingStatusLoadedMsg` · `AniListLoginMsg` · `UpdateCheckMsg` · `UpdateCompleteMsg`

---

## 2. Actual project directory tree

```
anilix/
├── main.go                          # config.Setup() + cmd.Execute()
├── AGENTS.md                        # agent instructions (new)
├── README.md · DISCLAIMER.md · LICENSE
├── go.mod · go.sum · .goreleaser.yml · .gitignore
├── install.sh · install.bat
├── .github/workflows/ci.yml · release.yml
├── assets/anilix.svg
├── docs/architechture.md
│
├── cmd/                             # cobra CLI
│   ├── root.go                      # rootCmd, tuiCmd, runTUI
│   ├── sources.go                   # "sources" (lists registered providers)
│   ├── version.go                   # "version"
│   └── anilist.go                   # "anilist login|logout|status"
│
├── config/config.go                 # viper wrapper; ~/.anilix/anilix.toml
│
├── source/                          # data model + Source interface
│   ├── source.go  anime.go  episode.go  stream.go  episode_test.go
│
├── provider/
│   ├── provider.go                  # Provider struct + registry (All/Get/Register)
│   ├── init.go                      # comment only
│   ├── jikan/                       # jikan.go client.go types.go init.go + tests
│   ├── anilist/                     # anilist.go client.go tracker.go types.go init.go? (none)
│   └── hianime/                     # hianime.go client.go blob.go timeouts.go init.go + tests
│
├── tui/                             # 4,086 lines — the UI layer
│   ├── models.go        (119)       # states + view-model structs
│   ├── search.go        (918)       # SearchModel, Init, Update, resize, RunSearch
│   ├── views.go         (885)       # all rendering
│   ├── commands.go      (870)       # tea.Cmd async work
│   ├── items.go         (151)       # list items + bubbles list factories
│   ├── messages.go      (111)       # message types
│   ├── keymap.go        (113)       # key bindings
│   ├── theme.go         (218)       # colours + shared render helpers
│   ├── coverimage.go    (326)       # image protocol detection + rendering
│   ├── kittyimg.go      (228)       # Kitty graphics APC sequences
│   ├── recent.go         (43)       # recent searches persistence
│   ├── commands_test.go  (~90)      # live search-flow test
│   └── style/
│       ├── style.go      (46)       # thin lipgloss wrappers (mostly unused)
│       └── extra.go      (~20)      # Tag()
│
├── player/                          # play.go detect.go proxy.go ani-skip.lua + tests
├── aniskip/client.go
├── auth/auth.go
├── update/update.go
├── version/version.go
├── curl/                            # curl.go (Get/Post/PostRaw) + dns.go (GetResolved)
└── extractor/                       # 9 extractors + aes.go + m3u8.go — NOT IMPORTED by the app
```

---

## 3. Existing screen and component inventory

### 3.1 Screens

| Screen | View fn | Entry | Exit |
|---|---|---|---|
| Search | `viewSearchState` `views.go:40` | default | `Enter` → detail |
| Detail | `viewDetailState` `views.go:235` | `Enter` on a result | `Esc` → search |
| Quit confirm | `viewConfirmQuit` `views.go:586` | `Ctrl+C` | `y`/`Enter` quit, `n`/`Esc` back |
| Settings | `viewSettings` `views.go:635` | `Ctrl+S` | `Esc` → `prevState` |
| AniList login | `viewAniListLogin` `views.go:741` | `Enter` on AniList row | `Esc` → settings |

### 3.2 Chrome (shared frame)

`renderChrome` — `views.go:781-869`:
- Title bar: gradient "ANILIX" (`gradientTitle`), sub/dub pill (`renderSubDubSwitch`), quality text, AniList user indicator (`● username`) — right-aligned by manual space padding.
- Optional back line (`← esc back`) in detail state.
- Content area, padded to fill the remaining height.
- Progress bar (`progress.ViewAs`) when `searchState.Loading || episodeState.Loading || episodeState.Playing`.
- Help bar: only for `confirmQuitState` and `settingsState`; list states use the bubbles list's own help bar.

### 3.3 Layout panels

| Panel | Function | Content |
|---|---|---|
| Search bar | `views.go:44-51` | `gradientPopupBox(textInput.View(), width, 1)` |
| Results list | `makeSearchList` `items.go:81` | bubbles `list` + default delegate, left thick border on selection |
| Metadata preview | `renderMetadataPreview` `views.go:116` | cover art, title/english/native, score badge, info blocks, stats, genre tags, gradient rule, truncated synopsis, source line |
| Detail left | `renderDetailLeftPanel` `views.go:299` | cover art, score badge, stats, info blocks, genre tags |
| Detail right | `renderDetailRightPanel` `views.go:387` | episode list + episode metadata |
| Detail narrow | `renderDetailSingleColumn` `views.go:478` | single column fallback |
| Recent searches | `renderRecentSearches` `views.go:763` | header + ▸ cursor row |
| Episode list | `makeEpisodeList` `items.go:119` | bubbles list |

### 3.4 Shared widgets & theme

`tui/theme.go` — one global mutable `Theme` struct (`theme.go:12-39`):
`Primary #00d4ff`, `Secondary #9d4edd`, `Border #444444`, `Text #e4e4e7`, `Faint #666666`, `Success #10b981`, `Warning #f59e0b`, `Error #ef4444`, plus a 6-stop `Gradient` (cyan→sky→indigo→violet→fuchsia→rose).

Helper renderers, all in `theme.go`:
`gradientTitle` (per-character gradient) · `gradientLine` · `gradientPopupBox` (hand-built rounded border, per-character colour) · `renderSubDubSwitch` (pill) · `scoreBadge` · `infoBlock` · `genreTag` · `coverPlaceholder` · `truncateTitle` · `statLine`.

`tui/style/` — `New`, `NewColored`, `Fg`, `Bg`, `Truncate`, `Faint`, `Bold`, `Italic`, `Underline`, `Tag`. Effectively unused by the views (which call `lipgloss.NewStyle()` directly).

### 3.5 Existing features that must be preserved

1. Two-column search with live metadata preview (`views.go:56-72`).
2. Batched metadata — one AniList call for all results → instant j/k (`commands.go:159-201`).
3. Detail view with episode browser + per-episode Jikan metadata (title, aired, score, filler/recap, synopsis, duration).
4. Episode-number filtering: typing in the detail state filters the episode list (`filterEpisodesByNumber` `search.go:867`); `Enter` jumps to a number (`selectEpisodeByNumber` `search.go:891`).
5. Sub/Dub toggle `Ctrl+T`, persisted to `translation_type` (`search.go:232-239`).
6. Quality presets best→1080p→720p→480p→360p→auto, persisted to `quality` (`search.go:289-305`).
7. AniSkip intro/outro auto-skip via the embedded mpv Lua script (`player/ani-skip.lua`).
8. AniList OAuth login/logout + automatic progress tracking on playback (`commands.go:633-675`).
9. Recent searches (last 10, `~/.anilix/history.json`) with a selectable cursor (`recent.go`).
10. Cover art in the terminal: Kitty graphics, half-block, or chafa (`coverimage.go`).
11. Self-update from the settings popup (`commands.go:795-824`, `update/`).
12. Android playback via local proxy + player detection (`player/proxy.go`, `player/detect.go`).
13. Loading spinner + animated progress bar.
14. Quit confirmation dialog.
15. Background ID resolution + metadata prefetch (`resolveIDs`, `enrichMALIDs`, `prefetchMetadata`).

---

## 4. Navigation and keyboard interaction analysis

### 4.1 Bindings

`keymap.go:18-61`:

| Key | Action | Scope |
|---|---|---|
| `↑`/`k`, `↓`/`j` | navigate | list |
| `Enter` | search / select / confirm | context |
| `Esc` | back / clear input | context |
| `Ctrl+C` | quit (list's default `q` overridden — `items.go:108`) | global |
| `Ctrl+T` | toggle sub/dub | global |
| `/` | focus search input | global |
| `Ctrl+S` | settings | search state (focused `search.go:240`, unfocused `search.go:438`) |
| `y`/`Enter`, `n`/`Esc` | confirm quit | quit dialog |
| `↑↓`/`←→`/`h`/`l`/`Enter`/`Esc` | settings rows | settings |

### 4.2 Navigation model

Flat, single-level: **search → detail**, with two modal overlays (quit confirm, settings) and one transient (AniList login). There is no screen stack, no drawer, no back-stack semantics — `Esc` behaviour is hardcoded per state (`search.go:382-393`).

### 4.3 Interaction details worth preserving

- The text input is **reused for two different jobs**: the search query in search state, and the episode filter in detail state (`search.go:252-277`). `Enter` in detail state selects an episode number instead of searching.
- `Ctrl+T` in detail state re-fetches the episode list for the new translation (`search.go:417-429`).
- List navigation triggers a debounced metadata fetch (`metadataDebounceMsg`, `episodeMetadataDebounceMsg`).
- `m.textInput.Focused()` gates a whole key branch (`search.go:185`, `252`).

---

## 5. Existing image-rendering capabilities

### 5.1 Protocol detection (`coverimage.go:39-66`)

| Protocol | Detection | Rendering |
|---|---|---|
| `ProtocolKitty` | `KITTY_WINDOW_ID` set, or `TERM_PROGRAM` ∈ {kitty, ghostty}, or `TERM` contains kitty/ghostty (`coverimage.go:69-82`) | Unicode placeholder cells (`U+0010EEEE` + 17 combining marks, `kittyimg.go:17-57`); image transmitted via APC `tea.Raw()` (`kittyimg.go:227`) |
| `ProtocolHalfBlock` | `COLORTERM` = truecolor/24bit and `TERM` not in {vt100,vt220,linux,dumb,""} (`coverimage.go:85-96`) | SGR colours + `▀` char, max 30 rows (`coverimage.go:256`) |
| `ProtocolChafa` | `chafa` binary on PATH (`coverimage.go:98-101`) | `chafa --size=WxH --format=symbols --color-space=rgb --work=9 --align=left` (`coverimage.go:225-251`) |
| `ProtocolNone` | otherwise | no cover art |

Detection is cached for the session in package-level vars (`coverimage.go:31-32`).

### 5.2 Pipeline

`downloadCoverCmd` (`commands.go:204-225`) → `DownloadCoverImage` (curl → disk) → `RenderCoverImageKitty` → `CoverImageLoadedMsg{Rendered, KittyTransmitSeq, KittyImageID, Index}` → the model stores the rendered string in `MetadataPanel.CoverImage` and emits `tea.Raw(seq)` (`search.go:710-717`).

- **Disk cache**: `~/.anilix/cache/covers/<sha256(url)[:8] hex>.ext` (`coverimage.go:104-117`).
- **Scaling**: CatmullRom via `xdraw` (`coverimage.go:294-318`), aspect-preserving.
- **Kitty geometry**: cols = width/3 clamped 20..50, rows = cols×3/2, pixel resolution capped at 1200 (`coverimage.go:186-222`).
- **Cleanup**: `DeleteKittyImageCmd(id)` emits an APC delete; triggered on leaving detail state (`search.go:387-391`, `399-403`). The id lives on the model (`m.kittyImageID`).
- **Truncation**: covers are clipped to `(termHeight-4)*3/4` rows via `truncateToRows` (`views.go:131`, `303`).

---

## 6. Identified UI/UX problems

**Theme & visual language**
1. No token system. `Theme` is a flat global struct of 8 semantic-ish colours + a gradient. Material 3 needs role pairs (surface/onSurface, primary/onPrimary/primaryContainer) and elevation tiers — none exist.
2. Gradient is applied to nearly everything (title, borders, rules, badges) via per-character styling. `gradientPopupBox` (`theme.go:69-119`) builds a border one styled character at a time — O(width) lipgloss allocations per frame, and visually noisy against an M3 surface model.
3. No shape scale (corner radii), no spacing scale, no type scale, no elevation model.
4. No light/dark variants; colours are hardcoded hex literals with no contrast system.

**Layout**
5. The 80-column breakpoint is duplicated (`views.go:89`, `views.go:261`) and ratios are hardcoded (55/45 search, 35/65 detail). No shared breakpoint constant.
6. Magic numbers in `resize` (`search.go:798-813`): `padX=4`, `padY=2`, `usedHeight=7`.
7. Popups have a fixed width of 44 columns (`views.go:620`, `views.go:724`, `views.go:745`) with no clamp to terminal width → overflow on a 40-column Termux terminal.
8. Narrow mode (<80 cols) simply *drops* the metadata preview; there is no compact/phone layout, no drawer, no menu affordance — Termux users (40–60 cols) get the degraded path almost always.
9. `renderMetadataPreview` (`views.go:116`) and `renderDetailLeftPanel` (`views.go:299`) re-implement the same sections (cover, score, info blocks, stats, genre tags) with different ordering — the exact duplication a redesign must avoid.

**Components & structure**
10. `tui/style/` is a near-duplicate of what lipgloss already offers and is barely used — dead weight or an unfinished abstraction.
11. The settings screen is bespoke: rows are hardcoded in the view (`views.go:649-733`) and the cursor is a magic int `0..3` (`models.go:87`). Adding a row requires edits in the model, the view, and the key handler (`search.go:279-341`).
12. No screen hierarchy — a hamburger drawer cannot be expressed in a flat enum; `prevState` is a single slot (`models.go:50`).
13. `m.textInput` serves as both the search field and the episode filter — two roles, one widget, so both screens' chrome is coupled to one bubbles component.
14. List items are plain strings (`items.go:20-53`) with `%-40s` padding — no hierarchy, no icons, no secondary/tertiary text styling beyond the delegate defaults.

**Interaction & feedback**
15. No mouse support anywhere (no `tea.MouseMsg` handling) — clicking is impossible.
16. Recent-search rows show a `▸` cursor but there is no visible key hint and no way to delete entries.
17. Loading states are inconsistent: `searchState.Loading`, `episodeState.Loading` and `episodeState.Playing` all drive the same progress bar (`views.go:851`); the settings dialog builds fresh key bindings on **every frame** (`views.go:833-843`).
18. Progress animates by a fixed `+0.08` per 100 ms tick (`search.go:170-182`) — not time-based, so it looks different at different frame rates.
19. Error states are one-line plain text (`views.go:76`); there is no retry affordance and no distinction between "no results", "network error" and "provider blocked" (e.g. hianime's Cloudflare message).
20. The help bar differs per screen and the global keymap's `ShortHelp` omits `Toggle` and `Search` (`keymap.go:63-65`) while the list injects them via `AdditionalShortHelpKeys` (`items.go:113`) — inconsistent discoverability.

---

## 7. Technical limitations

1. **Cell grid only.** No widgets, no rounded/elevation primitives beyond box-drawing characters; no shadow, no blur, no real ripple/hover.
2. **Colour depth.** Cover art needs truecolor; `COLORTERM` unset → no image at all. No 256-colour fallback for images.
3. **Kitty protocol is terminal-specific.** Ghostty/Kitty only; the Unicode-placeholder trick depends on 17 specific combining marks (`kittyimg.go:17-57`) and font support. Other terminals get the blocky half-block path.
4. **Kitty lifecycle is delicate.** Placeholder cells + `tea.Raw()` APC sequences + explicit deletion (`kittyImageID`). A re-render or state change that forgets to delete leaves ghost images in the terminal. Any layout rewrite must preserve this.
5. **Bubble Tea v2 ultraviolet renderer** re-renders the full frame each tick; hand-computed widths must be exact or output wraps. The current code manually pads to `innerWidth` (`theme.go:107-114`) — an off-by-one shows up as a broken border.
6. **Alt screen only.** No inline mode → no native scrollback, no copy-paste of results, no tmux-style window reuse.
7. **No mouse, no OSC 8 hyperlinks, no clipboard integration** (the bubbles clipboard dependency is present but unused by the app).
8. **No i18n / no width-aware text.** `runewidth` handles CJK, but layout maths assumes single-width glyphs; the theme helpers use `len(text)` (bytes) in `gradientTitle` (`theme.go:45`) — a multi-byte title colours incorrectly.
9. **Terminal width on Termux is typically 40–60 cols** → the existing two-column design is unavailable on the primary mobile target; the redesign must be compact-first.
10. **Provider fragility.** hianime markup can change (it already did once during this work), and Jikan's API goes down for hours — the UI must keep working when metadata is missing.
11. `extractor/` is **not imported by any non-test file** — the app currently resolves streams entirely inside `provider/hianime`. Any UI work that "reuses" extractors would be reviving dead code.

---

## 8. Recommended redesign architecture

### 8.1 Principles

- Keep Bubble Tea v2, lipgloss v2, bubbles v2. No framework change.
- **Tokens first, pixels second.** Introduce M3 roles mapped onto the cyberpunk palette so the identity survives.
- **Components are pure functions**: `func(data, width) string`. No model access, no globals → testable and reusable in both layouts.
- **One responsive system**, one breakpoint source of truth.
- **Preserve every behaviour** in §3.5; the redesign is presentation-layer only.

### 8.2 Proposed package layout (additive, no duplication)

```
tui/
├── theme/                    ← NEW: replaces theme.go's globals
│   ├── tokens.go             # M3 colour roles (dark scheme) + cyberpunk accents
│   ├── shape.go              # radius scale, spacing scale (4/8/12/16/24)
│   ├── type.go               # type scale: display/title/body/label + weights
│   └── elevation.go          # border/shadow levels per surface tier
├── ui/                       ← NEW: the component library
│   ├── appbar.go             # top app bar (title, sub/dub, quality, account)
│   ├── drawer.go             # hamburger navigation drawer
│   ├── searchfield.go        # M3 search field
│   ├── resultlist.go         # list item: title / secondary / trailing badge
│   ├── metapanel.go          # metadata preview + detail panel (ONE component)
│   ├── episodelist.go        # episode rows, selected state
│   ├── settings.go           # settings rows (data-driven, not hardcoded)
│   ├── dialog.go             # confirm dialog
│   ├── chips.go              # genre chips / status badges
│   ├── empty.go              # empty + error + retry states
│   ├── progress.go           # indeterminate/determinate bar
│   └── layout.go             # Breakpoint(Compact|Medium|Expanded) + helpers
├── style/                    ← either adopted by ui/ or deleted (do not keep both)
└── theme.go                  ← thin façade over theme/ during migration
```

### 8.3 Token model (Material 3, dark scheme, cyberpunk accents)

| M3 role | Value | Use |
|---|---|---|
| `surface` | `#0d0d12` | app background |
| `surfaceContainer` | `#14141c` | panels, list |
| `surfaceContainerHigh` | `#1c1c26` | selected rows, sheets |
| `onSurface` | `#e4e4e7` | primary text (keeps current Text) |
| `onSurfaceVariant` | `#9a9aa8` | secondary text (replaces `#666666`) |
| `primary` | `#00d4ff` | accent — current Primary |
| `onPrimary` | `#001318` | text on accent |
| `primaryContainer` | `#00363f` | accent chips |
| `onPrimaryContainer` | `#8ee9ff` | |
| `secondary` | `#9d4edd` | current Secondary |
| `tertiary` | `#d946ef` | gradient's violet/fuchsia tier |
| `outline` | `#3a3a48` | dividers, borders |
| `error` | `#ef4444` | current Error |
| `success` | `#10b981` | current Success |

The 6-stop gradient survives as the **brand** accent (app bar logo, active indicator, focus ring), not as a border treatment.

### 8.4 Screen & navigation model

Replace the flat `tuiState` enum with a small stack so overlays and a drawer are expressible:

```go
type Screen int
const (ScreenSearch, ScreenDetail, ScreenSettings, ScreenQuit, ScreenAniListLogin)

type Nav struct {
    stack   []Screen      // search → detail
    drawer  bool          // overlay, does not push
    modal   bool          // settings/quit block input
}
```

Key additions: `m` (or `Ctrl+D`) toggles the drawer; `Esc` pops the stack (drawer → close, detail → search); the drawer hosts Search / Recent / Settings / Account / Update destinations.

### 8.5 Breakpoints

```go
type Breakpoint int
const (Compact Breakpoint = iota; Medium; Expanded)

func BreakpointOf(w int) Breakpoint {
    switch {
    case w < 60:  return Compact   // Termux phone
    case w < 100: return Medium
    default:      return Expanded
    }
}
```

- **Compact**: single column, app bar + search field + list; metadata and episode details open as full-screen sheets; drawer as the primary navigation.
- **Medium**: list + narrow preview (like today's 80–100 band).
- **Expanded**: list + wide preview/detail, exactly today's two-column feel.

---

## 9. Files likely to require modification

| File | Change | Risk |
|---|---|---|
| `tui/theme.go` | split into `tui/theme/` + façade | low |
| `tui/style/style.go`, `style/extra.go` | adopt into `tui/ui/` or delete | low |
| `tui/views.go` | rewrite to compose `ui/` components | **high** — 885 lines, all rendering |
| `tui/items.go` | custom delegate for the new list item | medium |
| `tui/models.go` | `Nav` stack, drawer state, data-driven settings rows | medium |
| `tui/search.go` | `Update()`: drawer keys, new state transitions, `resize()` against the new chrome | **high** — 918 lines, the whole event loop |
| `tui/keymap.go` | add nav/drawer bindings; keep all existing ones | low |
| `tui/messages.go` | add drawer/sheet messages only; **do not change existing contracts** | low |
| `tui/commands.go` | new commands for sheet open/close; keep provider calls untouched | medium |
| `tui/coverimage.go`, `tui/kittyimg.go` | wrap in a `ui.CoverArt` component; keep protocol logic | medium |
| `tui/recent.go` | support delete/clear for the drawer | low |
| `README.md` | update screenshots + keybindings + feature table | low |
| `docs/architechture.md` | update the TUI section | low |
| `cmd/`, `provider/`, `player/`, `aniskip/`, `auth/`, `update/`, `config/`, `curl/`, `source/` | **unchanged** | — |

New files: `tui/theme/*` (4), `tui/ui/*` (~11).

---

## 10. Potential regression risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | **Message-contract breakage.** Changing `EpisodesLoadedMsg` / `MetadataBatchLoadedMsg` shapes silently breaks provider/command code. | Freeze `tui/messages.go`; treat existing messages as a public API of the TUI layer. |
| R2 | **Kitty image leaks.** Orphaned APC images stay painted in the terminal until resize. | Keep `kittyImageID` + `DeleteKittyImageCmd` on *every* state transition, including new drawer/sheet transitions; add a regression check. |
| R3 | **Off-by-one width bugs** in the hand-rolled borders (`gradientPopupBox`) and new components → wrapped frames, broken borders. | Make all components pure `width → string` functions and unit-test the width maths; use `lipgloss.Width()` for measurement, never `len()`. |
| R4 | **Compact-layout regression on Termux.** Users on 40–60 cols currently get a usable single-column list; a new app bar + drawer could eat the vertical budget. | Design Compact first; verify at 40×20; keep the episode list scrollable with the list's own pagination. |
| R5 | **Settings persistence breakage.** `config.Set` writes `quality`, `translation_type`, `aniskip.enabled` immediately; changing the settings UI must keep the same keys or users silently lose settings. | Keep the exact key names; assert them in a test. |
| R6 | **Player selection depends on `NeedsReferrer`** (mpv vs vlc-android) — a stream-model change breaks Android playback. | Do not touch `source.Stream`; keep `NeedsReferrer` semantics. |
| R7 | **Tracking breaks if MALID stops being filled** (AniList `ResolveByTitles` + hianime embed `/mal/<id>/`). | Keep the background ID-resolution commands and their messages intact. |
| R8 | **Self-update replaces the running binary.** A malformed UI-driven update path could brick installs. | Do not touch `update/` or its messages; keep the "Restart to apply" wording. |
| R9 | **`tea.Raw()` interleaving.** APC sequences must stay outside the ultraviolet redraw path; moving them into a component could corrupt the frame. | Keep `tea.Raw` emission in the model/command layer, not inside view components. |
| R10 | **Behaviour loss during refactor.** Episode filtering, `Enter`-to-jump, Ctrl+T re-fetch, quit confirmation, recent-search cursor are easy to drop. | Keep the §3.5 checklist as an explicit per-phase acceptance list. |
| R11 | **Over-engineering the component layer** — a large `ui/` package used by one screen can become a second framework. | Keep components small and stateless; no interfaces unless a second implementation exists. |
| R12 | **Unicode width bugs** — `gradientTitle` colours by byte index (`theme.go:45`), breaking CJK titles. | Colour by rune index; use `runewidth` for measurement. |

---

## 11. Proposed phase-by-phase implementation plan

**Phase 1 — Foundations (no visual change)**
- Create `tui/theme/` (tokens, shape, type, elevation) and `tui/ui/` with a *minimal* primitive set: `Chip`, `Badge`, `Divider`, `SectionHeader`, `EmptyState`, `ErrorState`, `ProgressBar`.
- Add `ui/layout.go` with `BreakpointOf` and width helpers.
- `tui/theme.go` becomes a façade over the new tokens (same colours at first).
- **Gate:** `go vet ./...` + `go test -short ./...` green; the search view renders byte-identical to today.
- **Risk:** R3 (width maths) — mitigated by unit tests on the new primitives.

**Phase 2 — Search screen**
- Compose `AppBar` + `SearchField` + `ResultList` + `MetaPanel` from `ui/`.
- Replace the gradient border with a single-colour `outline`/`primary` focus ring; keep the gradient only for the "ANILIX" wordmark.
- Metadata preview and detail panel become **one** `MetaPanel` component (fixes duplication #9).
- **Gate:** every §3.5 behaviour for search still works; batch metadata still one request.

**Phase 3 — Detail screen**
- `EpisodeList` with a proper selected state, `EpisodeSheet` for per-episode metadata.
- Compact mode renders the episode sheet as a full-screen overlay.
- **Gate:** filtering, jump-to-episode, Ctrl+T re-fetch, AniSkip, cover art all intact.

**Phase 4 — Dialogs & sheets**
- `SettingsSheet` with **data-driven rows** (model exposes `[]SettingRow`), fixing the magic-int cursor.
- `ConfirmDialog` for quit.
- Clamp all overlays to `min(44, width-4)`.
- **Gate:** settings still persist the same three config keys (R5).

**Phase 5 — Navigation drawer**
- Introduce `Nav` stack in `tui/models.go`; `m`/`Ctrl+D` toggles the drawer.
- Drawer destinations: Search, Recent (with delete), Settings, AniList account, Update.
- Wire `Esc` to pop the stack.
- **Gate:** no behaviour regression in any state; kitty cleanup on every transition (R2).

**Phase 6 — Polish & hardening**
- Empty/error/retry states, `Ctrl+R` retry on failure.
- Time-based progress animation (replace the `+0.08` tick), respecting `tea.Quit`.
- Rune-safe gradients, `runewidth`-based measurement.
- Contrast pass on the token set (WCAG-ish for onSurface/onSurfaceVariant).
- README screenshots, keybinding table, and `docs/architechture.md` updates.

**Phase 7 — Optional (only if warranted)**
- Mouse support (click to select, wheel to scroll) — currently impossible.
- OSC 8 hyperlinks for titles and source links.
- Inline (non-alt-screen) mode for tmux scrollback.

---

### Verification strategy for every phase

```
go vet ./... && go test -short ./...        # CI parity
go test ./provider/hianime -run TestIntegration   # provider untouched
go test ./tui -run TestSearchFlowLive        # end-to-end search flow
```
Plus a manual matrix: 40×20 (Termux), 80×24, 120×40, 200×50 — Compact / Medium / Expanded.

---

*End of audit. No source files were modified in Phase 0.*

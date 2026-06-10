# Resume Watching Card — Design Spec

## Problem

On the anime details page, progress shows as "11/13" but users must manually scroll through the episode list to find where they left off. This creates friction when resuming a series.

## Goal

A prominent, focusable action card above the episode list that lets users resume watching with one keypress. No searching required.

## Design

### Data Source

Progress resolution priority:
1. AniList `TrackingProgress` — if authenticated and progress > 0
2. Local history — most recent episode for this anime (fallback)
3. No progress — show "Start Watching" for episode 1

Card needs: episode number, total episode count, percentage. Episode title is optional (requires Jikan fetch, not blocking).

### Visual

Gradient-bordered box (matching existing `gradientPopupBox` style). Two lines:

```
▶ Resume Watching
  Episode 11 • 84% Complete
```

No progress:
```
▶ Start Watching
  Episode 1
```

Focused state: brighter border / primary-colored accent. Unfocused: faint/muted treatment.

### Position

Between tracking bar and episode list header in the right panel. Only renders when resume data exists.

### Navigation Model

New `detailFocus` field with zones:

| Zone | When | Keybinds |
|------|------|----------|
| `focusResumeCard` | Default on entry (if resume data exists) | Enter/r plays episode; j/k/Tab moves to episode list |
| `focusEpisodeList` | j/k from resume card | Normal episode navigation |
| `focusSearchBar` | / or digit key | Existing filter behavior |

When no resume data: focus skips directly to episode list (current behavior).

### Keybinds (on resume card focus)

- **Enter / r** — triggers `playEpisode()` with the resume episode
- **j/k or Tab** — moves focus to episode list
- **Esc** — back to home/search (existing behavior)

### State Changes (`models.go`)

Add to `EpisodeState`:
- `resumeEpisode int` — episode number to resume (0 = no resume)
- `resumeFocus bool` — whether resume card has focus

### Rendering (`views.go`)

Insert `renderResumeCard()` call in right panel between tracking bar and episode list header.

### Key Handling (`search.go`)

- On entering detail state: set `resumeEpisode` from tracking/history, default `resumeFocus = true`
- Route Enter key based on `resumeFocus`
- Add `r` keybind on resume card focus

### Files to Modify

| File | Change |
|------|--------|
| `tui/models.go` | Add `resumeEpisode`, `resumeFocus` to `EpisodeState` |
| `tui/views.go` | Add `renderResumeCard()`, insert in `renderDetailRightPanel()` |
| `tui/search.go` | Set resume state on detail entry, route keybinds |
| `tui/keymap.go` | Add `r` keybind for resume |

### Out of Scope

- Intra-episode position tracking (requires mpv IPC or state file)
- Auto-advance to next episode after playback
- Resume card on home screen (Continue Watching cards already handle that)

# Anilix Design System

**Status:** Complete — every screen (search, detail, history, watchlist,
settings, AniList, dialogs, navigation drawer) draws itself from the
tokens of `tui/theme` and the components of `tui/ui`, including the
polish pass: empty/error/retry states, time-based progress and the
contrast check.
**Scope:** every pixel the TUI draws. Screens, providers and playback are out of scope.

This document is the specification. The code in `tui/theme` and `tui/ui` is its
implementation, and `anilix theme` renders it as a live showcase inside the
application.

---

## 1. Principles

1. **Tokens first, pixels second.** A screen never names a colour. It asks for a
   *role* — `theme.Primary`, `theme.SurfaceVariant` — and the theme decides
   what that role looks like on the terminal it is running on.
2. **Material Design 3.** Material 3's role model (surface / onSurface,
   primary / onPrimary / primaryContainer) and dark baseline palette provide
   both the structure and the visual character: modern, clean, comfortable
   to read, and free of decorative clutter.
3. **A terminal has one font size.** Hierarchy is expressed with weight,
   colour, case and spacing — never with a point size. Every "bigger" element
   is a different *role*, not a bigger font.
4. **One source of truth.** Spacing, colours, borders and breakpoints live in
   the theme package. Screens compose; they do not decide.
5. **Degrade, never break.** A terminal with 16 colours, or none at all, must
   still show a complete, readable application.
6. **Components are pure functions.** `func(data, width) string`. No model
   access, no globals beyond the tokens, deterministic output. That is what
   makes them testable and reusable across layouts.

---

## 2. Architecture

```
tui/theme/     tokens, typography, spacing, shape, capability detection, icons
tui/ui/        components, layout helpers, state components
tui/shell.go   the application shell: app bar, content region, footer
tui/nav.go     the navigation drawer and its destinations
tui/*.go       screens, composed from ui components and theme roles
```

| Package | Owns | Must not |
|---|---|---|
| `tui/theme` | Colour roles, type scale, spacing scale, elevation, icon set, terminal capability detection | Render whole components |
| `tui/ui` | Buttons, chips, cards, lists, menus, tabs, dialogs, input fields, empty/error states, breakpoints | Know about the model, providers or network state |
| `tui/shell.go`, `tui/nav.go` | The frame of the application and the destinations of the drawer | Decide how a component looks |

### Using the system

```go
// A screen asks a component for a string. It never builds a style itself.
row := ui.ListItem{
    Title:       anime.Name,
    Description: "TV · 2023 · 28 eps",
    Trailing:    ui.Badge{Label: "★ 9.03"}.Render(),
    Selected:    isCursor,
    Focused:     listHasFocus,
}.Render(panelWidth)
```

```go
// A screen that needs a colour asks for a role.
title := theme.Title.Render(anime.Name)
```

**Hard rules**

- No `lipgloss.Color("#…")` outside `tui/theme`. A literal colour in a screen
  is a bug.
- No `lipgloss.NewStyle()` in a screen for anything visual. Screens use
  `ui` components and `theme` roles.
- Every component that takes a width returns exactly that width. The width
  contract is covered by tests in `tui/ui/ui_test.go`.

---

## 3. Colour tokens

Colour is expressed as **roles**, never as hues. A role answers "what is this
for?", so a palette change never requires touching a screen.

### 3.1 Surfaces

| Role | Value | Use |
|---|---|---|
| `background` | `#111318` | The canvas behind everything |
| `surface` | `#1C2028` | Cards, panels, list containers |
| `surface-variant` | `#20242C` | Selected rows, raised controls, chips (surface container) |
| `surface-high` | `#292D36` | Dialogs, drawers, menus (surface container high) |
| `surface-low` | `#171B21` | Wells, input fields, recessed areas |
| `scrim` | `#0A0C10` | Dimming behind a modal |

### 3.2 Accents

| Role | Value | Use |
|---|---|---|
| `primary` | `#D0BCFF` | Focus, selection, primary action |
| `on-primary` | `#381E72` | Text on `primary` |
| `primary-hover` | `#DCC7FF` | Hover state of a primary action |
| `primary-pressed` | `#B69EFF` | Pressed state of a primary action |
| `primary-container` | `#4F378B` | Chips, badges, tonal fills |
| `on-primary-container` | `#EADDFF` | Text on `primary-container` |
| `secondary` | `#CCC2DC` | Supporting accent: quality, status, "live" |
| `on-secondary` | `#332D41` | Text on `secondary` |
| `secondary-container` | `#33304A` | Tonal fills |
| `on-secondary-container` | `#E8DEF8` | Text on `secondary-container` |
| `tertiary` | `#EFB8C8` | Third accent, used sparingly (scores, favourites) |
| `on-tertiary` | `#492A43` | Text on `tertiary` |
| `tertiary-container` | `#633B48` | Tonal fills |
| `on-tertiary-container` | `#FFD8E4` | Text on `tertiary-container` |

### 3.3 Status

| Role | Value | Use |
|---|---|---|
| `error` | `#F2B8B5` | Failures, destructive actions |
| `on-error` | `#601410` | Text on `error` |
| `error-container` | `#8C1D18` | Error chips and banners |
| `on-error-container` | `#F9DEDC` | Text on `error-container` |
| `success` | `#86D9A4` | Confirmations, connected accounts |
| `on-success` | `#003919` | Text on `success` |
| `warning` | `#F5C26B` | Cautionary states |
| `on-warning` | `#3A2B10` | Text on `warning` |

### 3.4 Text and lines

| Role | Value | Use |
|---|---|---|
| `text` | `#E6E1E5` | Primary reading text (on-surface) |
| `muted` | `#CAC4D0` | Secondary text, descriptions (on-surface-variant) |
| `disabled` | `#6A6E7A` | Unavailable content, hints, captions |
| `outline` | `#938F99` | Container borders, strong dividers |
| `outline-variant` | `#262B33` | Subtle dividers, card borders |

`role-unset` (`theme.RoleUnset`) is the zero value of a role: it means "no
role". Components with an optional colour use it so that a zero-valued struct
field never renders an accidental colour.

### 3.5 Contrast

The palette is checked against WCAG ratios by `tui/theme/theme_test.go`:

| Pair | Minimum |
|---|---|
| `text` on `background` / `surface` | 7.0 : 1 |
| `muted` on `surface` | 4.5 : 1 |
| `primary` on `background` | 4.5 : 1 |
| `on-*` on its container | 4.5 : 1 |
| `error` on `surface` | 4.5 : 1 |
| `disabled` on `surface` | 2.0 : 1 |

A new role must be added to that table. A role that cannot meet its minimum
is a role that does not ship.

---

## 4. Typography

Ten steps. Each is a *role*, not a size.

| Step | Treatment | Use |
|---|---|---|
| `display` | Bold, `primary` | The `ANILIX` wordmark in the app bar |
| `headline` | Bold, `text` | Screen titles, dialog titles |
| `title` | Bold, `text` | Item and section titles |
| `body-strong` | Bold, `text` | Emphasised body text |
| `body` | `text` | Default reading text |
| `label` | `muted` | Metadata paired with a value |
| `label-strong` | Bold, `muted` | Emphasised metadata labels |
| `caption` | Faint, `disabled` | Hints, timestamps, counts, key hints |
| `overline` | Bold, `primary`, uppercase | Section eyebrows |
| `code` | `muted` | URLs, ids, technical values |

**Rules**

- Upper case is reserved for `overline`. Labels stay sentence case.
- One `overline` per section. It is an eyebrow, not a heading.
- Faint (`caption`) is for information the user may skip. Never for
  information they need.
- A title is never longer than two lines; truncate rather than shrink.

---

## 5. Spacing and sizing

### 5.1 The scale

A 4dp rhythm rounded to whole cells. Terminals have no fractional spacing,
so the scale snaps to 1, 2, 4, 8, 12, 16, 24, 32 cells.

| Token | Cells | Use |
|---|---|---|
| `space1` | 1 | Inside a single line |
| `space2` | 2 | Between related lines, between inline elements |
| `space3` | 4 | Between blocks inside a section |
| `space4` | 8 | Between sections |
| `space5` | 12 | Between major groups |
| `space6` | 16 | Screen padding |
| `space7` | 24 | Generous separation |
| `space8` | 32 | Hero spacing |

`theme.Gap(n)` returns a run of spaces; `ui.Row(items, gap)` joins inline
elements with a gap so a row of buttons or chips always shares one rhythm.

### 5.2 Control sizes

| Size | Height | Use |
|---|---|---|
| `compact` | 1 row | List rows, chips, badges |
| `medium` | 3 rows | Buttons, inputs |
| `expanded` | 5 rows | Dialogs, sheets |

### 5.3 Breakpoints

The only place the codebase decides how wide a screen is.

| Tier | Width | Layout |
|---|---|---|
| `compact` | < 60 | Single column. No side panel. Drawer navigation. |
| `medium` | 60–99 | List + narrow preview (40%) |
| `expanded` | ≥ 100 | List + wide preview (45%, capped at 80) |

`ui.BreakpointOf(width)`, `ui.ListWidth(width)` and `ui.PreviewWidth(width)`
are the only helpers that may look at raw column counts. Termux is usually
40–60 columns wide, so **compact is the primary target**, not an edge case.

Overlays are clamped with `ui.OverlayWidth(desired, terminalWidth)`: a 44
column dialog becomes 36 on a 40 column terminal. A dialog never wraps.

---

## 6. Borders, separators and surfaces

### 6.1 Elevation

A terminal has no shadows, so elevation is a surface tint plus a border
strength.

| Elevation | Surface | Border | Use |
|---|---|---|---|
| 0 | `background` | `outline-variant` | Flush with the canvas |
| 1 | `surface` | `outline-variant` | Cards, panels |
| 2 | `surface-variant` | `outline` | Selected rows, raised controls |
| 3 | `surface-high` | `outline` | Dialogs, drawers, menus |

### 6.2 Shape

| Style | Drawing | Use |
|---|---|---|
| `thin` | `─ │ ┌ ┐ └ ┘` | Square containers, plain panels |
| `rounded` | `╭ ─ ╮ ╰ ╯` | Cards, dialogs, outlined buttons |
| `focus-bar` | `▌` | The focused row of a list |

### 6.3 Separators

- **Subtle** (`outline-variant`): between rows of the same group.
- **Strong** (`outline`): between sections.
- A separator is always a full-width rule of the content box — never a
  gradient, never a partial line.

> The pre-redesign code drew gradient borders cell by cell
> (`gradientPopupBox`). That is gone from the design system: a border is one
> colour, one character, one line.

---

## 7. States

Terminals have no hover, no ripple and no shadow. The state model is
therefore small and every state is expressible with what a terminal can show.

| State | Expression |
|---|---|
| `default` | The resting style of the component |
| `hover` | Accent lightens (`primary-hover`); used by mouse-capable terminals, as an emphasis state otherwise |
| `pressed` | Accent darkens (`primary-pressed`) |
| `focused` | The accent carries the border (the "focus ring") and the label stays bold |
| `selected` | Accent bar `▌` + `primary` label + `surface-variant` fill |
| `disabled` | `disabled` colour, faint attribute, no accent |

**Selected vs. focused.** They are different: *selected* means "this is the
cursor row", *focused* means "this pane holds the keyboard". A list that has
lost focus keeps its accent bar but mutes the row, so the user can always
tell where the cursor is and where the keys go.

**Disabled.** Never remove a control from the layout — a control that
disappears moves everything else. Disabled controls stay in place, dimmed.

---

## 8. Components

### 8.1 Button

| Variant | Container | Use |
|---|---|---|
| `filled` | `primary` fill, `on-primary` label | The single primary action of a screen |
| `tonal` | `secondary-container` fill | Secondary actions |
| `outlined` | Transparent, `outline` border, `primary` label | Lower-emphasis actions that need a hit target |
| `text` | Label only | Tertiary and destructive actions |

- One row tall. An outlined button draws left and right rules only — a full
  box would need three rows and break inline layout.
- Exactly one `filled` button per screen or dialog.
- Destructive actions use `text` or `outlined` with the `error` role, never a
  filled accent.

### 8.2 Chip and badge

- **Chip** — a tag: genre, format, quality. Filled with a `*-container`
  role and its `on-*-container` label.
- **Badge** — a count. Small, bold, `secondary-container` by default.
- A chip is one line tall and never wraps.

### 8.3 Card

A bordered container: title, optional subtitle, body, optional trailing line.
The width is the **outer** width — border and padding are subtracted from the
content box, so a card of width 44 renders exactly 44 cells.

### 8.4 List item

Three parts, one line each:

```
▌ Title                          trailing
  Description
```

- The accent bar marks the cursor row.
- The fill (`surface-variant`) marks that the list has focus.
- The trailing value is right-aligned to the panel edge.

### 8.5 Menu item

Icon, label, optional right-aligned hint. The selected row carries the
accent bar; a menu that lost focus mutes the row instead of dropping the bar.

### 8.6 Tab

The active tab is `primary` with a `▎` marker; inactive tabs are `muted`. A
badge shows a count in parentheses.

### 8.7 Dialog

Elevated container with a headline, body and a row of actions. Width is the
outer width and is clamped to the terminal.

### 8.8 Input field

A bordered field: the search icon, then the value or the placeholder. The
icon never changes — the focus is expressed with the border colour and the
weight of the icon, so the text never jumps.

```
╭──────────────────────────────────────────────╮
│ ⌕ one piece                                  │
╰──────────────────────────────────────────────╯
```

The field owns its frame, not its text: the value is rendered by the caller
and handed over, so an editable input and a read-only value share the same
frame and the same focus treatment.

### 8.9 Metadata rows

- **Info row** — a label and a value on one line. The label column has a
  fixed width, so a block of metadata reads as a table: the values line up
  under each other instead of drifting with the length of the labels.
- **Chip row** — the tags of an entry, wrapped to the width of the column.

### 8.10 Empty, error and loading states

- **Empty** — icon, a short statement, and what to do next.
- **Error** — the error role for the failure, `muted` for the explanation,
  and the retry hint in `disabled`. A failed screen must stay readable.
- **Loading** — a spinner with a label; a determinate bar when there is
  progress.

Never a blank screen. An empty state and an error state must never look the
same.

### 8.11 The supporting screens

History, watchlist, AniList and settings are pages of the shell, built
from the same components as the search screen.

- **History** — the played episodes as a structured list, grouped under
  the day they were played on: today, yesterday, then the date. A row
  carries the title, the episode, and how long ago it was played. The
  rows say nothing the store does not know.
- **Watchlist** — the saved list of the AniList account, fetched when the
  screen opens. The three phases of a remote screen are explicit:
  loading, error, and content, the empty ones included - a terminal that
  is not connected gets the words, not a spinner. On a terminal with
  room for two columns the selected entry gets the artwork preview.
- **AniList** — the account page: authentication status, the account
  name, the tracking switch, and the actions that exist. It does not
  promise a sync it does not have: progress is pushed after playback,
  and the page says so. Logging out asks first.
- **Settings** — grouped rows with the label in the text role and the
  value in a chip, so label and value never share a style. The groups
  follow the settings the application has: Playback, Audio &
  Subtitles, Appearance, Images, Player, AniList, Application. The
  screen is adaptive: a wide terminal gets the dialog (elevation 3,
  clamped by `ui.OverlayWidth`), a compact terminal gets a full-screen
  page that scrolls - no cramped centred dialog on a phone. The rows,
  the order and the keys are the same on both, and the configuration
  keys behind them are unchanged.

### 8.12 Artwork

The cover art is rendered by the adaptive renderer in `tui/renderer.go`;
the screen only asks `renderArtwork` for a picture of a size.

- **The chain.** A terminal gets an ordered chain of the protocols it can
  do: kitty (kitty, ghostty, WezTerm), half-blocks (truecolor), chafa,
  and at the end the text placeholder, which is not a renderer at all.
  The first link that produces a picture wins; a link that produces
  nothing once is dropped for the rest of the session.
- **Configuration.** `artwork.renderer` in `anilix.toml` forces a
  renderer (`kitty`, `sixel`, `iterm2`, `halfblock`, `chafa`) or turns
  the artwork off (`ascii`/`none`). A configured renderer is honoured
  only when the terminal actually advertises it — a converter on the
  PATH is not a terminal capability — otherwise the choice falls back to
  the detected chain. SIXEL and iTerm2 are never in the detected chain:
  they draw over the text grid of terminals that keep images outside of
  it, so they are opt-in.
- **The capability query.** The application asks the terminal what it can
  draw when it starts: the answer to the device attributes query (DA1)
  carries attribute 4 where the terminal speaks SIXEL — the way Termux
  advertises its graphics support in the builds that carry it. An answer
  that arrives after a cover was drawn redraws it with the better
  protocol; a terminal that never answers simply keeps the renderers its
  environment proves, and a Termux build from before the graphics
  support merged falls back to half-blocks.
- **Lifecycle.** Covers are downloaded once and cached on disk, the
  rendered output of the self-contained protocols is cached in memory,
  and a cover is re-rendered only when its size changes. A screen change
  deletes the image the kitty protocol left in the memory of the
  terminal. A missing cover, a failed download or a terminal without a
  protocol shows the framed placeholder, never a hole in the layout.

---

## 9. Icons

The icon set lives in `tui/theme/icons.go` and follows four rules:

1. **One cell wide.** Every glyph is exactly one cell, so it never breaks a
   column. (`icon-pause` is the single documented exception and is marked as
   two cells wide.)
2. **No emoji.** Emoji are two cells wide and render inconsistently.
3. **ASCII fallback for everything.** `theme.Icon()` returns a plain
   equivalent when the terminal cannot render unicode, and the fallback has
   the same width as the original so the layout never shifts.
4. **Named by meaning, not by shape.** A screen asks for `theme.IconSearch`,
   never for `"⌕"`.

The set: menu, menu item, search, chevrons, check, close, settings, play,
pause, episode, star, dot, warning, info, external link, lock, selection bar,
divider, tab marker and the four spinner frames.

---

## 10. Terminal capability degradation

`theme.Profile()` detects the capability of the terminal once per process;
`theme.SetProfile` overrides it for tests and for the showcase.

| Capability | Behaviour |
|---|---|
| Truecolor | Exact palette values |
| 256 colours | Palette is downsampled to the 256 colour cube |
| 16 colours | Palette is downsampled to the 16 colour palette |
| No colour | `Role.Color()` returns `nil`; components fall back to weight, case and box drawing characters |

- `theme.Fg` and `theme.Bg` never emit a colour the terminal cannot show —
  they return the style unchanged when there is no colour support.
- Unicode support is detected from the locale. Without it, box drawing
  characters degrade to ASCII (`+-|`) and icons to their fallbacks.
- `anilix theme` forces truecolor when it is piped, unless `NO_COLOR` is
  set, so the showcase stays readable in a file or a pull request.

### The reset trap

Lip Gloss closes every styled run with a reset, and **a reset also clears
the background colour**. Nesting a coloured run inside a filled block
therefore leaves a hole in the fill.

Two rules follow:

- A filled component styles **every** segment with the fill
  (`ui.styledLine`, `ui.filledLabel`). Never nest a coloured run inside a
  filled one.
- A background colour is the last thing set on a style, and a label's own
  colour is applied to the same style, not nested inside it.

---

## 11. Using the system in a new screen

```go
func (m *SearchModel) viewSomething() string {
    bp := ui.BreakpointOf(m.width)          // 1. pick the layout tier
    width := ui.ListWidth(m.width)          // 2. ask for the column widths

    var b strings.Builder                      // 3. compose components
    b.WriteString(theme.Overline.RenderUpper("Section"))
    b.WriteString("\n")
    b.WriteString(ui.ListItem{...}.Render(width))
    return b.String()
}
```

Checklist before a screen is merged:

- [ ] No colour literal outside `tui/theme`.
- [ ] Every style comes from `theme` or `ui`.
- [ ] The layout tier comes from `ui.BreakpointOf`, not from a literal
      column count.
- [ ] Overlays are clamped with `ui.OverlayWidth`.
- [ ] Components render to the exact width they were given (a test asserts
      this).
- [ ] The screen renders correctly at 40, 80, 120 and 200 columns, and with
      `NO_COLOR` set.

---

## 12. Verification

```sh
go vet ./...
go test -short ./...        # includes the token, contrast, width and state tests
go build . && ./anilix theme # the live showcase
```

Manual matrix: 40×20 (Termux), 80×24, 120×40, 200×50 — compact, medium and
expanded; plus `NO_COLOR=1` and a 16 colour terminal.

---

## 13. Roadmap

| Phase | Scope | State |
|---|---|---|
| **1 — Foundations** | Tokens, type, spacing, shape, elevation, components, breakpoints, showcase, this document | done |
| 2 — Search screen | App bar, search field, result list and metadata panel composed from `ui` | done |
| 3 — Detail screen | Episode list, episode sheet, compact layout | done |
| 4 — Dialogs and sheets | Data-driven settings, confirm dialog, clamped overlays | done |
| 5 — Navigation drawer | Drawer destinations, navigation stack | done |
| 6 — Polish | Empty/error/retry states, time-based progress, contrast pass | done |
| 7 — Supporting screens | History, watchlist (the AniList list), AniList page, adaptive settings | done |

The pre-redesign palette (`tui/theme/legacy.go` and the `Theme` variable of
`tui/theme.go`) is gone: every screen of the application draws itself from
the tokens of `tui/theme` and the components of `tui/ui`.

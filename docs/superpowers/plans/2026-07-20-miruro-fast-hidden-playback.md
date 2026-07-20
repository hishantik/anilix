# Fast Hidden Miruro Playback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hide managed Chromium and reduce Miruro episode-to-mpv startup from roughly one minute to a few seconds without changing AniList completion timing.

**Architecture:** Race Miruro mirrors and retain the fastest origin, run the browser fallback headlessly, and expose Miruro source candidates lazily to the TUI. The TUI tries a cached successful provider or `ally` first, launches immediately, and resolves another provider only after a source or player failure.

**Tech Stack:** Go 1.25, `net/http`, `context`, `chromedp`, Bubble Tea v2, existing `source` and `player` packages.

## Global Constraints

- Managed Chromium must never show a window.
- `ally` branding is acceptable and `ally` is the initial desktop preference.
- Provider preference is an optimization, not a permanent lock.
- AniList progress updates only after desktop playback exits successfully.
- The `o` Miruro watch-page fallback remains available after all native choices fail.
- Android intent playback behavior remains unchanged.
- Upstream PR #3 remains open until the replacement is manually validated and published to the fork.

---

### Task 1: Hidden managed Chromium

**Files:**
- Modify: `provider/miruro/managed_browser.go`
- Test: `provider/miruro/managed_browser_test.go`

**Interfaces:**
- Consumes: `NewManagedBrowserTransport(profileDir string, origins []string) (Transport, error)`.
- Produces: `managedBrowserAllocatorOptions(executable, profileDir string) []chromedp.ExecAllocatorOption`, used by `ensureStarted`.

- [ ] **Step 1: Write the failing allocator-option test**

Add `TestManagedBrowserAllocatorUsesHeadlessMode`. Use a fake executable or inspect the allocator options through a small option-building seam. Assert headless mode is enabled and `--headless=false` is absent. Keep executable discovery tests unchanged.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./provider/miruro -run TestManagedBrowserAllocatorUsesHeadlessMode -count=1 -v`

Expected: FAIL because the helper does not exist or the current options explicitly set headless false.

- [ ] **Step 3: Extract and use headless allocator options**

Implement:

```go
func managedBrowserAllocatorOptions(executable, profileDir string) []chromedp.ExecAllocatorOption {
	return append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(executable),
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
}
```

Call it from `ensureStarted` and change the startup error so it no longer asks for visible verification.

- [ ] **Step 4: Run package tests and verify GREEN**

Run: `go test ./provider/miruro -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add provider/miruro/managed_browser.go provider/miruro/managed_browser_test.go
git commit -m "fix: run Miruro browser fallback headlessly"
```

### Task 2: Fast sticky mirror racing

**Files:**
- Modify: `provider/miruro/transport.go`
- Test: `provider/miruro/transport_test.go`

**Interfaces:**
- Consumes: `HTTPTransport.Get(ctx context.Context, path string, query url.Values) ([]byte, error)`.
- Produces: cancellable mirror racing and a mutex-protected `preferredOrigin string`.

- [ ] **Step 1: Write failing concurrency and sticky-origin tests**

Add these deterministic `httptest.Server` cases:

```go
func TestHTTPTransportReturnsFastMirrorWithoutWaitingForSlowMirror(t *testing.T)
func TestHTTPTransportPrefersLastSuccessfulMirror(t *testing.T)
func TestHTTPTransportPreservesAccessDeniedWhenAllMirrorsDeny(t *testing.T)
```

The slow server blocks until its request context is cancelled; the fast server returns valid JSON. The sticky test verifies a prior winner can satisfy the next call without contacting the former loser. The denial test asserts `errors.Is(err, ErrAccessDenied)`.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./provider/miruro -run 'TestHTTPTransport(ReturnsFast|PrefersLast|PreservesAccess)' -count=1 -v`

Expected: slow-mirror timing and sticky-origin assertions fail under serial iteration.

- [ ] **Step 3: Implement cancellable mirror racing**

Add `preferredOrigin string` and `sync.RWMutex` to `HTTPTransport`. Put the preferred origin first, start one goroutine per mirror with a five-second child timeout, and return the first successful decoded body. Cancel outstanding attempts after success. Update the preferred origin only after success. Aggregate failures while preserving `ErrAccessDenied` when every response was denied.

- [ ] **Step 4: Run transport and fallback tests**

Run: `go test ./provider/miruro -run 'TestHTTPTransport|TestFallbackTransport' -count=1 -v`

Expected: PASS with no leaked test-server requests.

- [ ] **Step 5: Commit**

```powershell
git add provider/miruro/transport.go provider/miruro/transport_test.go
git commit -m "perf: race and remember Miruro mirrors"
```

### Task 3: Lazy provider playback and success preference

**Files:**
- Modify: `provider/miruro/provider.go`
- Modify: `tui/commands.go`
- Test: `provider/miruro/provider_test.go`
- Test: `tui/miruro_test.go`

**Interfaces:**
- Produces: `type StreamCandidate struct { Provider string; Resolve func() ([]*source.Stream, error) }`.
- Produces: `func (p *Provider) CandidateStreams(episode *source.Episode) ([]StreamCandidate, error)`.
- Produces: `func (p *Provider) MarkSuccessfulProvider(name string)` and `func (p *Provider) PreferredProvider() string`.
- Keeps: `StreamsOf` for generic `source.Source` callers.

- [ ] **Step 1: Write failing provider-order tests**

Test candidate order as `last successful provider`, then `ally`, then remaining catalog order. Verify a missing preference is skipped, duplicates are removed, and constructing candidates performs no `sources` transport calls.

- [ ] **Step 2: Write failing TUI lazy-fallback test**

Inject a candidate source seam and player-launch seam. Make candidate one fail launch and candidate two succeed. Assert candidate two is unresolved until candidate one fails, `PlayStreamMsg` follows success, and the successful provider is recorded. Verify total failure retains `TUIErrorMsg.BrowserURL`.

- [ ] **Step 3: Run focused tests and verify RED**

Run: `go test ./provider/miruro ./tui -run 'Test.*(Candidate|Lazy|ProviderFallback)' -count=1 -v`

Expected: FAIL because the candidate APIs and seams do not exist.

- [ ] **Step 4: Implement lazy resolution**

Store the last successful provider behind the provider mutex. `CandidateStreams` obtains the cached catalog once, orders candidates, and builds closures that call `client.Sources` only when invoked. Refactor `playEpisode` to resolve and launch one candidate at a time. Mark success only after a zero desktop player exit. Collect provider-labelled errors and continue. Keep Android and `StreamsOf` compatibility unchanged.

- [ ] **Step 5: Run provider, player, and TUI tests**

Run: `go test ./provider/miruro ./player ./tui -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add provider/miruro/provider.go provider/miruro/provider_test.go tui/commands.go tui/miruro_test.go
git commit -m "perf: resolve Miruro providers lazily"
```

### Task 4: Verification and installed test build

**Files:**
- Generate outside Git: `C:\Users\Sayu\AppData\Local\anilix\anilix.exe`

**Interfaces:**
- Consumes the hidden transport and lazy playback path.
- Produces an installed test binary. It does not push or close the upstream PR.

- [ ] **Step 1: Run deterministic verification**

Run `go test ./... -count=1`, `go vet ./...`, `go build ./...`, and `git diff --check`.

Expected: every command exits 0.

- [ ] **Step 2: Run live timing verification**

Run:

```powershell
$env:MIRURO_INTEGRATION='1'
go test ./provider/miruro -run TestIntegrationCatalogAndPlayableSource -count=1 -v
Remove-Item Env:MIRURO_INTEGRATION
```

Log durations and provider names but never signed stream URLs. Expected: PASS and no Chromium window.

- [ ] **Step 3: Install the test executable**

```powershell
go build -o .\anilix-miruro.exe .
Copy-Item -LiteralPath .\anilix-miruro.exe -Destination C:\Users\Sayu\AppData\Local\anilix\anilix.exe -Force
anilix --help
Remove-Item -LiteralPath .\anilix-miruro.exe
```

Expected: help exits 0 and `git status --short` is clean.

- [ ] **Step 4: Manual checkpoint**

Ask the user to verify that no Chromium window appears, episodes load promptly, mpv opens within a few seconds, closing mpv updates AniList, and `o` remains available after total failure.

- [ ] **Step 5: Publish only after approval**

After manual approval, use the finishing-a-development-branch workflow to publish to `sayuop/anilix`. Verify the fork, then close upstream PR #3:

```powershell
gh pr close 3 --repo hishantik/anilix --comment "Closing this because the AllAnime request repair is no longer viable. It has been superseded in my fork by a Miruro-based provider migration."
```

Expected: `gh pr view 3 --repo hishantik/anilix --json state` reports `CLOSED`.

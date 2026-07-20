# Fast Hidden Miruro Playback Design

## Goal

Make Miruro-backed playback start quickly without showing a Chromium window, while preserving reliable mpv fallback and updating AniList only after successful desktop playback finishes.

## Current problems

The managed Chromium transport starts a visible browser when direct Miruro requests are denied. HTTP requests try official mirrors serially, so a slow or unavailable mirror can delay every operation. Playback source resolution also fetches every candidate before mpv starts. In the observed Naruto episode, `bonk` was not compatible with mpv, `kiwi` returned a Cloudflare response, and `ally` played successfully. Resolving all three made startup slow even though a working choice was known.

## Transport design

Managed Chromium will run in modern headless mode. It remains an API fallback only and never becomes the video player. If a challenge cannot be completed headlessly, that mirror or provider fails normally and resolution continues. The existing `o` key remains the final user-visible browser fallback.

The HTTP transport will race official Miruro mirrors under short per-attempt timeouts. The first successful response wins and its origin becomes the preferred origin for later calls. Subsequent calls try that sticky origin first. Outstanding requests are cancelled after a winner is selected. Error aggregation must preserve access-denied classification so the browser fallback still activates only for access challenges.

## Episode and source resolution

Episode catalogs retain their existing in-memory cache. Miruro configuration loading and catalog fetching must not add avoidable serial latency.

Playback will resolve provider candidates lazily instead of downloading every candidate before opening mpv:

1. Try the most recently successful provider when it is available for the episode.
2. Otherwise prefer `ally`, which is confirmed to work with desktop mpv in the current environment.
3. Try remaining Miruro-native candidates only when the preceding source request or player launch fails.
4. Record a provider as successful only after the player exits successfully.

Provider preference is an optimization, not a permanent lock. Missing episodes and failures continue through the remaining candidates. The permanent `allanime.site` branding in the `ally` video is acceptable to the user.

The candidate and stream APIs will remain isolated from the generic `source.Source` contract through a Miruro-specific lazy-resolution method. Existing callers of `StreamsOf` remain supported and tested, but the TUI playback path uses lazy groups so it can start as soon as one provider resolves.

## Playback and tracking

Desktop player execution remains inside the Bubble Tea command, so the TUI stays responsive. A non-zero mpv/VLC/IINA exit is a provider failure and triggers the next candidate. A zero exit marks playback successful and only then triggers the existing AniList progress/completion update. Android retains its existing intent-based behavior.

If every candidate fails, the TUI displays the accumulated error and offers `o` to open the Miruro watch page. Failed headless Chromium attempts must not display a window.

## Testing and timing

Deterministic tests will cover:

- headless Chromium allocator flags;
- concurrent mirror racing, cancellation, sticky-origin preference, and access-denied errors;
- provider preference and lazy fallback;
- successful-provider caching only after successful playback;
- AniList update messages remaining downstream of successful player completion;
- preservation of the Miruro watch-page fallback.

Live verification will measure episode catalog retrieval and time to the first playable provider. The target is a normal warm launch in a few seconds rather than approximately one minute. Verification also includes the full Go test suite, `go vet ./...`, `go build ./...`, and a manual installed-binary test by the user.

## Delivery

The changes remain on `feat/miruro-provider` until local testing is satisfactory. The installed Windows binary may be rebuilt for each manual test without merging the branch. Once the user confirms the remaining details:

1. push the finished branch or merge it into the fork's `main`, according to the user's chosen integration workflow;
2. verify the fork contains the Miruro implementation;
3. close upstream pull request [hishantik/anilix#3](https://github.com/hishantik/anilix/pull/3) with a short note that its AllAnime repair has been superseded by the fork's Miruro migration.

GitHub pull requests are closed rather than deleted. No upstream PR action occurs before the replacement has been tested and the user confirms it is ready.

# Anilix architecture

Anilix is a Go terminal application that searches AniList, displays supplemental episode metadata from Jikan, and plays Miruro-backed streams in the user's configured player.

## Playback flow

1. `tui` searches through `provider/anilist` and retains both AniList and MyAnimeList IDs.
2. `provider/miruro` fetches Miruro's episode catalog using the AniList ID.
3. The provider tries Miruro source candidates in the order supplied by Miruro's configuration.
4. `player` launches mpv, VLC, IINA, or an Android player with the selected quality and required HTTP headers.
5. When playback finishes, the existing AniList tracking command updates progress and completion state.
6. If no native stream can be played, the detail view offers `o` to open the matching Miruro watch page. Browser playback does not trigger an automatic tracking update.

## Provider layout

- `provider/anilist`: anime search, metadata, OAuth-backed tracking, and MAL ID mapping.
- `provider/jikan`: supplemental anime and episode metadata used by the TUI and AniSkip.
- `provider/miruro`: Miruro catalog parsing, source resolution, mirror rotation, and transport fallback.
- `provider/miruro/transport.go`: direct HTTP transport with official mirror rotation.
- `provider/miruro/managed_browser.go`: managed Chromium fallback for access challenges. It uses an isolated profile under `~/.anilix/miruro-browser` and reuses the session.
- `browser`: cross-platform system-browser launcher used by OAuth and the Miruro watch-page fallback.

## Miruro transport

Miruro API calls use the site's pipe envelope and decode either plain JSON or its compressed obfuscated response format. Direct HTTP is attempted first. When Miruro returns an access-denied response and a Chromium-family browser is installed, a visible managed browser performs the same-origin request with an isolated persistent profile.

The transport validates returned stream and subtitle URLs as HTTP(S), preserves per-stream referrer headers, caches catalogs briefly, and can be closed to release the managed browser process.

## Tests

Deterministic unit tests cover the pipe protocol, response decoding, mirror fallback, browser fallback, catalog parsing, provider selection, watch URL generation, TUI fallback behavior, and AniList ID mapping.

The live Miruro smoke test is opt-in:

```sh
MIRURO_INTEGRATION=1 go test ./provider/miruro -run TestIntegration -count=1 -v
```

Live Jikan checks are also opt-in with `JIKAN_INTEGRATION=1`; the default suite is deterministic and works offline.

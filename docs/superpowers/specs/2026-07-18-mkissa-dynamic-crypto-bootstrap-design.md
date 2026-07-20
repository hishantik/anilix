# Mkissa Dynamic Crypto Bootstrap Design

## Goal

Restore AllAnime episode-source resolution without a browser dependency by keeping the existing AllAnime catalog API and dynamically obtaining its rotating request-crypto parameters through Mkissa's frontend assets.

## Scope

Search and episode listing continue to use `https://api.allanime.day/api`. Only authenticated episode-source requests change. The implementation will fetch bootstrap data from `https://mkissa.to` and JavaScript assets hosted by either `cdn.mkissa.net` or `cdn.allanime.day`; it is not a full provider migration.

No remote branch, commit, or pull request will be created until the unit suite, project build, and live stream integration test pass. Existing uncommitted files will not be included in this work.

## Architecture

A focused crypto-bootstrap module in `provider/allanime` will own dynamic discovery, validation, caching, key derivation, and request-token generation. The existing client will consume the module through a small interface so parsing and caching can be tested with local HTTP servers rather than live upstream services.

The bootstrap flow is:

1. Fetch the Mkissa HTML page and extract its application-bundle URL, epoch, and Base64 `partB` value.
2. Accept application bundles only from HTTPS URLs on `cdn.mkissa.net` or `cdn.allanime.day`.
3. Fetch the application bundle, discover its relative JavaScript chunk paths, and scan those same-CDN chunks for a 64-character hexadecimal mask and numeric build ID.
4. Decode `partB`, XOR it with the 32-byte mask, and validate that the result is a 32-byte AES key.
5. Cache successful dynamic material for 30 minutes. Cache a failed discovery only briefly so a transient outage does not cause a request storm or remain sticky.
6. Generate an `aaReq` token for each episode request using the current five-minute time bucket, epoch, build ID, persisted-query hash, and AES-256-GCM.

## Episode Request Flow

The persisted GraphQL GET request retains the current variables and persisted-query hash. Its `extensions` object gains `aaReq`, and the request gains the current `x-build-id` header. The request continues to target `api.allanime.day` with the `youtu-chan.com` referer/origin expected by the current service.

Dynamic material is attempted first. A bundled last-known material set may be attempted once as a short-lived compatibility fallback, but dynamic discovery remains the primary path. A normal GraphQL POST cannot bypass `aaReq`; it will not be treated as a valid authentication fallback for episode queries.

## Response Handling

The client will distinguish transport errors, bootstrap parsing failures, rejected/stale crypto, GraphQL errors, encrypted payload decode failures, and successful responses containing no sources. Error messages will contain the stage and upstream classification without exposing keys or complete tokens.

Encrypted `tobeparsed` episode payloads will be decoded with AES-256-GCM using the selected dynamic key. The existing response-secret and legacy AES-CTR behavior may remain as bounded compatibility fallbacks. Direct `sourceUrls` responses continue to work unchanged.

## Security and Reliability

Dynamic asset fetching is restricted to HTTPS and the two explicit CDN hostnames. Relative chunk URLs are resolved against the validated application bundle, and redirects must not escape the allowlist. Parsed epoch, build ID, mask, and `partB` values are length- and type-checked before use.

The cache is concurrency-safe so simultaneous episode requests share one refresh. Context cancellation and bounded HTTP timeouts apply to every bootstrap request. No WebView, JavaScript runtime, external crypto process, or new third-party Go dependency is required.

## Testing and Success Criteria

Test-driven unit coverage will verify HTML parsing, chunk discovery, hostname restrictions, XOR key derivation, cache reuse/expiry, five-minute bucketing, deterministic token envelope decryption, and final episode URL/header construction. Tests will use fixed clocks, deterministic randomness, and local HTTP fixtures.

The repair is considered successful only when:

- `go test ./provider/allanime -short -count=1` passes;
- `go test ./... -short -count=1` passes;
- the project builds on the current Windows environment;
- `TestIntegration_FullStreamExtraction` retrieves at least one playable stream from the live service.

Nothing will be pushed or uploaded until these checks succeed and the user explicitly approves it.

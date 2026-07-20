# Mkissa Dynamic Crypto Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore live AllAnime episode playback by dynamically discovering rotating crypto material from Mkissa and attaching a valid `aaReq` token to episode-source requests.

**Architecture:** Add a focused `crypto_bootstrap.go` module that discovers, validates, and caches crypto material and creates AES-GCM request tokens. Adapt the existing AllAnime client to use that module for persisted episode queries and adapt response decoding to use the selected material while retaining bounded compatibility fallbacks.

**Tech Stack:** Go 1.26 standard library (`net/http`, `crypto/aes`, `crypto/cipher`, `crypto/sha256`, `encoding/base64`, `encoding/json`, `sync`, `time`) and the existing Go test suite.

## Global Constraints

- Search and episode listing remain on `https://api.allanime.day/api`.
- Dynamic bootstrap uses `https://mkissa.to` and only HTTPS assets on `cdn.mkissa.net` or `cdn.allanime.day`.
- No WebView, JavaScript runtime, external crypto process, or new Go dependency.
- Successful material is cached for 30 minutes; failed discovery receives only a short negative cache.
- No commit, push, or other Git upload until all verification succeeds and the user explicitly approves it.
- Preserve unrelated and pre-existing uncommitted files.

---

### Task 1: Crypto material parsing and key derivation

**Files:**
- Create: `provider/allanime/crypto_bootstrap.go`
- Create: `provider/allanime/crypto_bootstrap_test.go`

**Interfaces:**
- Produces: `type cryptoMaterial struct { epoch int; buildID string; key []byte; legacyCTR bool }`
- Produces: `parseBootstrapPage(page string) (appURL string, epoch int, partB string, err error)`
- Produces: `parseCryptoChunk(chunk string) (maskHex string, buildID string, err error)`
- Produces: `deriveCryptoKey(maskHex, partB string) ([]byte, error)`
- Produces: `validateCryptoAssetURL(rawURL string) (*url.URL, error)`

- [ ] **Step 1: Write failing table tests** for bootstrap HTML extraction, chunk mask/build extraction, malformed values, XOR derivation, and allowed/blocked asset URLs using fixed fixture strings.

- [ ] **Step 2: Verify RED**

Run: `go test ./provider/allanime -run 'Test(ParseBootstrapPage|ParseCryptoChunk|DeriveCryptoKey|ValidateCryptoAssetURL)' -count=1 -v`

Expected: build failure because the parser and derivation functions do not exist.

- [ ] **Step 3: Implement minimal parsing and derivation** using compiled regular expressions, standard Base64/hex decoding, explicit 32-byte validation, and exact HTTPS hostname checks.

- [ ] **Step 4: Verify GREEN**

Run the Task 1 command again. Expected: all selected tests pass.

### Task 2: Dynamic discovery and concurrency-safe caching

**Files:**
- Modify: `provider/allanime/crypto_bootstrap.go`
- Modify: `provider/allanime/crypto_bootstrap_test.go`

**Interfaces:**
- Produces: `type cryptoBootstrap struct { client *http.Client; pageURL string; now func() time.Time; ... }`
- Produces: `func (b *cryptoBootstrap) material(ctx context.Context) (cryptoMaterial, error)`
- Consumes: parsing, validation, and derivation interfaces from Task 1.

- [ ] **Step 1: Write failing HTTP-fixture tests** proving discovery fetches the page, app bundle, and chunks; skips irrelevant/unavailable chunks; rejects redirect escapes; reuses a successful cache; retries after negative-cache expiry; and coalesces concurrent refreshes.

- [ ] **Step 2: Verify RED**

Run: `go test ./provider/allanime -run 'TestCryptoBootstrap' -count=1 -v`

Expected: build failure because `cryptoBootstrap` and `material` do not exist.

- [ ] **Step 3: Implement minimal discovery/cache behavior** with context-aware `http.Request`, bounded response reads, redirect hostname enforcement, a mutex/condition-based single refresh, 30-minute success TTL, and one-minute failure TTL.

- [ ] **Step 4: Verify GREEN**

Run the Task 2 command again. Expected: all selected tests pass and fixture request counts match assertions.

### Task 3: Deterministic `aaReq` generation

**Files:**
- Modify: `provider/allanime/crypto_bootstrap.go`
- Modify: `provider/allanime/crypto_bootstrap_test.go`

**Interfaces:**
- Produces: `createAAReq(queryHash string, material cryptoMaterial, now time.Time) (string, error)`
- Consumes: `cryptoMaterial` from Task 1.

- [ ] **Step 1: Write a failing token test** that Base64-decodes the result, validates the version/IV/ciphertext/tag envelope, decrypts it using the fixture key, and asserts the exact JSON payload fields and five-minute timestamp bucket.

- [ ] **Step 2: Verify RED**

Run: `go test ./provider/allanime -run TestCreateAAReq -count=1 -v`

Expected: build failure because `createAAReq` does not exist.

- [ ] **Step 3: Implement minimal AES-256-GCM token creation** with the deterministic SHA-256-derived 12-byte IV and JSON payload `{v,ts,epoch,buildId,qh}`.

- [ ] **Step 4: Verify GREEN**

Run the Task 3 command again. Expected: token decrypts and all assertions pass.

### Task 4: Authenticated episode request integration

**Files:**
- Modify: `provider/allanime/client.go`
- Modify: `provider/allanime/client_test.go`

**Interfaces:**
- Changes: `AllanimeClient` owns a bootstrap provider and clock.
- Changes: `buildEpisodeRequestURL` receives an `aaReq` string and places it beside `persistedQuery` in `extensions`.
- Changes: persisted episode GET adds `x-build-id` and returns the crypto material used with the response.
- Consumes: `cryptoBootstrap.material` and `createAAReq` from Tasks 2–3.

- [ ] **Step 1: Extend the existing URL test so it fails** unless `extensions.aaReq` is encoded correctly, then add an HTTP-fixture test asserting `x-build-id`, referer, origin, variables, persisted hash, and a decryptable `aaReq`.

- [ ] **Step 2: Verify RED**

Run: `go test ./provider/allanime -run 'Test(BuildEpisodeRequestURL|PersistedEpisodeRequest)' -count=1 -v`

Expected: URL assertion fails and the request test fails because no token/header is sent.

- [ ] **Step 3: Implement the minimal client integration** and stop treating unauthenticated GraphQL POST as an episode-query fallback. Preserve POST behavior for catalog queries.

- [ ] **Step 4: Verify GREEN**

Run the Task 4 command again. Expected: both URL and request tests pass.

### Task 5: Dynamic encrypted-response decoding

**Files:**
- Modify: `provider/allanime/decoder.go`
- Create: `provider/allanime/decoder_test.go`
- Modify: `provider/allanime/client.go`

**Interfaces:**
- Produces: `decodeToBeParsed(encoded string, material cryptoMaterial) ([]SourceUrl, error)`
- Consumes: the material selected for the successful request in Task 4.

- [ ] **Step 1: Write failing response tests** for AES-GCM data encrypted by the fixture dynamic key, the response-secret fallback, malformed envelopes, and the bounded legacy CTR path.

- [ ] **Step 2: Verify RED**

Run: `go test ./provider/allanime -run 'TestDecodeToBeParsed' -count=1 -v`

Expected: dynamic GCM cases fail because the current decoder always uses a static CTR key.

- [ ] **Step 3: Implement minimal GCM decoding** using bytes 1–12 as IV and the final 16 bytes as tag; try the chosen material, then the versioned response-secret key, and only use CTR when `legacyCTR` is true. Parse source JSON with `encoding/json` instead of regular-expression pairing.

- [ ] **Step 4: Adapt client response flow** to pass the selected material into decoding and return stage-specific errors without logging secrets.

- [ ] **Step 5: Verify GREEN**

Run the Task 5 command again. Expected: all decoder tests pass.

### Task 6: Regression and live verification

**Files:**
- Modify only if a failing test exposes an in-scope defect in the new flow.

**Interfaces:**
- Consumes all preceding tasks; produces evidence that the user-visible playback path is restored.

- [ ] **Step 1: Format changed Go files**

Run: `gofmt -w provider/allanime/crypto_bootstrap.go provider/allanime/crypto_bootstrap_test.go provider/allanime/client.go provider/allanime/client_test.go provider/allanime/decoder.go provider/allanime/decoder_test.go`

- [ ] **Step 2: Run focused unit tests**

Run: `go test ./provider/allanime -short -count=1`

Expected: PASS.

- [ ] **Step 3: Run the complete short suite**

Run: `go test ./... -short -count=1`

Expected: PASS.

- [ ] **Step 4: Build the application**

Run: `go build ./...`

Expected: exit code 0.

- [ ] **Step 5: Run live stream extraction**

Run: `go test ./provider/allanime -run TestIntegration_FullStreamExtraction -count=1 -v`

Expected: PASS with at least one extracted playable stream.

- [ ] **Step 6: Inspect the working tree without committing**

Run: `git status --short` and `git diff -- provider/allanime`

Expected: only intended provider files are modified; no Git commit or push occurs.

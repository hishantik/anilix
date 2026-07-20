# AllAnime aaReq Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make PR #3's episode-source request satisfy AllAnime's live crypto requirement and return a playable stream.

**Architecture:** Extend the existing request URL builder to generate an authenticated `aaReq` value and place it beside `persistedQuery` in GraphQL extensions. Keep encoding, fallback behavior, and decoder logic unchanged.

**Tech Stack:** Go standard library (`crypto/aes`, `crypto/cipher`, `crypto/sha256`, `encoding/base64`, `encoding/json`, `net/url`), Go testing.

## Global Constraints

- Push only `provider/allanime/client.go` and `provider/allanime/client_test.go`.
- Do not push specifications or plans.
- Push only after focused tests, build, and live full-stream extraction pass.

---

### Task 1: Authenticated episode request

**Files:**
- Modify: `provider/allanime/client.go`
- Test: `provider/allanime/client_test.go`

**Interfaces:**
- Consumes: `buildEpisodeRequestURL(baseURL, showID, translationType, episodeString, queryHash string)`
- Produces: `buildAAReq(queryHash string, now time.Time) (string, error)` and an `aaReq` field in encoded GraphQL extensions.

- [ ] **Step 1: Write failing request regression tests**

Add tests that parse the URL from `buildEpisodeRequestURL`, require non-empty `extensions.aaReq`, decode its version/nonce/ciphertext envelope, decrypt it with AES-GCM and `allAnimeKeyHex`, and assert payload fields `v`, `ts`, `epoch`, `buildId`, and `qh`.

- [ ] **Step 2: Verify RED**

Run: `go test ./provider/allanime -run 'TestBuildEpisodeRequestURLIncludesAAReq|TestBuildAAReqPayload' -count=1 -v`

Expected: FAIL because `aaReq` and `buildAAReq` do not exist.

- [ ] **Step 3: Implement the minimal request change**

Generate the five-minute timestamp bucket, JSON payload, deterministic IV derived from epoch/build/query hash/timestamp, and AES-256-GCM envelope. Add the base64 token to `extensions.aaReq`, preserving `persistedQuery` and standard query encoding.

- [ ] **Step 4: Verify GREEN locally**

Run: `go test ./provider/allanime -short -count=1`

Expected: PASS.

- [ ] **Step 5: Verify the workaround live**

Run: `go build .`

Expected: exit 0.

Run: `go test ./provider/allanime -run TestIntegration_FullStreamExtraction -v -count=1`

Expected: PASS with at least one source and one playable stream. If it fails, stop, retain changes locally, and do not push.

- [ ] **Step 6: Commit and update PR #3**

Run: `git add provider/allanime/client.go provider/allanime/client_test.go`

Run: `git commit -m "fix: authenticate AllAnime episode requests"`

Confirm the commit contains only those two paths, then run `git push origin main`.

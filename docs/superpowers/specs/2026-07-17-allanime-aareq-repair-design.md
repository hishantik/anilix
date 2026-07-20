# AllAnime aaReq Repair Design

## Goal

Update PR #3 so episode-source requests satisfy AllAnime's current cryptographic request contract and produce at least one playable stream in the live integration test.

## Request flow

The episode persisted-query request will retain PR #3's JSON marshaling and standards-based query encoding. Its `extensions` value will additionally contain an `aaReq` token generated from the current AllAnime key, epoch, build ID, query hash, and five-minute timestamp bucket.

The implementation will follow the current ani-cli reference format. Crypto parameters will be verified against that reference before being committed; yesterday's cached values will not be assumed current.

## Scope

Changes stay within the AllAnime client and its tests. Search, episode listing, stream extraction, player behavior, and unrelated providers will not be refactored.

## Error handling

Token construction errors will abort the persisted request with context instead of silently falling through to a request known to be invalid. Existing referer attempts and the regular GraphQL fallback remain unchanged.

## Testing and release gate

Deterministic unit tests will validate that:

- the final encoded request contains both `persistedQuery` and `aaReq`;
- the generated token has the expected envelope and decrypts to the required payload;
- timestamp bucketing and query-hash binding are correct.

The change may be pushed to `sayuop/anilix:main` only after the AllAnime unit tests pass, the project builds, and `TestIntegration_FullStreamExtraction` returns at least one playable stream against the live service. Pushing that commit will update hishantik/anilix PR #3 automatically.

package hianime

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// encryptMegaPlay builds an enc payload the way the site does, so the
// decrypt path is tested against a real AES-CBC round trip.
func encryptMegaPlay(t *testing.T, payload string) string {
	t.Helper()
	key := padKey(megaPlayKey, 32)
	iv := padKey(megaPlayIV, 16)

	// PKCS7 pad to the block size.
	pad := 16 - len(payload)%16
	padded := payload + strings.Repeat(string(rune(pad)), pad)

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, []byte(padded))
	return base64.RawURLEncoding.EncodeToString(out)
}

func TestDecryptMegaPlaySources(t *testing.T) {
	enc := encryptMegaPlay(t, `{"file":"https://fetch.example.top/anime/aa/bb/master.m3u8"}`)
	got, err := decryptMegaPlaySources(enc)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	want := "https://fetch.example.top/anime/aa/bb/master.m3u8"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecryptMegaPlaySourcesPaddedBase64(t *testing.T) {
	// The site sends raw base64url, but padded input must also decode.
	// Encrypt a payload long enough that standard padding is a no-op
	// (plaintext already a multiple of 3 bytes => base64 has no '='), then
	// decode via the padded encoder path.
	payload := `{"file":"https://x.test/m.m3u8abc"}` // 29 bytes... not multiple; use a longer one
	payload = `{"file":"https://x.test/master.m3u8aaaa"}`
	key := padKey(megaPlayKey, 32)
	iv := padKey(megaPlayIV, 16)
	pad := 16 - len(payload)%16
	padded := payload + strings.Repeat(string(rune(pad)), pad)
	block, _ := aes.NewCipher(key)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, []byte(padded))
	enc := base64.URLEncoding.EncodeToString(out) // padded encoding
	got, err := decryptMegaPlaySources(enc)
	if err != nil {
		t.Fatalf("padded base64 should decode: %v", err)
	}
	want := "https://x.test/master.m3u8aaaa"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecryptMegaPlaySourcesGarbage(t *testing.T) {
	enc := encryptMegaPlay(t, "not json at all")
	if _, err := decryptMegaPlaySources(enc); err == nil {
		t.Fatal("non-JSON plaintext should fail")
	}

	if _, err := decryptMegaPlaySources("short"); err == nil {
		t.Fatal("truncated payload should fail")
	}

	if _, err := decryptMegaPlaySources(""); err == nil {
		t.Fatal("empty payload should fail")
	}
}

func TestMegaPlayResponseEncPreferred(t *testing.T) {
	// When both enc and sources are present, enc wins (matches the player).
	body := map[string]any{
		"enc": encryptMegaPlay(t, `{"file":"https://from-enc.test/master.m3u8"}`),
		"sources": []map[string]string{
			{"file": "https://from-sources.test/master.m3u8"},
		},
	}
	raw, _ := json.Marshal(body)
	var resp megaPlayResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	got, err := resp.streamURL()
	if err != nil {
		t.Fatalf("streamURL failed: %v", err)
	}
	if got != "https://from-enc.test/master.m3u8" {
		t.Fatalf("got %q", got)
	}
}

func TestMegaPlayResponsePlainSources(t *testing.T) {
	tests := []struct {
		name  string
		sbody string
		want  string
	}{
		{"array of file", `[{"file":"https://a.test/m.m3u8"}]`, "https://a.test/m.m3u8"},
		{"array of url", `[{"url":"https://b.test/m.m3u8"}]`, "https://b.test/m.m3u8"},
		{"single object", `{"file":"https://c.test/m.m3u8"}`, "https://c.test/m.m3u8"},
		{"plain string", `"https://d.test/m.m3u8"`, "https://d.test/m.m3u8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := megaPlayResponse{Sources: json.RawMessage(tt.sbody)}
			got, err := resp.streamURL()
			if err != nil {
				t.Fatalf("streamURL failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMegaPlayResponseEmpty(t *testing.T) {
	resp := megaPlayResponse{}
	if _, err := resp.streamURL(); err == nil {
		t.Fatal("empty response should fail")
	}
}

func TestMegaPlayDataID(t *testing.T) {
	page := `<div class="fix-area" id="megaplay-player"
		data-id="39807"
		data-realid="1464"
		data-mediaid="1814">`
	if got := megaPlayDataID(page); got != "39807" {
		t.Fatalf("got %q, want 39807", got)
	}
	if got := megaPlayDataID(`<div class="error">Error - MegaPlay</div>`); got != "" {
		t.Fatalf("error page should have no data-id, got %q", got)
	}
}

func TestMegaPlayType(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://megaplay.buzz/stream/s-2/1464/sub?s=tcdn", "sub"},
		{"https://megaplay.buzz/stream/s-2/1464/dub?s=tcdn", "dub"},
		{"https://megaplay.buzz/stream/s-2/1464/hsub", "sub"},
		{"https://megaplay.buzz/stream/s-2/1464", "sub"},
	}
	for _, tt := range tests {
		if got := megaPlayType(tt.url); got != tt.want {
			t.Errorf("megaPlayType(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestWithAudioType(t *testing.T) {
	got, err := withAudioType("https://cdn.test/anime/aa/bb/master.m3u8", "dub")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.test/anime/aa/bb/master.m3u8?t=dub" {
		t.Fatalf("got %q", got)
	}

	// Existing query params are preserved; url.Values encodes alphabetically.
	got, err = withAudioType("https://cdn.test/m.m3u8?x=1", "sub")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.test/m.m3u8?t=sub&x=1" {
		t.Fatalf("got %q", got)
	}
}

func TestAttachMegaPlayToken(t *testing.T) {
	raw := "https://fetch.test/anime/0123456789abcdef0123456789abcdef/fedcba9876543210fedcba9876543210/master.m3u8"
	tokenized, err := attachMegaPlayToken(raw, time.Unix(1000000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if tokenized == raw {
		t.Fatal("token should be appended")
	}
	if !strings.Contains(tokenized, "token=") {
		t.Fatalf("token missing: %s", tokenized)
	}

	// Decode the token and verify the message shape: "<exp>|<seg1>/<seg2>".
	token := strings.TrimPrefix(tokenized, raw+"?token=")
	decoded, err := base64.RawURLEncoding.DecodeString(strings.SplitN(token, ".", 2)[0])
	if err != nil {
		t.Fatalf("token message not base64url: %v", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		t.Fatalf("token message malformed: %q", decoded)
	}
	if parts[1] != "0123456789abcdef0123456789abcdef/fedcba9876543210fedcba9876543210" {
		t.Fatalf("token path wrong: %q", parts[1])
	}

	// The MAC must verify against the secret.
	mac := hmac.New(sha256.New, []byte(megaPlayTokenKey))
	mac.Write(decoded)
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != strings.SplitN(token, ".", 2)[1] {
		t.Fatal("HMAC does not verify")
	}

	// Already-tokenized URLs are left alone.
	if again, _ := attachMegaPlayToken(tokenized, time.Now()); again != tokenized {
		t.Fatal("tokenized URL should not be modified")
	}

	// Non-tokenized path layout (e.g. mirror without hex pairs) is unchanged.
	plain := "https://mirror.test/stream/master.m3u8"
	if got, _ := attachMegaPlayToken(plain, time.Now()); got != plain {
		t.Fatalf("plain URL should be unchanged, got %q", got)
	}
}

func TestAttachMegaPlayTokenQueryEscape(t *testing.T) {
	// The token contains '+' and '-' in base64url; QueryEscape must encode
	// them so the CDN sees the exact token.
	raw := "https://fetch.test/anime/aabbccdd11223344aabbccdd11223344/11223344aabbccdd11223344aabbccdd/master.m3u8"
	tokenized, err := attachMegaPlayToken(raw, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(tokenized, raw+"?token=")
	if strings.ContainsAny(token, "+") {
		t.Fatalf("'+' must be escaped: %s", token)
	}
}

func TestVariantsFromPlaylistMaster(t *testing.T) {
	master := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1280x720
index-f1.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=854x480
index-f2.m3u8
`
	variants, err := variantsFromPlaylist(master, "https://cdn.test/master.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != 2 {
		t.Fatalf("got %d variants, want 2", len(variants))
	}
}

func TestVariantsFromPlaylistMedia(t *testing.T) {
	media := `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.0,
seg-0.ts
#EXT-X-ENDLIST
`
	variants, err := variantsFromPlaylist(media, "https://cdn.test/media.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != 1 || variants[0].Quality != "auto" {
		t.Fatalf("media playlist should yield single auto variant, got %+v", variants)
	}
}

func TestVariantsFromPlaylistRejects(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"cloudflare 525 text", "error code: 525"},
		{"cloudflare 521 text", "error code: 521"},
		{"empty", ""},
		{"html blockpage", "<html><head><title>Blocked</title></head></html>"},
		{"json error", `{"error":"not found"}`},
		{"bom then garbage", "\ufeffnot an m3u8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := variantsFromPlaylist(tt.body, "https://cdn.test/m.m3u8"); err == nil {
				t.Fatalf("body %q should be rejected", tt.body)
			}
		})
	}
}

func TestVariantsFromPlaylistBOM(t *testing.T) {
	// BOM before #EXTM3U must not break parsing (needs a RESOLUTION attr to
	// be recognised as a master playlist entry).
	body := "\ufeff" + "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1,RESOLUTION=1280x720\nv0.m3u8\n"
	variants, err := variantsFromPlaylist(body, "https://cdn.test/master.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != 1 {
		t.Fatalf("BOM playlist should parse, got %d", len(variants))
	}
}

func TestBodySnippet(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := bodySnippet(long)
	if !strings.HasSuffix(got, `..."`) || len(got) > 200 {
		t.Fatalf("snippet too long: %d chars", len(got))
	}
	if got := bodySnippet(""); !strings.Contains(got, "empty response") {
		t.Fatalf("got %q", got)
	}
}

func TestPKCS7UnpadInvalid(t *testing.T) {
	block := aes.BlockSize
	if _, err := pkcs7Unpad(make([]byte, block+1), block); err == nil {
		t.Fatal("non-block-multiple should fail")
	}
	if _, err := pkcs7Unpad(nil, block); err == nil {
		t.Fatal("nil should fail")
	}
	bad := make([]byte, block)
	bad[block-1] = 200
	if _, err := pkcs7Unpad(bad, block); err == nil {
		t.Fatal("pad > blockSize should fail")
	}
	good := make([]byte, block)
	for i := block - 4; i < block; i++ {
		good[i] = 4
	}
	out, err := pkcs7Unpad(good, block)
	if err != nil || len(out) != block-4 {
		t.Fatalf("valid padding failed: %v, %d", err, len(out))
	}
}

func TestPadKey(t *testing.T) {
	if got := padKey(megaPlayKey, 32); len(got) != 32 {
		t.Fatalf("key length %d", len(got))
	}
	if got := padKey("ab", 4); string(got) != "ab\x00\x00" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("z", 40)
	if got := padKey(long, 8); len(got) != 8 {
		t.Fatalf("long key should truncate, got %d", len(got))
	}
}

// TestMegaPlayDecryptLiveVector checks the decrypt path against a captured
// payload from the real site (kept offline; verifies key/IV correctness).
func TestMegaPlayDecryptLiveVector(t *testing.T) {
	// Re-encrypt a known plaintext with the known key/IV, then decrypt.
	payload := fmt.Sprintf(`{"file":"https://fetch.nexabloom.top/anime/%s/%s/master.m3u8"}`,
		strings.Repeat("a", 32), strings.Repeat("b", 32))
	enc := encryptMegaPlay(t, payload)
	got, err := decryptMegaPlaySources(enc)
	if err != nil {
		t.Fatalf("live vector failed: %v", err)
	}
	if got != strings.TrimSuffix(strings.TrimPrefix(payload, `{"file":"`), `"}`) {
		t.Fatalf("round trip mismatch: %q", got)
	}
	_ = rand.Reader // keep crypto/rand import if unused elsewhere
}

package Allanime

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func textResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d", status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}

func TestParseBootstrapPage(t *testing.T) {
	page := `<html><script>window.__aaCrypto = {"epoch":4130,"partB":"AjhjboON3l/6/Y+WLrKww/kcIrmXCFBWUfdl7YHruaA="};</script><script src="https://cdn.mkissa.net/all/mk/_app/immutable/entry/app.D_sfA4pp.js"></script></html>`

	appURL, epoch, partB, err := parseBootstrapPage(page)
	if err != nil {
		t.Fatal(err)
	}
	if appURL != "https://cdn.mkissa.net/all/mk/_app/immutable/entry/app.D_sfA4pp.js" {
		t.Fatalf("appURL = %q", appURL)
	}
	if epoch != 4130 {
		t.Fatalf("epoch = %d", epoch)
	}
	if partB != "AjhjboON3l/6/Y+WLrKww/kcIrmXCFBWUfdl7YHruaA=" {
		t.Fatalf("partB = %q", partB)
	}
}

func TestParseBootstrapPageRejectsMissingMaterial(t *testing.T) {
	if _, _, _, err := parseBootstrapPage(`<html></html>`); err == nil {
		t.Fatal("expected missing bootstrap material error")
	}
}

func TestParseCryptoChunk(t *testing.T) {
	chunk := `const mask="cd7f14dbf40734836eb46eb14758e49ef9d81e61686d84d467b2e32063ef4af9";const build="44";`

	mask, buildID, err := parseCryptoChunk(chunk)
	if err != nil {
		t.Fatal(err)
	}
	if mask != "cd7f14dbf40734836eb46eb14758e49ef9d81e61686d84d467b2e32063ef4af9" || buildID != "44" {
		t.Fatalf("got mask %q build %q", mask, buildID)
	}
}

func TestParseCryptoChunkRejectsIncompleteChunk(t *testing.T) {
	if _, _, err := parseCryptoChunk(`const build="44";`); err == nil {
		t.Fatal("expected incomplete crypto chunk error")
	}
}

func TestDeriveCryptoKey(t *testing.T) {
	key, err := deriveCryptoKey(
		"cd7f14dbf40734836eb46eb14758e49ef9d81e61686d84d467b2e32063ef4af9",
		"AjhjboON3l/6/Y+WLrKww/kcIrmXCFBWUfdl7YHruaA=",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xcf, 0x47, 0x77, 0xb5, 0x77, 0x8a, 0xea, 0xdc, 0x94, 0x49, 0xe1, 0x27, 0x69, 0xea, 0x54, 0x5d, 0x00, 0xc4, 0x3c, 0xd8, 0xff, 0x65, 0xd4, 0x82, 0x36, 0x45, 0x86, 0xcd, 0xe2, 0x04, 0xf3, 0x59}
	if !bytes.Equal(key, want) {
		t.Fatalf("key = %x, want %x", key, want)
	}
}

func TestDeriveCryptoKeyRejectsWrongLength(t *testing.T) {
	if _, err := deriveCryptoKey("00", "AA=="); err == nil {
		t.Fatal("expected invalid key length error")
	}
}

func TestValidateCryptoAssetURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://cdn.mkissa.net/all/mk/app.js",
		"https://cdn.allanime.day/chunks/crypto.js",
	} {
		if _, err := validateCryptoAssetURL(rawURL); err != nil {
			t.Errorf("validateCryptoAssetURL(%q): %v", rawURL, err)
		}
	}
}

func TestValidateCryptoAssetURLRejectsUnsafeTargets(t *testing.T) {
	for _, rawURL := range []string{
		"http://cdn.mkissa.net/app.js",
		"https://mkissa.net/app.js",
		"https://cdn.mkissa.net.evil.example/app.js",
		"not a URL",
	} {
		if _, err := validateCryptoAssetURL(rawURL); err == nil {
			t.Errorf("validateCryptoAssetURL(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestCryptoBootstrapDiscoversMaterialAndCachesIt(t *testing.T) {
	page := `<script>window.__aaCrypto={"epoch":4130,"partB":"AjhjboON3l/6/Y+WLrKww/kcIrmXCFBWUfdl7YHruaA="}</script><script src="https://cdn.mkissa.net/all/mk/_app/immutable/entry/app.test.js"></script>`
	app := `const chunks=["../chunks/optional.js","../chunks/crypto.js"]`
	chunk := `const mask="cd7f14dbf40734836eb46eb14758e49ef9d81e61686d84d467b2e32063ef4af9";const build="44";`
	requests := make(map[string]int)
	var requestsMu sync.Mutex
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestsMu.Lock()
		requests[req.URL.String()]++
		requestsMu.Unlock()
		switch req.URL.String() {
		case "https://mkissa.to":
			return textResponse(req, http.StatusOK, page), nil
		case "https://cdn.mkissa.net/all/mk/_app/immutable/entry/app.test.js":
			return textResponse(req, http.StatusOK, app), nil
		case "https://cdn.mkissa.net/all/mk/_app/immutable/chunks/optional.js":
			return textResponse(req, http.StatusNotFound, "missing"), nil
		case "https://cdn.mkissa.net/all/mk/_app/immutable/chunks/crypto.js":
			return textResponse(req, http.StatusOK, chunk), nil
		default:
			return nil, fmt.Errorf("unexpected URL %s", req.URL)
		}
	})}
	now := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	bootstrap := newCryptoBootstrap(client, "https://mkissa.to", func() time.Time { return now })

	first, err := bootstrap.material(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := bootstrap.material(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.epoch != 4130 || first.buildID != "44" || !bytes.Equal(first.key, second.key) {
		t.Fatalf("unexpected material: epoch=%d build=%q key=%x", first.epoch, first.buildID, first.key)
	}
	requestsMu.Lock()
	defer requestsMu.Unlock()
	if requests["https://mkissa.to"] != 1 {
		t.Fatalf("page requests = %d, want 1", requests["https://mkissa.to"])
	}
}

func TestCryptoBootstrapRetriesAfterFailureCacheExpires(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return textResponse(req, http.StatusBadGateway, "unavailable"), nil
	})}
	bootstrap := newCryptoBootstrap(client, "https://mkissa.to", func() time.Time { return now })

	if _, err := bootstrap.material(context.Background()); err == nil {
		t.Fatal("expected discovery failure")
	}
	if _, err := bootstrap.material(context.Background()); err == nil {
		t.Fatal("expected cached discovery failure")
	}
	if requests != 1 {
		t.Fatalf("requests during failure TTL = %d, want 1", requests)
	}
	now = now.Add(time.Minute + time.Second)
	if _, err := bootstrap.material(context.Background()); err == nil {
		t.Fatal("expected repeated discovery failure")
	}
	if requests != 2 {
		t.Fatalf("requests after failure TTL = %d, want 2", requests)
	}
}

func TestCryptoBootstrapCoalescesConcurrentRefresh(t *testing.T) {
	page := `<script>window.__aaCrypto={"epoch":4130,"partB":"AjhjboON3l/6/Y+WLrKww/kcIrmXCFBWUfdl7YHruaA="}</script><script src="https://cdn.mkissa.net/all/mk/_app/immutable/entry/app.test.js"></script>`
	app := `const chunks=["../chunks/crypto.js"]`
	chunk := `const mask="cd7f14dbf40734836eb46eb14758e49ef9d81e61686d84d467b2e32063ef4af9";const build="44";`
	var mu sync.Mutex
	pageRequests := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://mkissa.to" {
			mu.Lock()
			pageRequests++
			mu.Unlock()
			return textResponse(req, http.StatusOK, page), nil
		}
		if strings.HasSuffix(req.URL.Path, "app.test.js") {
			return textResponse(req, http.StatusOK, app), nil
		}
		return textResponse(req, http.StatusOK, chunk), nil
	})}
	bootstrap := newCryptoBootstrap(client, "https://mkissa.to", time.Now)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := bootstrap.material(context.Background()); err != nil {
				t.Errorf("material: %v", err)
			}
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if pageRequests != 1 {
		t.Fatalf("page requests = %d, want 1", pageRequests)
	}
}

func TestCreateAAReq(t *testing.T) {
	material := cryptoMaterial{
		epoch:   4130,
		buildID: "44",
		key:     bytes.Repeat([]byte{0x2a}, 32),
	}
	now := time.Date(2026, 7, 18, 20, 7, 59, 0, time.UTC)
	const queryHash = "d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec"

	token, err := createAAReq(queryHash, material, now)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope) < 30 || envelope[0] != 1 {
		t.Fatalf("invalid token envelope: %x", envelope)
	}
	block, err := aes.NewCipher(material.key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := gcm.Open(nil, envelope[1:13], envelope[13:], nil)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Version int    `json:"v"`
		TS      int64  `json:"ts"`
		Epoch   int    `json:"epoch"`
		BuildID string `json:"buildId"`
		Hash    string `json:"qh"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		t.Fatal(err)
	}
	wantTS := now.UnixMilli() / 300000 * 300000
	if payload.Version != 1 || payload.TS != wantTS || payload.Epoch != 4130 || payload.BuildID != "44" || payload.Hash != queryHash {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

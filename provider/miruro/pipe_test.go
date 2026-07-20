package miruro

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestEncodePipeRequest(t *testing.T) {
	encoded, err := encodePipeRequest("episodes", map[string]string{"anilistId": "20"})
	if err != nil {
		t.Fatalf("encodePipeRequest: %v", err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}

	var got pipeEnvelope
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got.Path != "episodes" || got.Method != "GET" || got.Body != nil {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if got.Query["anilistId"] != "20" {
		t.Fatalf("unexpected query: %+v", got.Query)
	}
}

func TestDecodePipeResponsePlainJSON(t *testing.T) {
	got, err := decodePipeResponse([]byte(`{"providers":{}}`), "")
	if err != nil {
		t.Fatalf("decodePipeResponse: %v", err)
	}
	if string(got) != `{"providers":{}}` {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestDecodePipeResponseObfuscatedGzip(t *testing.T) {
	want := []byte(`{"streams":[{"url":"https://video.example/master.m3u8"}]}`)
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	encodedBytes := compressed.Bytes()
	for i := range encodedBytes {
		encodedBytes[i] ^= pipeObfuscationKey[i%len(pipeObfuscationKey)]
	}
	encoded := base64.RawURLEncoding.EncodeToString(encodedBytes)

	got, err := decodePipeResponse([]byte(encoded), "2")
	if err != nil {
		t.Fatalf("decodePipeResponse: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded response = %s, want %s", got, want)
	}
}

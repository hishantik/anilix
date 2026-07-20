package miruro

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

const pipeObfuscationKeyHex = "71951034f8fbcf53d89db52ceb3dc22c"

var pipeObfuscationKey = mustDecodeHex(pipeObfuscationKeyHex)

type pipeEnvelope struct {
	Path   string            `json:"path"`
	Method string            `json:"method"`
	Query  map[string]string `json:"query"`
	Body   any               `json:"body"`
}

func encodePipeRequest(path string, query map[string]string) (string, error) {
	raw, err := json.Marshal(pipeEnvelope{Path: path, Method: "GET", Query: query, Body: nil})
	if err != nil {
		return "", fmt.Errorf("marshal Miruro pipe request: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodePipeResponse(body []byte, obfuscation string) ([]byte, error) {
	if obfuscation == "" {
		if !json.Valid(body) {
			return nil, fmt.Errorf("Miruro pipe returned invalid JSON")
		}
		return body, nil
	}

	raw := make([]byte, base64.RawURLEncoding.DecodedLen(len(body)))
	n, err := base64.RawURLEncoding.Decode(raw, bytes.TrimSpace(body))
	if err != nil {
		return nil, fmt.Errorf("decode Miruro pipe response: %w", err)
	}
	raw = raw[:n]
	if obfuscation == "2" {
		for i := range raw {
			raw[i] ^= pipeObfuscationKey[i%len(pipeObfuscationKey)]
		}
	}

	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("open Miruro pipe response: %w", err)
	}
	defer zr.Close()
	decoded, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("decompress Miruro pipe response: %w", err)
	}
	if !json.Valid(decoded) {
		return nil, fmt.Errorf("Miruro pipe returned invalid decoded JSON")
	}
	return decoded, nil
}

func mustDecodeHex(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return decoded
}

package Allanime

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func encryptGCMEnvelope(t *testing.T, key []byte, version byte, plaintext string) string {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	iv := []byte("0123456789ab")
	envelope := append([]byte{version}, iv...)
	envelope = gcm.Seal(envelope, iv, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(envelope)
}

func TestDecodeToBeParsedUsesDynamicGCMKey(t *testing.T) {
	material := cryptoMaterial{key: []byte("0123456789abcdef0123456789abcdef")}
	encoded := encryptGCMEnvelope(t, material.key, 1, `[{"sourceUrl":"https://video.example/master.m3u8","sourceName":"Default"}]`)

	sources, err := decodeToBeParsed(encoded, material)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].SourceUrl != "https://video.example/master.m3u8" || sources[0].SourceName != "Default" {
		t.Fatalf("unexpected sources: %+v", sources)
	}
}

func TestDecodeToBeParsedUsesVersionedResponseSecret(t *testing.T) {
	key := sha256.Sum256([]byte("Xot36i3lK3:v1"))
	encoded := encryptGCMEnvelope(t, key[:], 1, `{"sourceUrls":[{"sourceUrl":"https://video.example/video.mp4","sourceName":"Fallback"}]}`)

	sources, err := decodeToBeParsed(encoded, cryptoMaterial{key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].SourceName != "Fallback" {
		t.Fatalf("unexpected sources: %+v", sources)
	}
}

func TestDecodeToBeParsedRejectsMalformedEnvelope(t *testing.T) {
	if _, err := decodeToBeParsed(base64.StdEncoding.EncodeToString([]byte("short")), cryptoMaterial{key: make([]byte, 32)}); err == nil {
		t.Fatal("expected malformed envelope error")
	}
}

func TestDecodeToBeParsedDoesNotUseCTRWithoutLegacyFlag(t *testing.T) {
	data := make([]byte, 29)
	data[0] = 1
	if _, err := decodeToBeParsed(base64.StdEncoding.EncodeToString(data), cryptoMaterial{key: make([]byte, 32)}); err == nil {
		t.Fatal("expected GCM authentication error")
	}
}

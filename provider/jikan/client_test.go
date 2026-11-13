package jikan

import (
	"testing"
	"time"
)

func TestJikanClient_NewClient(t *testing.T) {
	client := NewClient("https://api.jikan.moe/v4")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.baseURL != "https://api.jikan.moe/v4" {
		t.Errorf("expected base URL https://api.jikan.moe/v4, got %s", client.baseURL)
	}
}

func TestJikanClient_OutageBreaker(t *testing.T) {
	client := NewClient("https://api.jikan.moe/v4")

	if client.IsDown() {
		t.Error("fresh client must not be marked down")
	}

	client.markDown()
	if !client.IsDown() {
		t.Error("client must be marked down after a failure")
	}

	client.markUp()
	if client.IsDown() {
		t.Error("client must be usable again after a success")
	}
}

func TestRateLimiter_Acquire(t *testing.T) {
	rl := newRateLimiter(3, time.Second)

	if !rl.acquire() {
		t.Error("expected to acquire first token")
	}
	if !rl.acquire() {
		t.Error("expected to acquire second token")
	}
	if !rl.acquire() {
		t.Error("expected to acquire third token")
	}

	if rl.acquire() {
		t.Error("expected to be rate limited")
	}
}

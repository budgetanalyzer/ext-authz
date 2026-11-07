package main

import (
	"strings"
	"testing"
)

func TestSessionKeyMatchesSessionGatewayGoldenVector(t *testing.T) {
	const token = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	const want = "session:16362f566387b3cf5a6e92fb0a986c76ca20eb3a0c12cbdfbd0b29501e0c18df"

	got := sessionKey("session:", token)
	if got != want {
		t.Fatalf("sessionKey = %q, want %q", got, want)
	}
	if strings.Contains(got, token) {
		t.Fatalf("sessionKey %q contains the raw token", got)
	}
}

func TestSessionKeyUsesConfiguredPrefix(t *testing.T) {
	got := sessionKey("custom:", "3f2504e0-4f89-41d3-9a0c-0305e82c3301")
	want := "custom:16362f566387b3cf5a6e92fb0a986c76ca20eb3a0c12cbdfbd0b29501e0c18df"
	if got != want {
		t.Fatalf("sessionKey = %q, want %q", got, want)
	}
}

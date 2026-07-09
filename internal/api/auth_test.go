package api

import (
	"os"
	"testing"
)

func TestGetClientAuthReturnsDefault(t *testing.T) {
	// When CRUNCHYROLL_CLIENT_AUTH is unset, getClientAuth() must return the
	// compiled-in defaultClientAuth constant.
	orig, had := os.LookupEnv("CRUNCHYROLL_CLIENT_AUTH")
	if had {
		os.Unsetenv("CRUNCHYROLL_CLIENT_AUTH")
		t.Cleanup(func() {
			os.Setenv("CRUNCHYROLL_CLIENT_AUTH", orig)
		})
	}

	got := getClientAuth()
	if got != defaultClientAuth {
		t.Fatalf("getClientAuth() = %q, want defaultClientAuth %q", got, defaultClientAuth)
	}
}

func TestGetClientAuthPrefersEnv(t *testing.T) {
	want := "Basic my-custom-auth-token"
	t.Setenv("CRUNCHYROLL_CLIENT_AUTH", want)

	got := getClientAuth()
	if got != want {
		t.Fatalf("getClientAuth() = %q, want %q", got, want)
	}
}

func TestGetClientAuthEmptyEnvFallsBack(t *testing.T) {
	// When CRUNCHYROLL_CLIENT_AUTH is explicitly set to an empty string,
	// getClientAuth() should return the default — an empty env var is treated
	// as "not provided".
	t.Setenv("CRUNCHYROLL_CLIENT_AUTH", "")

	got := getClientAuth()
	if got != defaultClientAuth {
		t.Fatalf("getClientAuth() = %q, want defaultClientAuth %q", got, defaultClientAuth)
	}
}

func TestGetClientAuthDefaultConstantMatches(t *testing.T) {
	// Verify the defaultClientAuth constant is the known public credential.
	// This test documents the expected value so changes are intentional.
	const expectedDefault = "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6"
	if defaultClientAuth != expectedDefault {
		t.Fatalf("defaultClientAuth = %q, want %q", defaultClientAuth, expectedDefault)
	}
}

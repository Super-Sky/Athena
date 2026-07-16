package app

import (
	"context"
	"testing"
	"time"

	"moss/internal/tools"
)

func TestResolveRemoteToolSecretReadsRotationRevocationAndExpiration(t *testing.T) {
	t.Setenv("ATHENA_TEST_CALLBACK_TOKEN", "first")
	secret, err := resolveRemoteToolSecret(context.Background(), "env://ATHENA_TEST_CALLBACK_TOKEN")
	if err != nil || secret.Value != "first" {
		t.Fatalf("initial secret = %#v, error = %v", secret, err)
	}

	t.Setenv("ATHENA_TEST_CALLBACK_TOKEN", "rotated")
	t.Setenv("ATHENA_TEST_CALLBACK_TOKEN_REVOKED", "true")
	t.Setenv("ATHENA_TEST_CALLBACK_TOKEN_EXPIRES_AT", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	secret, err = resolveRemoteToolSecret(context.Background(), "env://ATHENA_TEST_CALLBACK_TOKEN")
	if err != nil || secret.Value != "rotated" || !secret.Revoked || secret.ExpiresAt.IsZero() {
		t.Fatalf("rotated secret = %#v, error = %v", secret, err)
	}
}

func TestValidateRemoteToolSecretProviderRejectsUninstalledProvider(t *testing.T) {
	if err := validateRemoteToolSecretProvider(&tools.RemoteAuth{SecretRef: "env://CALLBACK_TOKEN"}); err != nil {
		t.Fatalf("env provider error = %v", err)
	}
	if err := validateRemoteToolSecretProvider(&tools.RemoteAuth{SecretRef: "vault://callback"}); err == nil {
		t.Fatal("expected uninstalled provider rejection")
	}
}

func TestResolveRemoteToolSecretRejectsUnsupportedOrMissingReferences(t *testing.T) {
	for _, reference := range []string{"vault://callback", "env://lower-case", "env://ATHENA_MISSING_CALLBACK_TOKEN"} {
		if _, err := resolveRemoteToolSecret(context.Background(), reference); err == nil {
			t.Fatalf("expected reference %q to fail", reference)
		}
	}
}

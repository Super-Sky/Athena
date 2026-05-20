package entry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"moss/internal/config"
)

func TestCheckHealthEndpointAcceptsHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("path = %s, want /healthz", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := CheckHealthEndpoint(context.Background(), srv.URL+"/healthz"); err != nil {
		t.Fatalf("CheckHealthEndpoint() error = %v", err)
	}
}

func TestCheckHealthEndpointRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if err := CheckHealthEndpoint(context.Background(), srv.URL); err == nil {
		t.Fatalf("expected healthcheck error for non-200 status")
	}
}

func TestRunHealthcheckUsesConfiguredPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("path = %s, want /healthz", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: mustPortFromURL(t, srv.URL)},
	}
	if err := RunHealthcheck(cfg); err != nil {
		t.Fatalf("RunHealthcheck() error = %v", err)
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/thebrazenbeard/workbridge/internal/config"
)

func TestBearerMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := requireBearerToken("abcdefghijklmnopqrstuvwxyz123456", next)
	for _, tc := range []struct {
		name string
		header string
		want int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "Bearer nope", http.StatusUnauthorized},
		{"correct", "Bearer abcdefghijklmnopqrstuvwxyz123456", http.StatusNoContent},
		{"case-insensitive-scheme", "bearer abcdefghijklmnopqrstuvwxyz123456", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d", rec.Code, tc.want)
			}
		})
	}
}

func TestExactPathRejectsSubpaths(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := exactPath("/mcp", next)
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/mcp", http.StatusNoContent},
		{"/mcp/", http.StatusNotFound},
		{"/mcp/extra", http.StatusNotFound},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("path=%s status=%d want=%d", tc.path, rec.Code, tc.want)
		}
	}
}

func TestHTTPBearerConfigurationFailsClosed(t *testing.T) {
	cfg := &config.Config{}
	if _, err := loadHTTPBearerToken(cfg); err == nil {
		t.Fatal("HTTP accepted missing bearer-token environment name")
	}
	cfg.HTTP.BearerTokenEnv = "WORKBRIDGE_TEST_TOKEN"
	t.Setenv("WORKBRIDGE_TEST_TOKEN", "")
	if _, err := loadHTTPBearerToken(cfg); err == nil {
		t.Fatal("HTTP accepted empty bearer token")
	}
	t.Setenv("WORKBRIDGE_TEST_TOKEN", "short")
	if _, err := loadHTTPBearerToken(cfg); err == nil {
		t.Fatal("HTTP accepted short bearer token")
	}
	t.Setenv("WORKBRIDGE_TEST_TOKEN", "abcdefghijklmnopqrstuvwxyz12345 ")
	if _, err := loadHTTPBearerToken(cfg); err == nil {
		t.Fatal("HTTP accepted whitespace-bearing token")
	}
	t.Setenv("WORKBRIDGE_TEST_TOKEN", "abcdefghijklmnopqrstuvwxyz123456")
	got, err := loadHTTPBearerToken(cfg)
	if err != nil || got == "" {
		t.Fatalf("valid bearer rejected: %q %v", got, err)
	}
	_ = os.Unsetenv("WORKBRIDGE_TEST_TOKEN")
}

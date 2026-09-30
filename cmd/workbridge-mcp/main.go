package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thebrazenbeard/workbridge/internal/bridge"
	"github.com/thebrazenbeard/workbridge/internal/config"
)

func main() {
	var (
		configPath = flag.String("config", "", "path to WORKBRIDGE_CONFIG_V1 JSON")
		transport = flag.String("transport", "stdio", "MCP transport: stdio or http")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *showVersion {
		fmt.Println(bridge.Version)
		return
	}
	if *configPath == "" {
		*configPath = os.Getenv("WORKBRIDGE_CONFIG")
	}
	if *configPath == "" {
		log.Fatal("config path is required via --config or WORKBRIDGE_CONFIG")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	rt, err := bridge.New(cfg)
	if err != nil {
		log.Fatalf("initialize WorkBridge: %v", err)
	}
	defer rt.Close()

	switch strings.ToLower(*transport) {
	case "stdio":
		if err := rt.Server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	case "http":
		if err := runHTTP(rt); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unsupported transport %q; use stdio or http", *transport)
	}
}

func runHTTP(rt *bridge.Runtime) error {
	cfg := rt.Config
	token, err := loadHTTPBearerToken(cfg)
	if err != nil {
		return err
	}
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return rt.Server
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	protected := requireBearerToken(token, mcpHandler)
	mux := http.NewServeMux()
	mux.Handle(cfg.HTTP.Path, exactPath(cfg.HTTP.Path, protected))
	healthPath := cfg.HTTP.Path + "/healthz"
	mux.Handle(healthPath, exactPath(healthPath, requireBearerToken(token, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"version": bridge.Version,
		})
	}))))

	server := &http.Server{
		Addr: cfg.HTTP.Listen,
		Handler: mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 * 1024,
	}
	log.Printf("WorkBridge MCP listening on http://%s%s", cfg.HTTP.Listen, cfg.HTTP.Path)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func loadHTTPBearerToken(cfg *config.Config) (string, error) {
	name := strings.TrimSpace(cfg.HTTP.BearerTokenEnv)
	if name == "" {
		return "", errors.New("HTTP transport requires http.bearer_token_env")
	}
	token, ok := os.LookupEnv(name)
	if !ok || token == "" {
		return "", fmt.Errorf("HTTP bearer token environment variable %s is not set", name)
	}
	if strings.IndexFunc(token, unicode.IsSpace) >= 0 {
		return "", errors.New("HTTP bearer token must not contain whitespace")
	}
	if len(token) < 32 {
		return "", errors.New("HTTP bearer token must be at least 32 bytes")
	}
	return token, nil
}

func requireBearerToken(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(strings.TrimSpace(r.Header.Get("Authorization")), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") ||
			len(parts[1]) != len(expected) ||
			subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="WorkBridgeMCP"`)
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "bearer authorization required", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func exactPath(expected string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expected {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

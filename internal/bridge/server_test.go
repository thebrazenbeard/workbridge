package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thebrazenbeard/workbridge/internal/config"
)

func TestMCPServerListsAndCallsReadTool(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(file, []byte("hello from workbridge"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Schema: config.Schema,
		ReadRoots: []string{root},
		Limits: config.Limits{MaxReadBytes: 1024, MaxWriteBytes: 1024, MaxDirectoryEntries: 10},
		Process: config.ProcessConfig{MaxRuntimeSeconds: 1, MaxOutputBytes: 1024, MaxArgs: 8},
		HTTP: config.HTTPConfig{Listen: "127.0.0.1:8765", Path: "/mcp"},
	}
	rt, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := rt.Server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "workbridge-test", Version: "0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"workbridge_health", "workspace_list", "workspace_stat", "workspace_read_text"} {
		if !names[name] {
			t.Fatalf("missing tool %q", name)
		}
	}
	if names["workspace_write_text"] || names["workspace_move"] || names["process_run"] {
		t.Fatalf("disabled mutation tool exposed: %#v", names)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "workspace_read_text",
		Arguments: map[string]any{"path": file},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result)
	}
	clientSession.Close()
	serverSession.Wait()
}

func TestMCPServerOmitsReadToolsWithoutReadRoots(t *testing.T) {
	cfg, err := config.Parse([]byte(`{"schema":"WORKBRIDGE_CONFIG_V1","read_roots":[],"write_roots":[],"process":{"enabled":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	rt, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := rt.Server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "workbridge-test", Version: "0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "workbridge_health" {
		t.Fatalf("rootless profile exposed unavailable tools: %#v", tools.Tools)
	}
	clientSession.Close()
	serverSession.Wait()
}


func TestMCPServerExposesMoveOnlyWithWriteRoot(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Schema: config.Schema,
		ReadRoots: []string{root},
		WriteRoots: []string{root},
		Limits: config.Limits{MaxReadBytes: 1024, MaxWriteBytes: 1024, MaxDirectoryEntries: 10},
		Process: config.ProcessConfig{MaxRuntimeSeconds: 1, MaxOutputBytes: 1024, MaxArgs: 8},
		HTTP: config.HTTPConfig{Listen: "127.0.0.1:8765", Path: "/mcp"},
	}
	rt, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := rt.Server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "workbridge-test", Version: "0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "workspace_move" {
			found = true
		}
	}
	if !found {
		t.Fatal("workspace_move missing with write root")
	}
	clientSession.Close()
	serverSession.Wait()
}

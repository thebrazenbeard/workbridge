package bridge

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thebrazenbeard/workbridge/internal/config"
	"github.com/thebrazenbeard/workbridge/internal/runner"
	"github.com/thebrazenbeard/workbridge/internal/workspace"
)

const Version = "0.1.0"

type Runtime struct {
	Server    *mcp.Server
	Workspace *workspace.Service
	Runner    *runner.Runner
	Config    *config.Config
}

type EmptyInput struct{}

type HealthOutput struct {
	Version        string `json:"version"`
	ReadEnabled    bool   `json:"read_enabled"`
	WriteEnabled   bool   `json:"write_enabled"`
	ProcessEnabled bool   `json:"process_enabled"`
}

type PathInput struct {
	Path string `json:"path" jsonschema:"absolute path admitted by configured roots"`
}

type ListOutput struct {
	Entries []workspace.Entry `json:"entries"`
}

type StatOutput struct {
	Stat *workspace.Stat `json:"stat"`
}

type ReadOutput struct {
	Text string `json:"text"`
}

type WriteInput struct {
	Path      string `json:"path" jsonschema:"absolute path admitted by configured write roots"`
	Content   string `json:"content" jsonschema:"UTF-8 text to write"`
	Overwrite bool   `json:"overwrite" jsonschema:"must be true to replace an existing file"`
}

type WriteOutput struct {
	BytesWritten int `json:"bytes_written"`
}

type MkdirInput struct {
	Path    string `json:"path" jsonschema:"absolute path admitted by configured write roots"`
	Parents bool   `json:"parents,omitempty" jsonschema:"create missing parent directories when true"`
}

type MkdirOutput struct {
	Created bool `json:"created"`
}

type MoveInput struct {
	Source      string `json:"source" jsonschema:"absolute source path admitted by configured write roots"`
	Destination string `json:"destination" jsonschema:"absolute destination path admitted by configured write roots"`
}

type MoveOutput struct {
	Moved bool `json:"moved"`
}

type RunInput struct {
	Executable string   `json:"executable" jsonschema:"configured executable grant name, never an arbitrary path"`
	Args       []string `json:"args,omitempty" jsonschema:"arguments passed literally without a shell"`
	WorkingDir string   `json:"working_dir" jsonschema:"absolute configured process working root or descendant"`
}

func New(cfg *config.Config) (*Runtime, error) {
	ws, err := workspace.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	pr, err := runner.New(cfg.Process)
	if err != nil {
		_ = ws.Close()
		return nil, fmt.Errorf("process runner: %w", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "workbridge-mcp", Version: Version}, nil)
	rt := &Runtime{Server: server, Workspace: ws, Runner: pr, Config: cfg}

	mcp.AddTool(server, &mcp.Tool{
		Name: "workbridge_health",
		Description: "Report the WorkBridge version and enabled local capabilities.",
	}, rt.health)
	if len(cfg.ReadRoots) > 0 {
		mcp.AddTool(server, &mcp.Tool{
			Name: "workspace_list",
			Description: "List one admitted directory. Paths outside configured read roots are denied.",
		}, rt.list)
		mcp.AddTool(server, &mcp.Tool{
			Name: "workspace_stat",
			Description: "Stat one admitted filesystem path.",
		}, rt.stat)
		mcp.AddTool(server, &mcp.Tool{
			Name: "workspace_read_text",
			Description: "Read a bounded UTF-8 text file from configured read roots.",
		}, rt.readText)
	}
	if ws.CanWrite() {
		mcp.AddTool(server, &mcp.Tool{
			Name: "workspace_write_text",
			Description: "Create or explicitly overwrite a bounded UTF-8 text file inside configured write roots.",
		}, rt.writeText)
		mcp.AddTool(server, &mcp.Tool{
			Name: "workspace_mkdir",
			Description: "Create an admitted directory, optionally including missing parents, inside configured write roots.",
		}, rt.mkdir)
		mcp.AddTool(server, &mcp.Tool{
			Name: "workspace_move",
			Description: "Move or rename one path within the same configured write root without overwriting an existing destination.",
		}, rt.move)
	}
	if pr.Enabled() {
		mcp.AddTool(server, &mcp.Tool{
			Name: "process_run",
			Description: "Run one SHA-256-pinned executable grant with literal arguments inside a configured working root.",
		}, rt.run)
	}
	return rt, nil
}

func (rt *Runtime) Close() error {
	if rt == nil {
		return nil
	}
	var first error
	if rt.Workspace != nil {
		first = rt.Workspace.Close()
	}
	if rt.Runner != nil {
		if err := rt.Runner.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (rt *Runtime) health(context.Context, *mcp.CallToolRequest, EmptyInput) (*mcp.CallToolResult, HealthOutput, error) {
	return nil, HealthOutput{
		Version: Version,
		ReadEnabled: len(rt.Config.ReadRoots) > 0,
		WriteEnabled: rt.Workspace.CanWrite(),
		ProcessEnabled: rt.Runner.Enabled(),
	}, nil
}

func (rt *Runtime) list(_ context.Context, _ *mcp.CallToolRequest, input PathInput) (*mcp.CallToolResult, ListOutput, error) {
	entries, err := rt.Workspace.List(input.Path)
	return nil, ListOutput{Entries: entries}, err
}

func (rt *Runtime) stat(_ context.Context, _ *mcp.CallToolRequest, input PathInput) (*mcp.CallToolResult, StatOutput, error) {
	value, err := rt.Workspace.Stat(input.Path)
	return nil, StatOutput{Stat: value}, err
}

func (rt *Runtime) readText(_ context.Context, _ *mcp.CallToolRequest, input PathInput) (*mcp.CallToolResult, ReadOutput, error) {
	text, err := rt.Workspace.ReadText(input.Path)
	return nil, ReadOutput{Text: text}, err
}

func (rt *Runtime) writeText(_ context.Context, _ *mcp.CallToolRequest, input WriteInput) (*mcp.CallToolResult, WriteOutput, error) {
	n, err := rt.Workspace.WriteText(input.Path, input.Content, input.Overwrite)
	return nil, WriteOutput{BytesWritten: n}, err
}

func (rt *Runtime) mkdir(_ context.Context, _ *mcp.CallToolRequest, input MkdirInput) (*mcp.CallToolResult, MkdirOutput, error) {
	err := rt.Workspace.Mkdir(input.Path, input.Parents)
	return nil, MkdirOutput{Created: err == nil}, err
}

func (rt *Runtime) move(_ context.Context, _ *mcp.CallToolRequest, input MoveInput) (*mcp.CallToolResult, MoveOutput, error) {
	err := rt.Workspace.Move(input.Source, input.Destination)
	return nil, MoveOutput{Moved: err == nil}, err
}

func (rt *Runtime) run(ctx context.Context, _ *mcp.CallToolRequest, input RunInput) (*mcp.CallToolResult, *runner.Result, error) {
	result, err := rt.Runner.Run(ctx, input.Executable, input.Args, input.WorkingDir)
	return nil, result, err
}

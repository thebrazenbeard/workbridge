# Client setup

WorkBridgeMCP is intended to bridge an MCP client to a workstation under explicit
local capability controls.

## Preferred local mode: stdio

For a desktop MCP client, prefer launching the WorkBridgeMCP binary directly using
the client's local-process configuration.

Conceptually:

```json
{
  "mcpServers": {
    "workbridge": {
      "command": "C:\\Tools\\WorkBridgeMCP\\workbridge-mcp.exe",
      "args": ["--config", "C:\\Users\\you\\.workbridge\\config.json"]
    }
  }
}
```

The exact command-line flags are owned by the core runtime and must be checked
against the release you install.

## Persistent local mode

If the core runtime exposes Streamable HTTP, keep it loopback-only unless you have a
separately reviewed network/authentication design.

A persistent service is not required for stdio clients.

## Configuration principles

Treat configuration as authority, not convenience:
- configure only the filesystem roots the client genuinely needs;
- keep writes disabled unless required;
- keep process execution disabled unless required;
- avoid embedding secrets in service-wrapper XML, repository files, or client config;
- use separate profiles when different clients need materially different authority.

## Verification

After building/installing a release, run:

```powershell
./scripts/Test-WorkBridgeBinary.ps1 -Binary path/to/workbridge-mcp.exe
```

Then run the black-box security cases appropriate to the capabilities you enabled.

# WorkBridge

Standalone, provider-neutral Model Context Protocol (MCP) workstation server
and Synology NAS package source. This repository contains actual Go runtime
source, deterministic test coverage, configuration policy, and an independent
DSM WorkBridgeMedia package (no other application runtime required).

## Components

- `cmd/workbridge-mcp` / `internal/`: Go MCP server. Stdio by default.
  Optional HTTP is restricted to literal loopback and requires a bearer secret.
- `packaging/windows/`, `scripts/`: Windows service and qualification tools.
- `media_bridge/`: Synology DSM 7.2 WorkBridgeMedia package for a *single*
  Media/Library share, served through an optional OpenAI Secure MCP Tunnel.
  The package carries only WorkBridge and tunnel binaries supplied at build;
  it does not embed other orchestration systems.
- `docs/SECURITY.md`: authorization and threat-model boundaries.

## Local development

Requires Go 1.25.12. Then:

```sh
go mod verify
go test ./...
go vet ./...
go build ./cmd/workbridge-mcp
python -m unittest discover -s media_bridge/tests -v
```

To run locally, copy `config.example.json`, set explicit absolute read roots,
and start `workbridge-mcp --config /path/to/config.json`.
Writes require explicit configured write roots. Process execution is disabled
unless separately configured with stable named SHA-256-pinned executable grants.
No wildcard disk access is enabled by source presence.

## Synology package source

`media_bridge/` is an independent candidate derived from a tested standalone
DSM packaging branch. It is currently bound to the exact component binaries
in `media_bridge/component-bindings.json`. The builder intentionally
**refuses arbitrary binaries** if their SHA-256 differs. Its ARMv7 artifacts
and the external OpenAI tunnel adapter must be independently built/obtained,
licensed, and verified. Review `media_bridge/tools/build_spk.py` before use.
The Media share is a *sample bounded deployment profile*, not a default
permission grant on other hosts. The tool grants no remote process execution.

No NAS installation, provider activation, secret provisioning, service restart,
network exposure, or production readiness follows from this source copy.

## Provenance and licensing

See `docs/PROVENANCE.md` for precise code donor commits and excluded legacy
surfaces. Original material remains source-available proprietary (see
`LICENSE`). Separately licensed OpenAI tunnel material retains Apache-2.0
notices under `media_bridge/third_party/`.

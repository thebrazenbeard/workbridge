# WorkBridge Relay for Synology

This repository builds one standalone Synology DSM package:
WorkBridgeRelay-0.1.0-0001-armada38x.spk.

It packages the bounded WorkBridge MCP server and a pinned OpenAI Secure MCP
Tunnel runtime. The tunnel connects outbound; there is no public inbound
listener, Media share requirement, Vera runtime, or other application
dependency. DS216 / DSM 7.2 / ARMv7 is the explicit first target.

The default synology/payload/etc/workbridge-relay.json has empty read/write
roots and disabled process execution. The MCP health tool is available,
but filesystem and process tools are not admitted until an operator grants
their roots and capabilities in installed configuration and separately
verifies package-user filesystem permissions. Changing config alone does
not confer additional DSM filesystem permissions.

## Source directories

- synology/spk: DSM metadata, package-user privilege, systemd, lifecycle
  scripts, and tunnel-credential install wizard.
- synology/payload: outbound tunnel launcher and no-authority base config.
- synology/component-bindings.json: exact ARMv7 binary/source pins.
- synology/tools: deterministic SPK builder and fail-closed archive verifier.
- synology/tests: package security, identity, and lifecycle tests.
- cmd and internal: independent Go WorkBridge MCP runtime.

The included operator diagnostic runs on installed DSM:

    /var/packages/WorkBridgeRelay/target/bin/diagnose-workbridge-relay.sh

It distinguishes healthy/ready from actual MCP qualification; credentials
are never printed. On DSM it also reports MemTotal and MemAvailable
in KiB when the kernel exposes them, otherwise reports unknown. These
snapshots do not establish process RSS or rule out memory pressure. The tunnel listener binds an ephemeral loopback port,
records its address in a private package file, and avoids fixed-port
collisions. One active stdio tunnel-client per unique tunnel ID is required;
do not reuse the existing Lappy/workstation tunnel identity.

See docs/WORKBRIDGE_RELAY_DSM_OPERATOR_RUNBOOK.md for the exact operator-side installation, permissions and readiness gates.

The installer collects tunnel ID and runtime API key using DSM wizard
fields, stores them under package-owned state with mode 0600, and starts
the package-user service. It does not grant any NAS share permission.

## Build and verification

Go 1.25.12 is required for WorkBridge MCP source.

    go mod verify
    go test ./...
    go vet ./...
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -buildvcs=false -ldflags=-buildid= -o /tmp/workbridge-mcp-armv7 ./cmd/workbridge-mcp
    python3 -m unittest discover -s synology/tests -v

The GitHub workflow at .github/workflows/workbridge-relay-spk.yml builds
both pinned ARMv7 components and checks SHA-256 before creating and verifying
the SPK. Its artifact is a candidate for operator review, not deployment
evidence. The tunnel source is fixed at a specific upstream commit and
the six-line ARMv7 portability transform is verified before build.

The builder rejects a replacement binary whose SHA-256 differs from
synology/component-bindings.json. Do not treat an SPK source build as NAS
installation, successful connector pairing, or live MCP behavior.

Original project terms are in LICENSE, third-party notices in
THIRD_PARTY_NOTICES.md, historical donor evidence in docs/PROVENANCE.md.

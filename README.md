# WorkBridge Relay — governed MCP access for Synology DSM

**A source-available Go MCP server and reproducible Synology package build for explicitly admitted local file and process capabilities.**

WorkBridge provides a bounded Model Context Protocol (MCP) runtime (`cmd/workbridge-mcp`) and the source, tests, and verification tooling for a standalone outbound-tunnel Synology package named **WorkBridgeRelay**. A deploying operator owns the NAS, selects authorized filesystem roots and executable grants, provisions the tunnel identity, and verifies the resulting installation. This repository does **not** enroll or install itself on a NAS.

The current package manifest declares **WorkBridgeRelay 0.1.0-0003** for **DSM 7.2+ `armada38x` (ARMv7)**. Those are package targets, not evidence that every device, firmware version, or live connector has been qualified.

## Implemented surfaces

| Component | Source | Verified scope |
| --- | --- | --- |
| MCP server | [`cmd/workbridge-mcp`](cmd/workbridge-mcp/), [`internal/bridge`](internal/bridge/) | Health plus conditionally registered workspace and process tools, under server configuration. |
| File containment | [`internal/workspace`](internal/workspace/), [`internal/policy`](internal/policy/) | Configured root-relative filesystem operations using Go `os.Root`, bounded reads/writes and explicit overwrite rules. |
| Process admission | [`internal/runner`](internal/runner/) | Disabled by default; when enabled, only named, SHA-256-pinned executable grants with bounded arguments, runtime, output, and working roots. |
| DSM package | [`synology/spk`](synology/spk/), [`synology/payload`](synology/payload/) | Package-user lifecycle and outbound tunnel launch, install wizard, local diagnostics and no-authority initial configuration. |
| Reproducible archive | [`synology/component-bindings.json`](synology/component-bindings.json), [`synology/tools`](synology/tools/) | Pinned ARMv7 components, deterministic package construction, hash checks and fail-closed SPK verification. |

WorkBridge does **not** offer unconstrained shell access, a public inbound listener, an all-files NAS browsing entitlement, or a general authorization/consent service. A model request, connector identity, health response, or matching file path does not itself grant protected host permissions.

## Security defaults

- The packaged [`workbridge-relay.json`](synology/payload/etc/workbridge-relay.json) starts with **empty read and write roots**, **process execution disabled**, and bounded request/output limits. Only the health tool is available until the operator explicitly admits additional capabilities.
- The MCP server prefers **stdio**. Optional HTTP mode is loopback-only and requires a separately supplied bearer secret; a public or LAN listener is not an accepted configuration.
- Filesystem operations are rooted and constrained by Go `os.Root`. The source documents remaining limits under hostile local administrators, mount/special-file behavior, and object-identity races; this is not formal filesystem isolation.
- Process grants name an absolute executable path and expected digest. Repeated hashes reduce substitutions but do not cryptographically bind the final checked file to the OS execution handle. Run admitted executables from operator-controlled locations.
- The Synology package uses a **package user** and an **outbound** MCP tunnel; it does not grant NAS share access by installation. Operator-selected roots still depend on actual package-user filesystem permissions.
- Each concurrently active tunnel client must have its **own tunnel identity**. Reusing an existing active identity can cause connection conflicts; do not put a tunnel ID, API key, bearer token, or NAS credential in repository files or public logs.

Read the [security model](docs/SECURITY.md) and the [DSM operator runbook](docs/WORKBRIDGE_RELAY_DSM_OPERATOR_RUNBOOK.md) before a privileged deployment. The runbook contains some **historical 0002 package examples**: verify version, binding digests, and script flags against the current 0003 sources before using them. The package's diagnostic script reports health and selected resource observations, **not** proof of end-to-end authenticated MCP behavior.

## Source build and tests

For the Go WorkBridge MCP server, use **Go 1.25.12**, the version pinned in [`go.mod`](go.mod). The separately pinned OpenAI tunnel-client component has its **own Go 1.27.0 build requirement** in the [component bindings](synology/component-bindings.json); it is not built by `go test ./...` here.

From a clean source checkout:

```sh
# Read-only source check for known disguised editor-startup execution surfaces.
python3 tools/check_editor_startup.py .
python3 -m unittest discover -s tools -p test_editor_startup_guard.py -v

# Go MCP server tests and static analysis (requires the pinned Go toolchain).
go mod verify
go test ./...
go vet ./...

# DSM package/archive, lifecycle, and NAS-preflight tests.
python3 -m unittest discover -s synology/tests -v
```

The [SPK builder](synology/tools/build_spk.py) requires an **exact clean Git HEAD**, two separately built ARMv7 binaries, and binary SHA-256 values matching [`synology/component-bindings.json`](synology/component-bindings.json). It refuses substitutions and writes a deterministic candidate package; the [SPK verifier](synology/tools/verify_spk.py) checks archive structure and provenance. The package workflow compiles the pinned tunnel source with its reviewed ARMv7 portability transform. A passing build is **not** a live NAS installation, entitlement, tunnel pairing, or filesystem-permission probe.

The read-only [`nas_preflight.sh`](synology/tools/nas_preflight.sh) is for an **already authorized DSM shell**. It does not install packages, grant permission, or create a connector. Operators must separately verify the package-user identity, tunnel readiness, disconnect/reconnect behavior, tool policy, and post-test health on their own device.

## Integration and boundaries

The Go runtime is separately useful as a bounded MCP server, but the included Synology packaging scripts and pinned component manifest are **target-specific**; do not present this SPK as a universally compatible application. The [sample Go configuration](config.example.json) and the [package configuration](synology/payload/etc/workbridge-relay.json) use distinct deployment contexts. Adjust policy only after reviewing its real impact on host permissions.

This code does not implement host-owned immutable provider identity, cross-model consent transfer, a coherent external authorization decision service, or an automatic grant to perform effects. Higher-level hosts must authenticate the caller, bind provider and subject identity, evaluate consent and scope, and enforce each decision at the effect boundary.

## Licensing and provenance

Original project material is **source-available proprietary**, not generally open source; the [license](LICENSE) permits limited private noncommercial evaluation. [Commercial licensing](COMMERCIAL_LICENSE.md) requires a separate agreement. The OpenAI tunnel component is independently licensed and its [third-party notices](THIRD_PARTY_NOTICES.md) and [bundled license](synology/third_party/openai-tunnel-client-LICENSE) must be preserved. Do not infer that all other bundled assets have cleared redistribution rights from the presence of a root notice.

Before contributing, read [CONTRIBUTING.md](CONTRIBUTING.md) and [CLA.md](CLA.md). The required legal acknowledgment is a contributor decision, not something a model or automated workflow can silently provide.

For additional evidence and historical decisions, consult the [source provenance](docs/PROVENANCE.md) and [release readiness record](docs/WORKBRIDGE_RELAY_0002_READINESS_AND_REVIEW_20260930.md). These historical records do not establish current deployment.

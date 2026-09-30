# WorkBridgeRelay: requirements, SPK capabilities, and engineering gates

Date: 2026-09-30. Scope: stand-alone Synology DS216 (armada38x / linux armv7), DSM 7.2, package id WorkBridgeRelay. This is a repository design record, NOT an installed-runtime report.

## User-directed requirements (USER_DIRECT, current task and earlier direction)
- Create a general-purpose Synology SPK WorkBridge Relay. Do not substitute a Media Library package; WorkBridgeMedia is neither mandatory nor a runtime dependency.
- Populate meaningful work into thebrazenbeard/workbridge as it is developed; use an isolated Draft PR, leave protected main/installation/credential changes to Patrick.
- Support authorized ChatGPT MCP tool access without reliance on Remote Desktop Commander, public inbound NAS access, or new paid infrastructure.
- Do not silently grant entire NAS or workstation access. Keep filesystem operations scoped to explicitly admitted existing roots; process execution disabled absent an explicit later grant.
- Avoid changing existing Lappy WorkBridge Commander tunnel IDs or connector routes. The NAS relay needs its own separately provisioned tunnel ID; stdio tunnel IDs must not have concurrent live owners.
- Preserve non-secret config across upgrades; keep tunnel credentials out of Git, logs, and CLI argv.

## Verified external capabilities (EXTERNAL_EVIDENCE)
| Feature | Verified documentation | Packaging consequence |
|---|---|---|
| DSM 7.2 SPK format | https://help.synology.com/developer-guide/synology_package/introduction.html | INFO, package.tgz, executable lifecycle scripts, conf/privilege, conf/resource, icons, optional installation wizard |
| DSM 7 privileges | https://help.synology.com/developer-guide/breaking_changes.html and https://help.synology.com/developer-guide/privilege/privilege_config.html | run-as=package, distinct package user; no implicit root authority |
| User service lifecycle | https://help.synology.com/developer-guide/resource_acquisition/systemd_user_unit.html | pkguser-*.service plus synosystemctl; keep supervised single-process tree |
| Start/stop semantics | https://help.synology.com/developer-guide/synology_package/scripts.html | prestart checks, proper status codes, upgrade stop then restart when applicable |
| Package wizard 7.2.2 | https://help.synology.com/developer-guide/synology_package/wizard/WIZARD_UIFILES_v2.html | collect runtime key as password and tunnel ID without embedding credentials |
| Data-share worker | https://help.synology.com/developer-guide/resource_acquisition/data_share.html | data-share creates/grants a named share, so OMIT it rather than injecting Media permissions |
| Default icon size | https://help.synology.com/developer-guide/breaking_changes.html | DSM 7 needs 64x64 standard icon, not donor 72x72 |
| Secure MCP Tunnel | https://github.com/openai/tunnel-client/blob/master/docs/configuration.md | outbound stdio --mcp.command, file: API key reference, main channel, one active process per tunnel ID |
| Private tunnel readiness | https://github.com/openai/tunnel-client/blob/master/docs/onboarding.md | loopback /healthz and /readyz are distinct; prefer ephemeral health port with private URL-file to avoid port collisions |

All linked docs are research context; the exact pinned OpenAI tunnel-client v0.0.15 behavior must also be verified in CI, not inferred solely from current upstream main.

## Architecture: selected
ChatGPT authorized MCP app -> OpenAI Secure MCP Tunnel -> dedicated WorkBridgeRelay tunnel-client -> local stdio WorkBridge Go server -> bounded package-user resources on Synology. The SPK owns only its package user, runtime state, config, service and outbound connection. It is not a generalized trusted relay to Vera or Lappy and is not a messaging queue.

Alternatives considered:
- Port the complete VeraMesh gateway: rejected, introduces identity/runtime coupling, larger attack surface and unsupported dependencies.
- Adopt WorkBridgeMedia as dependency: rejected, grants a share and adds unnecessary service lifecycle.
- Expose WorkBridge HTTP on a NAS/LAN listener: rejected for this version; requires separate network, identity and token hardening, no advantage for local stdio.
- Enable full-NAS root reads/writes or arbitrary process commands: rejected as implicit default authority. Capabilities may be added only through explicit config plus DSM OS permissions.

## Implemented baseline and remaining gates
- Source: Go MCP tool server, policy roots, optional named SHA256-pinned process grants; standalone DSM SPK package-user unit.
- Default installed config: no read roots, no write roots, no process grants. Health tool only until the operator explicitly grants access.
- Installation wizard collects tunnel ID and runtime API key; package mode 0600 state files; no bundled credential.
- SPK arch: armada38x, Go linux/arm GOARM=7; exact binary digests pinned, archive paths/modes checked.
- GitHub exact-head CI must build both ARMv7 binaries from pinned source, run Go + package tests, verify and artifact the SPK.
- Runtime gates NOT MET: NAS install, DSM service manager readback, tunnel connected state, unique NAS tunnel ID, authenticated ChatGPT tool round-trip, accessible operator-selected roots, cross-restart receipts.
- Review gate: internal hostile tests do not constitute independent review.

## Hostile self-review
**Objection (accepted):** A standalone SPK whose default read/write roots are empty can be installed and connect without supplying useful file tools. This is deliberate fail-closed packaging, not completed NAS provisioning. Do not report "full remote NAS access" until an operator configures exactly allowed paths, DSM permissions are verified, and tool calls are observed.

**Objection (accepted):** /healthz proves a process listens; /readyz, control-plane status and authenticated MCP calls are separate proof. DSM status alone must not be presented as tunnel readiness.

**Objection (accepted):** A copied donor package can carry unnecessary permissions and lifecycle assumptions; manifest tests must assert no Media share, correct package user, executable scripts, pinning, and DSM7 icon dimensions.

**Objection (partially accepted):** Fixed health ports can collide with other installed tools; use loopback port 0 and a package-private URL file the exact pinned v0.0.15 source at a390c168ff1b2d14e73a95991c186c6aba3ff5a0 confirms both flags in pkg/runtimeconfig/config.go. Log and tool output must not reveal secrets.


## Implemented follow-up (same Draft PR)
- Replaced 72px donor icon with DSM7 64px PNG; verifier enforces both image sizes.
- Added package-private health URL with random loopback port, bounded restarts and service descendant teardown.
- Added read-only diagnostic distinguishing healthy/ready from unknown MCP behavior without revealing credentials.
- Included original source LICENSE in SPK, exact archive inventory, pinned component hashes, and adversarial tests.

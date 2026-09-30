# WorkBridgeRelay live verification record — 2026-09-30

## User-directed installation and report

Evidence class: USER_DIRECT. Patrick provided a Synology DSM Package Center
screenshot showing WorkBridgeRelay (beta) package version 0.1.0-0001 installed
on Volume 1 and the Package Center status displayed as Running. This is
evidence of the reported DSM UI state, not an independent on-NAS service,
resident-memory, file-grant or process-level audit. Patrick performed the
installation through DSM/QuickConnect; this Project did not install the SPK.

Evidence class: USER_DIRECT. Patrick provided OpenAI Platform screenshots
showing a distinct Workbridge Relay tunnel registered and a dedicated key
displayed Active; these screens do not prove tunnel readiness, active transport
or a verified physical host. No secret values are recorded here.

## ChatGPT connector observation

Evidence class: RUNTIME_OBSERVATION, limited to an authenticated plugin action
in the ChatGPT host session. The Workbridge Relay connector's exposed
workbridge_health tool was invoked successfully on 2026-09-30 and returned
these exact non-secret fields:

    version = 0.1.0
    read_enabled = false
    write_enabled = false
    process_enabled = false

This verifies that the **selected ChatGPT Workbridge Relay connector can
perform a real MCP health call**, not merely that a tunnel exists. It does NOT
by itself independently bind the returned process to the physical DS216,
attest the server executable hash, provide per-process RSS, demonstrate
tunnel-control-plane status, or prove any filesystem/process capability.

No read/write/process permission change was requested or performed, and
the default disabled capabilities remain appropriate until specific
operator-approved paths/permissions have been reviewed.

## Outstanding verification and operational gates

- Obtain NAS-local diagnostic output from the installed package doctor
  (the script prints MemTotal, optional MemAvailable, and local health/ready
  outcomes, not credentials) through an authenticated operator session.
- Measure the actual aggregate memory of the tunnel and WorkBridge MCP
  child processes under realistic DS216 load. GOMEMLIMIT is not an RSS cap.
- Verify the physical-device binding independently from the tunnel record
  before describing this instance as qualified DS216 runtime.
- Before enabling any read/write root, require exact operator approval
  for the path(s), filesystem ACL verification, and negative out-of-root tests.
- No repository merge, key rotation, NAS service restart, new share grant,
  or provider mutation follows automatically from this observation.

## Repository provenance

Record produced against WorkBridgeRelay Draft PR #1 at source head
aa494202c629e2ca5a818b2ec1bea3a732fcc83e.
The source checks and pinned ARMv7 SPK workflow passed at that head.
Source and tests are not equivalent to live device qualification.

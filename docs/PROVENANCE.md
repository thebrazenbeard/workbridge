# Provenance and extraction decision

## Verified source cuts (30 September 2026)

- Generic Go MCP server: \`thebrazenbeard/WorkBridgeMCP\` at
  \`f51a75d927346a172bb7cc3e4e3f18cd3f52d701\`.
  Copied: \`cmd/\`, \`internal/\`, Go module dependencies, default config,
  selected Windows scripts/package metadata and general security documentation.
  Module import path changed to \`github.com/thebrazenbeard/workbridge\`.
- Synology standalone media bridge: \`thebrazenbeard/vera-synology\` at
  \`6e18c262d8837e895f1237520112251cdd01858c\`,
  source subdirectory \`media_bridge/\` from Draft PR #11.
  Copied intact apart from neutral package maintainer and brand-new generic
  WB logo PNGs replacing original icon bytes.
  Retained exact third-party copyright/license notices and binary source pins.

## Intentionally excluded

- VeraMesh/Relay/Port, gateways, lifecycle managers, identity/adoption logic,
  protocol bindings, embedded sealed-relay snapshots, and NAS edge proxy.
- Vera-specific workflows, protocols, example credentials, tools, generated
  binary artifacts, system roots, control-plane policies, internal histories,
  and historical continuation material.
- Desktop Commander upstream Git submodule and duplicate-runtime mode.
  WorkBridge's bounded Go MCP server is independent of that surface.

Historical source identity is retained **only here**, not in installed app
names, service units, package IDs, code or runtime configuration.

## Evidence and limitations

Both donors are public repositories belonging to the same GitHub owner.
Do not infer that public source availability grants third parties redistributable
or commercial rights. Existing Patrick Sims license and third-party licenses are
preserved. This is a source extraction, not a production cutover or relabeling
of installed NAS services.

The Synology packaging builder currently pins a historical tested WorkBridgeMCP
ARMv7 binary and tunnel runtime. Regenerate/requalify bindings before treating
a build of the *new root module* as equivalent to that binary. The migration
does not claim that the new module has shipped to DSM.

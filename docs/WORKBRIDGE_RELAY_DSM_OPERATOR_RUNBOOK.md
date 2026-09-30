# WorkBridgeRelay DSM operator runbook

Status: repository source and candidate-SPK evidence only. Installation, provider connection and access grants have not been performed.

## Intended single-device route
ChatGPT WorkBridgeRelay connection -> OpenAI Secure MCP Tunnel -> DSM WorkBridgeRelay service -> stdio WorkBridge MCP server. This SPK is independent of the existing Lappy WorkBridge Commander app/connection. Provision a distinct tunnel ID; the upstream client does not support simultaneous stdio clients sharing one ID.

## Package qualification before any live effect
1. Check the exact commit, open Draft PR, WorkBridgeRelay ARMv7 SPK Actions outcome and attached SHA256. The workflow artifact (not a random upload) is the candidate.
2. Confirm DSM is 7.2.2+ as required by the wizard format and NAS target is armada38x. Package is marked beta pending on-NAS verification.
3. Verify SPK using the checked-in synology/tools/verify_spk.py and verify published SHA256. Check explicit privilege, no data-share resource grant, 64px DSM7 icon and mandatory executable script modes.
4. Do not import a donor SPK or reuse the old Media-specific package under a new name.

## Installation and credentials (requires explicit authority)
Install manually through DSM Package Center only after Patrick authorizes exact target and effect. The install wizard requests a dedicated tunnel ID and a runtime API key; enter them in DSM only, never in ChatGPT, GitHub, logs, or terminal history. Values are written mode 0600 inside package private var state. Package runs as user WorkBridgeRelay with UMask 0077. No credential is embedded in the SPK. There is no public inbound port, and no named share is automatically created or granted.

## File and process authority
Default config: /var/packages/WorkBridgeRelay/var/workbridge-relay.json
- read_roots: [] (no filesystem reads)
- write_roots: [] (no filesystem writes)
- process.enabled: false (no process tool)
This is intentionally useful for health and transport qualification first, NOT proof of file access. For later explicit operator configuration, set only specific absolute existing paths in read_roots, and only narrower explicitly authorized write_roots. Obtain/check DSM filesystem permission for the package user independently; JSON roots cannot confer filesystem permission. Do not use broad /, /volume1, or system folders by default. Process grants, if ever enabled, require named executable path+SHA256 and working-root bounds; they are a separate security review and protected effect.

Preserve the operator's existing config on upgrade. Do not auto-grant a share using DSM data-share merely to make a path work.

## Post-install verification gates (not yet performed)
1. Observe real DSM Package Center status and synosystemctl status for pkguser-workbridgerelay.service.
2. As the installed package user, run /var/packages/WorkBridgeRelay/target/bin/diagnose-workbridge-relay.sh. It prints MemTotal and MemAvailable snapshots if available (otherwise unknown) before health checks; do not treat them as process RSS or memory headroom proof. It distinguishes missing files, tunnel process health (/healthz), and ready tunnel (/readyz); output deliberately never reports the secret values.
3. Validate the dedicated tunnel in its provider's control plane, then observe authenticated MCP initialize/tools/list/tool-call from the exact selected ChatGPT connector. /readyz does not prove authorized ChatGPT access.
4. With no roots configured, confirm only a bounded health tool is registered and filesystem tool calls cannot proceed. After separately authorized grants, check in-root paths succeed, out-of-root paths fail, writes are absent unless specifically granted, and process_run remains absent.
5. Test DSM controlled stop/start and post-upgrade return of service only after authorizing an isolated maintenance window, verifying no second active process owns the tunnel ID and preserving the previously trusted service route.

## Recovery / incidents
- Differentiate package-active, local-health, tunnel-ready and authenticated-MCP-success states; none alone proves the next.
- A missing private health URL file may mean startup has not completed; do not adopt a foreign listener or clear another service's state. Tunnel binds to an ephemeral loopback port to avoid fixed-port collisions.
- On startup failure, collect non-secret DSM package logs, run doctor, and verify exact binary hashes, package state, and package-user permissions before changes.
- Never expose the local tunnel admin UI to a public/LAN bind or disable TLS trust checks to work around reachability.
- Do not delete package state, revoke credential, restart unrelated services, or mutate protected deployment state without exact authorization.

## Source references
Synology: https://help.synology.com/developer-guide/synology_package/introduction.html
Synology lifecycle: https://help.synology.com/developer-guide/synology_package/scripts.html
Synology privileges: https://help.synology.com/developer-guide/privilege/privilege_config.html
Synology unit: https://help.synology.com/developer-guide/resource_acquisition/systemd_user_unit.html
OpenAI client: https://github.com/openai/tunnel-client/blob/a390c168ff1b2d14e73a95991c186c6aba3ff5a0/docs/configuration.md

## Installer/upgrade filesystem safety
The DSM postinst/postupgrade scripts reject symbolic links for package-managed
state and its credential/config file targets. New files use unpredictable,
exclusive mktemp names with umask 077 and trap cleanup on failure; configuration
is staged before moving credentials into place. An upgrade preserves existing
operator configuration and secrets rather than silently rotating credentials.
Credential files are replaced individually; **the pair is not transactionally
atomic if an unexpected failure occurs between renames**. If an installation
fails mid-commit, stop and verify the intended tunnel ID and key pair using a
private operator channel before restarting; never print secrets into a ticket,
CI log, or chat. These protections are tested in Linux shell fixtures and do
not substitute for real DSM installation/permissions verification.

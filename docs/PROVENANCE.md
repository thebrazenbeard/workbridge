# WorkBridge Relay provenance

Source basis as of 2026-09-30: owner-held WorkBridgeMCP Go server at
thebrazenbeard/WorkBridgeMCP commit
f51a75d927346a172bb7cc3e4e3f18cd3f52d701.
Synology packaging donor was the standalone experimental Media profile at
thebrazenbeard/vera-synology commit
6e18c262d8837e895f1237520112251cdd01858c.
These references record history, not runtime dependencies.

The earlier donor's hardcoded Media root, DSM share grant, prestart check,
unit/package name, launcher filename, and tests have been removed and
replaced. The new WorkBridgeRelay package has no Media dependency. It
grants no filesystem roots or process tool authority by default.

The portable Go module path is github.com/thebrazenbeard/workbridge.
A linux/armv7 WorkBridge binary was built under Go 1.25.12 and its hash
pinned in synology/component-bindings.json. OpenAI tunnel-client source
is fixed at a390c168ff1b2d14e73a95991c186c6aba3ff5a0, v0.0.15,
with a documented narrow compile-assertion portability transform.

Vera-related relay, runtime identity, gateway, and memory code was excluded.
Source tests, SPK hash checks and CI are not proof of actual DSM install,
private provider binding, filesystem authorization, or end-to-end behavior.

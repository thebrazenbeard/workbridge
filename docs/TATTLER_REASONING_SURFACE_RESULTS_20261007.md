# Tattler reasoning-surface results — 2026-10-07

Status: OBSERVATION NOTE / TRANSPORT BOUNDARY

## Shared experiment result

On 2026-10-07, the same repository stress-test prompt was run through three ChatGPT surfaces while WorkLaptop was instrumented with Tattler plus a companion Codex process/network tracer.

Observed controlled windows:

- Desktop Chat, GPT-5.6 Sol High: **0 MXC launches** and **2 new established Codex TLS connections** in the companion tracer.
- ChatGPT Desktop Work, Ultra: **59 MXC launches** and **73 new established Codex TLS connections** using the same companion-tracer definitions.
- Firefox cloud Work, Max: browser-side traffic was observable locally, but the provider's server-side worker topology was not.

The experiment supports a bounded conclusion: Desktop Work used materially different local orchestration from ordinary High Chat in this runtime. It does **not** establish that sockets or MXC processes equal agents, that connection fanout grants a reasoning tier, or that a client can promote High into Ultra/Max by imitating transport behavior.

Canonical detailed evidence is being preserved in `thebrazenbeard/tattler` PR #7 and the reasoning interpretation in `thebrazenbeard/rezon` PR #103.


## Why WorkBridge Relay needs this result

This repository packages WorkBridge plus an outbound secure tunnel. The Tattler experiment reinforces a transport rule that matters here:

```text
tunnel/session count != reasoning-agent count
transport concurrency != reasoning tier
connection reuse/fanout != model entitlement
```

A WorkBridge Relay diagnostic may report tunnel/session/process health, but those observations must remain transport/runtime evidence. They should not be interpreted as proof that a remote client is using High, Ultra, Max, or any other reasoning configuration.

## No required implementation change

The current relay should remain model-agnostic. The result is useful as a claim ceiling for future diagnostics and experiment receipts, not as a reason to add model-routing logic to this repository.

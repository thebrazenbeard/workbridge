# WorkBridge Connector Stress Observation — 2026-09-30

> The first bounded connector checks below are preserved as historical evidence. They are **superseded for capacity claims** by the full 64-upstream / 8-execution saturation run later in this document.

## Scope and authority

Patrick explicitly requested stress testing of the connected Workbridge Relay and Workbridge Commander MCP apps. This was a bounded interactive test through live app actions, not a synthetic 60-minute soak test or a destructive capacity benchmark.
All relay requests used the exposed workbridge_health method. No NAS process execution, filesystem access, ACL/credential modification, package restart or installation was attempted.
Commander operations targeted only read-only metadata, a local repository listing, read-only README excerpts, configuration introspection, usage statistics, session listing, and short non-writing Windows PowerShell jobs.

## Workbridge Relay — observed host tool results

The relay returned version 0.1.0 and read_enabled=false, write_enabled=false, process_enabled=false before and after load.
Eight sequential timed health calls: 8 of 8 successful, no inconsistent capabilities. Per-call observed end-to-end latency ranged 335–730 ms; median observed approximately 402 ms.
Three waves of three concurrent calls: 9 of 9 successful. Measured wave wall-clock intervals were 1155, 1242, and 1066 ms.
Three waves of four concurrent calls: 12 of 12 successful. Measured wave wall-clock intervals were 1485, 1629, and 1915 ms.
Total measured load calls: 29 of 29 successful; separate baseline and post-stress health probes succeeded. No capability drift was observed.
One separate attempted 24-call orchestrator batch exceeded the host's per-invocation tool-call limit without a result report. Its attempted calls are deliberately excluded from counted results; the failure is not assigned to the relay.

## Workbridge Commander MCP — observed host tool results

Initial configuration and usage-statistics requests succeeded (2 of 2), without recording credentials or returned configuration values in this artifact.
Baseline read-only file metadata, directory listing, and a short local PowerShell check: 3 of 3 successful.
Three concurrent waves of read-only README excerpt, metadata, and repository directory listing: 9 of 9 successful; wall-clock intervals 974, 1218, and 988 ms per three-operation wave.
Three waves of two short PowerShell jobs with a bounded 25,000-step integer arithmetic loop and a completion marker: 6 of 6 successful; wave intervals 11,886, 12,532, and 4720 ms. These intervals include orchestration, startup, and scheduling delays.
Post-stress file metadata and session-list checks: 2 of 2 successful.
Total measured Commander requests across baseline, load and post-stress checks: 22 of 22 successful. None of those stress-test commands wrote files, modified workstation configuration, or executed against the NAS. This evidence document was written separately after testing.

## Interpretation and hostile review

> HOSTILE REVIEWER: A health-only stress test cannot establish NAS runtime stability under real filesystem workloads, sustained concurrency, large payloads, provider outages or memory pressure.

ACCEPTED. The connected Relay has only a health action exposed and its read/write/process capabilities are intentionally disabled. The successful calls establish bounded availability of that app method under the tested request arrival pattern, nothing stronger.

> HOSTILE REVIEWER: Long wall-clock time for Commander PowerShell calls may be dominated by connector and process-launch overhead rather than computation.

ACCEPTED. Reported wave times are end-to-end request measurements. No CPU benchmark claim, host-level throughput result or percentile reliability guarantee is inferred from the small sample.

The original DS216 has only 512 MiB physical RAM. These tests did not directly sample DS216 MemAvailable, RSS of the tunnel parent and MCP child, swap, CPU utilization, OOM events, or NAS disk I/O; those remain UNKNOWN. The successful version string 0.1.0 does not distinguish installed DSM package revision 0001 from the source-only 0002 upgrade candidate.

## Outcomes and follow-up

No observed request failures or permission changes in counted batches. This is INTERNAL host-observed evidence and not independent hardware attestation.
Before authorizing larger load or enabling file operations, obtain a device-local memory/process diagnostic, establish a conservative load and stop threshold, and explicitly approve the intended workload. Do not automatically enable roots, reuse a tunnel identity, merge the Draft PR, or replace the installed package on the basis of this test alone.

## Full live saturation run — 64 upstream contexts / 8 execution lanes

This run supersedes the preliminary bounded checks for any concurrency/capacity claim. The live WorkBridge Commander loopback health endpoint reported:

- connectedDeviceCount: 1;
- device: lappy, generation 1;
- executionCapacityPerDevice: 8;
- upstreamContextCapacity: 64;
- both qualification-floor checks true.

The harness used the already-present local WORKBRIDGE_CLIENT_TOKEN only from process environment and never printed or persisted its value. It opened 64 direct loopback MCP requests to the same live Commander service. Each request called the existing start_process tool with a six-second bounded sleep/marker command. Commander therefore admitted all 64 upstream contexts while its own per-device execution pool scheduled at most eight effects at a time. The harness sampled /health every 100 ms until all requests completed.

Raw evidence is committed as docs/evidence/WORKBRIDGE_COMMANDER_STRESS_64X8_20260930.json. Its exact SHA-256 before repository commit was:
43a46e6c7c8791b0146ef0645ab9aa18c79895e981bcfafaad14180b0303687e

Observed live peaks and completion:

- configured upstream capacity: 64;
- peak upstreamContextActive: **64**;
- peak upstreamContextQueued: **0**;
- configured execution capacity: 8;
- peak executionActive: **8**;
- peak executionQueued: **56**;
- health samples during run: 952;
- successful requests: **64 / 64**;
- failed requests: **0**;
- post-drain upstream active/queued: **0 / 0**;
- post-drain execution active/queued: **0 / 0**.

The execution queue observation is itself simultaneous evidence: executionActive=8 plus executionQueued=56 accounts for all 64 admitted effect-bearing contexts while those contexts remain live in the upstream pool.

End-to-end request-duration distribution, including queueing, remote-tool dispatch, process launch/state detection, the six-second sleep, and response propagation:

- min: 6,340 ms;
- p50: 51,278 ms;
- p90: 94,650 ms;
- p95: 100,362 ms;
- p99: 101,518 ms;
- max: 101,518 ms.

This proves capacity admission and eventual completion under the tested workload. It does **not** prove that a six-second workstation operation completes in six seconds under saturation. The tail latency is materially above the idealized eight-batches × six-seconds lower bound, so downstream process startup/state-detection/scheduling overhead remains a performance frontier rather than being hidden by the 64/64 success count.

## Relay overlap during full Commander saturation

While the 64-context Commander harness was active, Workbridge Relay received five independent waves of eight concurrent workbridge_health calls: **40 / 40 succeeded**. Per-wave end-to-end wall times were approximately 3.26 s, 3.66 s, 3.74 s, 4.82 s, and 4.46 s. Every returned payload remained version 0.1.0 with read_enabled=false, write_enabled=false, process_enabled=false.

Relay therefore remained available through this cross-connector load. This does not expose or measure DS216 CPU, aggregate RSS, swap, OOM behavior, or NAS disk I/O; those remain separate on-device qualification questions.

## Recovery and resource observations

Before saturation, the Lappy snapshot showed approximately 13,610.2 MiB free of 32,490.8 MiB visible RAM, 7% sampled CPU load, and 353 processes. Immediately after drain: 12,987.6 MiB free, 16% sampled CPU, 356 processes; the detached harness process was no longer alive and Commander reported no active sessions. Thirty seconds later: 13,063.3 MiB free, 18% sampled CPU, 354 processes.

The memory delta is **not classified as a leak** from these snapshots. Windows filesystem cache, process/runtime churn, other workstation activity, and sampling timing are uncontrolled. The evidence supports clean protocol/session drain and near-baseline process count, not a causal memory-leak verdict.

## Harness failures excluded from the full result

Two earlier harness attempts are preserved but excluded from the 64/64 result. The first failed before load because native PowerShell-to-Python argument quoting removed a Python string delimiter. A later 8-call emulation attempt executed seven calls but one was rejected by the ChatGPT host safety layer; that run also did not exercise Commander's actual 64-context orchestrator directly. Neither is counted as a Commander capacity failure.

The final harness used direct loopback MCP requests and Commander's own health counters. Its source script remained in Lappy scratch only; its SHA-256 was 4062e6e326086eaf6d20fff8da0671d3c989d559debfbe780fff5cf03514f911.

## Reconciliation / PR-cleanup discipline

The test followed the Project Runner repository-reconciliation pattern used for PR cleanup: preserve predecessor evidence instead of rewriting it away; bind the exact live subject before execution; separate admission/capacity evidence from deployment authority; keep collision/effect boundaries explicit; exclude ambiguous/failed harness attempts from successful counts; verify terminal state independently; and do not turn passing evidence into merge, install, credential, permission, or deployment authority.

> **HOSTILE REVIEWER:** 64 admitted upstream contexts and eight active execution slots do not prove useful throughput if end-to-end tail latency doubles the ideal batch schedule.

**ACCEPTED.** Capacity and throughput are different claims. The run proves the live 64/8 admission model and 64/64 eventual completion. The 101.5-second tail for six-second effects shows that saturated process execution has meaningful downstream overhead and should be treated as a performance issue if low latency under full saturation is a product requirement.

> **HOSTILE REVIEWER:** Relay health calls are cheap and do not represent filesystem or process stress on the DS216.

**ACCEPTED.** The installed Relay intentionally exposes only workbridge_health because read/write/process remain disabled. Forty successful overlapping health requests prove bounded connected-route availability under concurrent Commander load, not heavy NAS-workload capacity.

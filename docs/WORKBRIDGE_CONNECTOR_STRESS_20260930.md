# WorkBridge Connector Stress Observation — 2026-09-30

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

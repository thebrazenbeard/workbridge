# DS216 hardware resource profile

Status: engineering source configuration, 2026-09-30. Target: original DS216, not DS216+, DS216play or DS216se.

## Manufacturer-verified hardware
- CPU: Marvell Armada 88F6820, dual-core at 1.3 GHz; architecture armada38x / 32-bit ARMv7.
- RAM: 512 MB DDR3 SHARED by DSM, filesystem cache, and installed packages. It is not the available WorkBridge budget.
- Two SATA drive bays; one Gigabit Ethernet connector.
- Operator-reported DSM 7.2.2; INFO os_min_ver=7.2-72806.
References:
https://global.synologydownload.com/download/Document/Hardware/DataSheet/DiskStation/16-year/DS216/enu/Synology_DS216_Data_Sheet_enu.pdf
https://www.synology.com/en-global/company/news/article/Synology_Announces_DS216
https://help.synology.com/developer-guide/appendix/platarchs.html

## Engineering choices (INFERENCE, not NAS measurements)
- Service Environment=GOMEMLIMIT=96MiB. This is a soft Go runtime GC target PER PROCESS, not resident memory, a hard cgroup limit or an aggregate service cap. Both the tunnel and WorkBridge Go child inherit this environment.
- Service Environment=GOMAXPROCS=1 per process, leaving CPU capacity for DSM; does not strictly limit every OS thread or workload spike.
- Service Environment=GOGC=75 balances garbage-collection frequency and heap growth, to be measured in real service later.
- Outbound pinned tunnel client local control-plane.max-inflight=4; MCP dispatch max-concurrent-requests=1. The first setting bounds buffered poll commands, not a durable queue.
- Fresh-install text read/write max=524288 bytes per request; directory-list max=256 entries. No read/write roots by default.
- Process execution disabled; dormant max-concurrent=1, max-runtime=30 seconds, max-output=262144 bytes, max-args=32.
- No bundled runtime DB, Node.js, reverse proxy, Docker container, bulk transfer queue or verbose disk logs. A restart-limited user service manages a stdio child process.
- Existing operator-modified config preserved on upgrade; limits do not silently override it.

## Source evidence
Pinned OpenAI tunnel-client commit a390c168ff1b2d14e73a95991c186c6aba3ff5a0 defines both concurrency flags in pkg/runtimeconfig/config.go and uses exec.Command without setting Cmd.Env for the stdio child (pkg/mcpclient/stdio_command.go). Go documents soft GC targets and GOMAXPROCS at https://pkg.go.dev/runtime#hdr-Environment_Variables.
ARMv7 source build: CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7; CI checks ELF32/ARM, binary SHA256 pin, SPK archive and tests.

## Lightweight operator resource snapshot
The read-only package doctor reads /proc/meminfo and prints
memory_total_kib and memory_available_kib. If the kernel does not expose
MemAvailable, the result is explicitly unknown instead of a made-up
estimate. This output is a snapshot of NAS memory, not measured Go
RSS, peak consumption or a safety qualification. The operator can also
inspect DSM Resource Monitor and process-specific memory usage after
separate installation approval.

## Hostile self-review
OBJECTION (ACCEPTED): A 96 MiB soft target does not prove the NAS fits 512 MB RAM. TLS, Go stacks, page cache, DSM services and other packages can increase RSS or trigger OOM. Source tests cannot prove runtime consumption.
RESPONSE: After an explicitly authorized install, inspect measured process RSS/peak, swap use, DSM memory pressure, CPU idle and bursts, disk impact and OOM logs while running real MCP traffic. Prefer reducing load or disabling this optional relay if DSM is unstable. Do not assume DSM supports cgroup memory limits until proven on device.

OBJECTION (ACCEPTED): 512-KiB payload and 256-entry listing caps block certain legitimate workloads.
RESPONSE: Treat the DS216 as a lightweight bounded control interface; use another workstation for bulk tasks. Only widen per-operation budgets after explicit operator approval and live resource evidence.

CLAIM CEILING: source-level design and test qualification, NOT DSM installation, memory benchmark, provider connection, or current route.

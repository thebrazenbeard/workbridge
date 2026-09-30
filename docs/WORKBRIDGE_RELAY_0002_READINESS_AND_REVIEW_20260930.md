# WorkBridgeRelay 0.1.0-0002 — upgrade candidate (not installed)

## Source and observed state

Patrick installed 0.1.0-0001 on the original DS216. DSM Package Center showed
Running and an actual ChatGPT Workbridge Relay connector call to
workbridge_health returned version 0.1.0 with read/write/process all false.
These observations establish the displayed DSM state and a real MCP health
call, not independent physical-host identity, memory use or binary digest.

The former workspace_move checked the destination with Lstat and later
called os.Root.Rename. Another process could create the target in between,
so that check could not guarantee a non-overwriting move.

Fixed Go source commit: 4a34fdcaeb873e2316d2e61e78c2e204138fb8cb.
It atomically claims an absent target using os.Root.Link before unlinking
source, and admits only regular files. No fallback to a clobbering rename.

This source change does not mutate the already installed 0.1.0-0001 SPK.
A new 0.1.0-0002 SPK is an explicitly separate candidate upgrade.

## Exact version bindings

- WorkBridge Go source at 4a34fdcaeb873e2316d2e61e78c2e204138fb8cb,
  built with Go 1.25.12, CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7,
  -trimpath -buildvcs=false -ldflags=-buildid=.
- WorkBridge ARMv7 SHA256:
  3d930a0ab833270e9733091f300ef8fecbfb524c9a49ee44fb5de2fbceebf6ee.
- OpenAI tunnel client remains v0.0.15 from
  a390c168ff1b2d14e73a95991c186c6aba3ff5a0.
- Tunnel ARMv7 SHA256:
  3c27d0e9d7dc44488704a3c1687155b7fb5cf80b1fcd3c3d78fac1494229e671.
- Package ID WorkBridgeRelay, revision 0.1.0-0002, arch armada38x,
  DSM min 7.2 build 72806.
- MCP bridge.Version stays 0.1.0, a separate protocol/software version:
  workbridge_health.version alone cannot attest the exact installed SPK.
  Obtain DSM Package Center version and NAS-local digest to verify upgrade.
- Independently qualified CI release requires successful new-head
  GitHub Actions exact pin comparison, Go tests and SPK archive verification.

## Hostile design review

> HOSTILE REVIEWER: Hard-link-then-unlink is not an atomic multi-path
> rename: a crash after link can leave both names; filesystems may refuse
> hard links; crossing nested mounts may fail; directories cannot move.

ACCEPTED. The previous check-then-rename allowed overwriting under races.
The replacement guarantees an absent destination at the link creation step
for regular files, and fails closed for other inputs. If source deletion
fails, the operation reports an error identifying the partial state.
It is not a transaction or backup mechanism. Never blindly retry a failed
partial operation without inspecting both names.

> HOSTILE REVIEWER: Passing Windows tests and ARM cross-build does not
> demonstrate DS216 kernel and filesystem behavior or aggregate resident
> memory consumption of tunnel plus MCP child.

ACCEPTED. Hardware compatibility remains an on-device verification gate.
GOMEMLIMIT is a per-Go-runtime soft GC target, not a hard RSS constraint.

## Operator upgrade gate (not authorized merely by this PR)

Do not interrupt an active NAS service while Patrick is away/driving.
When Patrick explicitly elects a maintenance window:
1. Preserve the working 0.1.0-0001 version, private config and dedicated
   tunnel credentials. Confirm a recovery path and DSM access.
2. Download exact verified 0.1.0-0002 CI SPK plus its SHA256, verify both.
3. In DSM Package Center manually upgrade the same WorkBridgeRelay package.
   Postupgrade preserves settings and credentials. No new share is granted.
4. Check Package Center version and service status, NAS free/available RAM,
   actual process RSS and local health/ready diagnostic.
5. Recheck the selected ChatGPT connector's workbridge_health response.
   Read/write/process should all remain disabled.
6. If anything fails, collect non-secret logs and diagnose before changing
   credentials, package state or tunnel identity.

No merge, production routing change, credential change, automatic install,
service restart, file grant or independent review is claimed here.

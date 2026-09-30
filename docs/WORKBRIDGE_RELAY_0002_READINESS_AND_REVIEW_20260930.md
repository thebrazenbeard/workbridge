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

## Packaging provenance and outage recovery hardening

The SPK builder now rejects malformed or mismatched source-head arguments
and refuses to produce a package when tracked source files differ from
the checkout HEAD. The verifier checks that embedded source-head provenance
is a full 40-character Git SHA and, when provided, matches the independently
supplied expected Git SHA. CI explicitly passes its exact checkout HEAD
to the verifier. This is **build provenance checking**, not external
attestation of the physical NAS.

DSM service restart spacing is raised from 15 to 90 seconds while retaining
the five-starts-per-300-seconds ceiling. On an original DS216 this avoids a
transient WAN outage causing five immediate failures and permanent
StartLimit lockout, while reducing failed retry pressure on the NAS. It
does not replace an on-device network outage or restart test and can
introduce up to roughly a 90-second retry delay after a failed exit.
No running service was restarted or reconfigured by this source change.


## Archive verification resource ceiling and trust boundary

Untrusted SPK/tar archives are now rejected if the outer SPK exceeds 48 MiB,
a member exceeds 32 MiB, the aggregate uncompressed entries exceed 64 MiB,
or either archive has more than 64 members. The verifier checks header sizes
as members are read, avoiding an unbounded getmembers() scan before bounds.
This is a verification-side DOS safeguard, not an on-NAS RAM budget.

Critically, the SPK's embedded SHA256 manifest is NOT independently trusted.
The verifier checks embedded component bindings against the repository's
exact checked-out synology/component-bindings.json; replacing a binary and
rewriting its self-reported hash cannot by itself pass validation. Source
checkout provenance must still be trusted: a local modified repository is not
an independent external attestation. New tests forge each binary digest and
assert rejection. The CI verifier also accepts the exact expected source
commit explicitly from git rev-parse HEAD.


## GitHub pull-request checkout identity

GitHub Actions normally checks out a synthetic pull-request merge ref
unless checkout is given an explicit ref. The SPK job now supplies
the PR's actual head SHA (falling back to github.sha for push/manual
workflows), and checks git rev-parse HEAD against that value before
building. This keeps archived source_head tied to the reviewable exact
Draft PR branch revision rather than an ephemeral merge ref.

This identity check is a CI provenance requirement, not a claim that
the deployed 0001 package uses a newly built 0002 binary.

## Additional streamed-SPK verifier review (source-only hardening)

The SPK verifier now enforces per-member count, size and cumulative declared
uncompressed-byte budgets **on each tar header before requesting the next**,
rather than scanning all tar headers first and applying the cumulative budget
only during extraction. It rejects noncanonical path spellings (such as
repeated separators or embedded dot segments) as well as traversal. A
regression test instruments header requests and asserts that an aggregate
budget violation stops scanning at that member, without advancing to the
next. This change affects repository verification/tests, not the installed
NAS package, runtime binary, permissions, or on-device service.

> HOSTILE REVIEWER: tarfile still parses PAX/metadata and may internally
> decompress input while looking for the next header. Declared-size checks
> do not establish a mathematically strict bound on all parsing costs.

ACCEPTED. The 48 MiB compressed-SPK limit, maximum 64 members, 32 MiB
single-member size, 64 MiB total declared payload budget and streaming
admission reduce common archive-bomb exposure; they do not qualify
arbitrary hostile tar processing as memory-safe on the DS216. Verification
runs on a separate builder, **never inside the 512 MiB NAS service**.
An external archive must not be accepted solely on its own embedded
manifest or a self-asserted SHA.

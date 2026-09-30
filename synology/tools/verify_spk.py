#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import io
import json
import struct
import tarfile
from pathlib import Path, PurePosixPath




def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def read_archive(archive: tarfile.TarFile) -> tuple[dict[str, bytes], dict[str, int]]:
    members = archive.getmembers()
    names = [member.name for member in members]
    if names != sorted(names):
        raise ValueError("archive entries are not deterministically sorted")
    if len(names) != len(set(names)):
        raise ValueError("archive contains duplicate members")

    payloads: dict[str, bytes] = {}
    modes: dict[str, int] = {}
    for member in members:
        path = PurePosixPath(member.name)
        if path.is_absolute() or ".." in path.parts:
            raise ValueError(f"unsafe archive path: {member.name}")
        if not member.isfile():
            raise ValueError(f"non-regular archive member: {member.name}")
        if member.uid != 0 or member.gid != 0 or member.mtime != 0 or member.uname or member.gname:
            raise ValueError(f"nondeterministic archive metadata: {member.name}")
        fileobj = archive.extractfile(member)
        if fileobj is None:
            raise ValueError(f"missing archive payload: {member.name}")
        payloads[member.name] = fileobj.read()
        modes[member.name] = member.mode
    return payloads, modes


def require_armv7_elf(data: bytes, name: str) -> None:
    if len(data) < 20 or data[:4] != b"\x7fELF":
        raise ValueError(f"{name} is not ELF")
    if data[4] != 1:
        raise ValueError(f"{name} is not 32-bit ELF")
    if data[5] != 1:
        raise ValueError(f"{name} is not little-endian ELF")
    if struct.unpack("<H", data[18:20])[0] != 40:
        raise ValueError(f"{name} is not ARM ELF")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("spk")
    args = parser.parse_args()
    spk_path = Path(args.spk)

    with tarfile.open(spk_path, "r:") as outer_archive:
        outer, outer_modes = read_archive(outer_archive)

    required_outer = {
        "INFO",
        "LICENSE",
        "PACKAGE_ICON.PNG",
        "PACKAGE_ICON_256.PNG",
        "conf/privilege",
        "conf/resource",
        "conf/systemd/pkguser-workbridgerelay.service",
        "WIZARD_UIFILES/install_uifile",
        "scripts/preinst",
        "scripts/postinst",
        "scripts/preuninst",
        "scripts/postuninst",
        "scripts/preupgrade",
        "scripts/postupgrade",
        "scripts/start-stop-status",
        "package.tgz",
    }
    if set(outer) != required_outer:
        raise ValueError(f"SPK outer member mismatch missing={sorted(required_outer-set(outer))} unexpected={sorted(set(outer)-required_outer)}")
    for script in (
        "scripts/preinst",
        "scripts/postinst",
        "scripts/preuninst",
        "scripts/postuninst",
        "scripts/preupgrade",
        "scripts/postupgrade",
        "scripts/start-stop-status",
    ):
        if outer_modes[script] & 0o111 == 0:
            raise ValueError(f"lifecycle script is not executable: {script}")

    for name, size in (("PACKAGE_ICON.PNG", 64), ("PACKAGE_ICON_256.PNG", 256)):
        image=outer[name]
        if len(image) < 24 or image[:8] != b"\x89PNG\r\n\x1a\n" or struct.unpack(">II",image[16:24]) != (size,size):
            raise ValueError(f"DSM7 package icon {name} must be {size}x{size} PNG")
    if b"SOURCE-AVAILABLE PROPRIETARY LICENSE" not in outer["LICENSE"]:
        raise ValueError("SPK must carry the source license")
    info = outer["INFO"].decode("utf-8")
    for required in ('package="WorkBridgeRelay"', 'version="0.1.0-0001"', 'arch="armada38x"'):
        if required not in info:
            raise ValueError(f"INFO contract missing {required}")

    resource = json.loads(outer["conf/resource"])
    if resource != {"systemd-user-unit": {}}:
        raise ValueError("DSM resource contract must not grant NAS share access")

    privilege = json.loads(outer["conf/privilege"])
    if privilege != {
        "defaults": {"run-as": "package"},
        "username": "WorkBridgeRelay",
        "groupname": "WorkBridgeRelay",
    }:
        raise ValueError("package privilege contract mismatch")
    unit = outer["conf/systemd/pkguser-workbridgerelay.service"].decode("utf-8")
    for required in ("Environment=GOMEMLIMIT=96MiB", "Environment=GOGC=75", "Environment=GOMAXPROCS=1"):
        if required not in unit:
            raise ValueError(f"DS216 service Go runtime budget missing {required}")

    with tarfile.open(fileobj=io.BytesIO(outer["package.tgz"]), mode="r:gz") as inner_archive:
        inner, inner_modes = read_archive(inner_archive)

    required_inner = {
        "bin/workbridge-mcp",
        "bin/tunnel-client-runtime",
        "bin/run-workbridge-relay.sh",
        "bin/diagnose-workbridge-relay.sh",
        "etc/workbridge-relay.json",
        "provenance/component-bindings.json",
        "third_party/openai-tunnel-client-LICENSE",
        "third_party/openai-tunnel-client-NOTICE",
    }
    if set(inner) != required_inner:
        raise ValueError(f"SPK inner member mismatch missing={sorted(required_inner-set(inner))} unexpected={sorted(set(inner)-required_inner)}")

    for name in ("bin/workbridge-mcp", "bin/tunnel-client-runtime", "bin/run-workbridge-relay.sh", "bin/diagnose-workbridge-relay.sh"):
        if inner_modes[name] & 0o111 == 0:
            raise ValueError(f"required executable mode missing: {name}")

    require_armv7_elf(inner["bin/workbridge-mcp"], "workbridge-mcp")
    require_armv7_elf(inner["bin/tunnel-client-runtime"], "tunnel-client-runtime")

    provenance = json.loads(inner["provenance/component-bindings.json"])
    if provenance.get("schema") != "WORKBRIDGE_RELAY_RUNTIME_PROVENANCE_V1":
        raise ValueError("provenance schema mismatch")
    bindings = provenance["component_bindings"]
    if bindings["workbridge_mcp"]["commit"] != "b650b50abcbe1f81c653e1c2d305849b5d494489":
        raise ValueError("WorkBridge source binding mismatch")
    if bindings["openai_tunnel_client"]["commit"] != "a390c168ff1b2d14e73a95991c186c6aba3ff5a0":
        raise ValueError("tunnel-client source binding mismatch")

    wb_hash = sha256(inner["bin/workbridge-mcp"])
    tunnel_hash = sha256(inner["bin/tunnel-client-runtime"])
    if wb_hash != bindings["workbridge_mcp"]["qualified_binary_sha256"]:
        raise ValueError("WorkBridge binary hash mismatch")
    if tunnel_hash != bindings["openai_tunnel_client"]["qualified_binary_sha256"]:
        raise ValueError("tunnel-client binary hash mismatch")
    if provenance["artifacts"]["workbridge_mcp_sha256"] != wb_hash:
        raise ValueError("WorkBridge provenance hash mismatch")
    if provenance["artifacts"]["tunnel_client_runtime_sha256"] != tunnel_hash:
        raise ValueError("tunnel-client provenance hash mismatch")

    config = json.loads(inner["etc/workbridge-relay.json"])
    if config.get("read_roots") != [] or config.get("write_roots") != []:
        raise ValueError("default WorkBridge roots must be empty")
    if config.get("process", {}).get("enabled") is not False or config.get("process", {}).get("allowed_executables") != []:
        raise ValueError("default process execution must remain disabled")
    if config.get("http", {}).get("listen") != "127.0.0.1:8765":
        raise ValueError("default backend must bind loopback only")
    limits = config.get("limits", {})
    if limits.get("max_read_bytes", 2**32) > 524288 or limits.get("max_write_bytes", 2**32) > 524288:
        raise ValueError("DS216: default text transfer exceeds 512 KiB")
    if limits.get("max_directory_entries", 2**32) > 256:
        raise ValueError("DS216: default directory listing budget too large")
    proc = config.get("process", {})
    if proc.get("max_concurrent") != 1 or proc.get("max_output_bytes", 2**32) > 262144:
        raise ValueError("DS216: default process concurrency/output budget mismatch")


    launcher = inner["bin/run-workbridge-relay.sh"].decode("utf-8")
    if "--mcp.command" not in launcher or "--mcp.server-url" in launcher:
        raise ValueError("launcher must use stdio WorkBridge transport")
    if "--health.listen-addr \"127.0.0.1:0\"" not in launcher:
        raise ValueError("health listener must bind ephemeral loopback")
    if "--health.url-file \"$HEALTH_URL_FILE\"" not in launcher:
        raise ValueError("health URL must go to package-private file")
    if "file:$API_KEY_FILE" not in launcher:
        raise ValueError("runtime secret must be file-referenced")
    for required in ('--control-plane.max-inflight "4"', '--mcp.max-concurrent-requests "1"'):
        if required not in launcher:
            raise ValueError(f"DS216: missing low-resource tunnel bound {required}")

    if "shares/" in launcher or "Media" in launcher:
        raise ValueError("relay launcher must not select an arbitrary NAS share")

    doctor = inner["bin/diagnose-workbridge-relay.sh"].decode("utf-8")
    if "readyz" not in doctor or "healthz" not in doctor or "mcp=not_independently_verified" not in doctor:
        raise ValueError("operator doctor lacks readiness/claim distinctions")
    if not inner["third_party/openai-tunnel-client-LICENSE"].strip():
        raise ValueError("OpenAI tunnel-client LICENSE missing")
    if b"Copyright 2026 OpenAI" not in inner["third_party/openai-tunnel-client-NOTICE"]:
        raise ValueError("OpenAI tunnel-client NOTICE mismatch")

    print(json.dumps({
        "schema": "WORKBRIDGE_RELAY_SPK_VERIFY_V1",
        "status": "PASS",
        "sha256": sha256(spk_path.read_bytes()),
        "bytes": spk_path.stat().st_size,
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

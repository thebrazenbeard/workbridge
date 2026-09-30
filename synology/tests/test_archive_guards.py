import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

ROOT=Path(__file__).resolve().parents[2]
VERIFY=ROOT/"synology/tools/verify_spk.py"

def verifier():
    spec=importlib.util.spec_from_file_location("workbridge_spk_verifier",VERIFY)
    module=importlib.util.module_from_spec(spec)
    assert spec and spec.loader
    spec.loader.exec_module(module)
    return module

class RelayArchiveGuardTests(unittest.TestCase):
    @staticmethod
    def source_head():
        return subprocess.run(["git","rev-parse","HEAD"],cwd=ROOT,
                              check=True,capture_output=True,text=True).stdout.strip()

    def test_builder_rejects_forged_source_head(self):
        with tempfile.TemporaryDirectory() as d:
            fake=Path(d)/"untrusted-armv7"
            fake.write_bytes(b"not-an-authorized-binary")
            p=subprocess.run([sys.executable,str(ROOT/"synology/tools/build_spk.py"),
               "--workbridge-bin",str(fake),"--tunnel-bin",str(fake),
               "--source-head","0"*40],cwd=ROOT,capture_output=True,text=True)
            self.assertNotEqual(p.returncode,0)
            self.assertIn("differs from checked-out Git HEAD",p.stdout+p.stderr)

    def test_builder_rejects_malformed_source_head(self):
        with tempfile.TemporaryDirectory() as d:
            fake=Path(d)/"untrusted-armv7"
            fake.write_bytes(b"not-an-authorized-binary")
            p=subprocess.run([sys.executable,str(ROOT/"synology/tools/build_spk.py"),
               "--workbridge-bin",str(fake),"--tunnel-bin",str(fake),
               "--source-head","fake"],cwd=ROOT,capture_output=True,text=True)
            self.assertNotEqual(p.returncode,0)
            self.assertIn("full 40-character",p.stdout+p.stderr)

    def test_builder_rejects_dirty_tracked_checkout(self):
        spec=importlib.util.spec_from_file_location(
            "workbridge_spk_builder",ROOT/"synology/tools/build_spk.py")
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module=importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with patch.object(module.subprocess,"run",
                          return_value=SimpleNamespace(stdout=" M README.md\n")):
            with self.assertRaisesRegex(SystemExit,"tracked source is modified"):
                module.validate_clean_checkout()

    def test_verifier_rejects_forged_provenance_source_head(self):
        check=verifier().validate_provenance_head
        head=self.source_head()
        check(head,head)
        with self.assertRaisesRegex(ValueError,"missing or invalid"):
            check("short",None)
        with self.assertRaisesRegex(ValueError,"differs from expected"):
            check(head,"0"*40)

    def test_builder_rejects_unpinned_runtime(self):
        with tempfile.TemporaryDirectory() as d:
            fake=Path(d)/"untrusted-armv7"
            fake.write_bytes(b"not-an-authorized-binary")
            p=subprocess.run([sys.executable,str(ROOT/"synology/tools/build_spk.py"),
               "--workbridge-bin",str(fake),"--tunnel-bin",str(fake),
               "--source-head",self.source_head()],cwd=ROOT,
               capture_output=True,text=True)
            self.assertNotEqual(p.returncode,0)
            self.assertIn("digest does not match qualified binding",p.stdout+p.stderr)

    def test_verifier_rejects_forged_self_certifying_mcp_hash(self):
        embedded=json.loads((ROOT/"synology/component-bindings.json").read_text())
        verifier().require_trusted_bindings(embedded)
        embedded["workbridge_mcp"]["qualified_binary_sha256"]="0"*64
        with self.assertRaisesRegex(ValueError,"trusted checked-out component"):
            verifier().require_trusted_bindings(embedded)

    def test_verifier_rejects_forged_self_certifying_tunnel_hash(self):
        embedded=json.loads((ROOT/"synology/component-bindings.json").read_text())
        embedded["openai_tunnel_client"]["qualified_binary_sha256"]="0"*64
        with self.assertRaisesRegex(ValueError,"trusted checked-out component"):
            verifier().require_trusted_bindings(embedded)

    def test_verifier_rejects_traversal_archive(self):
        buf=io.BytesIO()
        with tarfile.open(fileobj=buf,mode="w") as t:
            info=tarfile.TarInfo("../escaped")
            info.size=1
            t.addfile(info,io.BytesIO(b"x"))
        buf.seek(0)
        with tarfile.open(fileobj=buf,mode="r:") as t:
            with self.assertRaisesRegex(ValueError,"unsafe archive path"):
                verifier().read_archive(t)

    def test_verifier_rejects_symlink_archive(self):
        buf=io.BytesIO()
        with tarfile.open(fileobj=buf,mode="w") as t:
            info=tarfile.TarInfo("link")
            info.type=tarfile.SYMTYPE
            info.linkname="target"
            t.addfile(info)
        buf.seek(0)
        with tarfile.open(fileobj=buf,mode="r:") as t:
            with self.assertRaisesRegex(ValueError,"non-regular"):
                verifier().read_archive(t)

    def test_verifier_rejects_tar_bomb_declared_member_size(self):
        tarinfo=tarfile.TarInfo("oversized")
        tarinfo.size=verifier().MAX_ARCHIVE_MEMBER_BYTES+1
        # One valid tar header with a deliberately impossible large length:
        # the verifier must reject BEFORE attempting to read the body.
        raw=tarinfo.tobuf()+b"\0"*1024
        with tarfile.open(fileobj=io.BytesIO(raw),mode="r:") as archive:
            with self.assertRaisesRegex(ValueError,"exceeds byte limit"):
                verifier().read_archive(archive)

    def test_verifier_rejects_tar_member_count_bomb(self):
        output=io.BytesIO()
        with tarfile.open(fileobj=output,mode="w") as archive:
            for i in range(verifier().MAX_ARCHIVE_MEMBERS+1):
                info=tarfile.TarInfo(f"regular-{i:03d}")
                info.size=0
                archive.addfile(info)
        output.seek(0)
        with tarfile.open(fileobj=output,mode="r:") as archive:
            with self.assertRaisesRegex(ValueError,"member count limit"):
                verifier().read_archive(archive)

    def test_verifier_rejects_non_arm_binary(self):
        with self.assertRaisesRegex(ValueError,"not ELF"):
            verifier().require_armv7_elf(b"not-elf","candidate")

if __name__=="__main__":
    unittest.main()

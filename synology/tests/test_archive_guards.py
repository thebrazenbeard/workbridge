import importlib.util
import io
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[2]
VERIFY=ROOT/"synology/tools/verify_spk.py"

def verifier():
    spec=importlib.util.spec_from_file_location("workbridge_spk_verifier",VERIFY)
    module=importlib.util.module_from_spec(spec)
    assert spec and spec.loader
    spec.loader.exec_module(module)
    return module

class RelayArchiveGuardTests(unittest.TestCase):
    def test_builder_rejects_unpinned_runtime(self):
        with tempfile.TemporaryDirectory() as d:
            fake=Path(d)/"untrusted-armv7"
            fake.write_bytes(b"not-an-authorized-binary")
            p=subprocess.run([sys.executable,str(ROOT/"synology/tools/build_spk.py"),
               "--workbridge-bin",str(fake),"--tunnel-bin",str(fake),
               "--source-head","test-fixture"],cwd=ROOT,
               capture_output=True,text=True)
            self.assertNotEqual(p.returncode,0)
            self.assertIn("digest does not match qualified binding",p.stdout+p.stderr)

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

    def test_verifier_rejects_non_arm_binary(self):
        with self.assertRaisesRegex(ValueError,"not ELF"):
            verifier().require_armv7_elf(b"not-elf","candidate")

if __name__=="__main__":
    unittest.main()

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "synology/tools/nas_preflight.sh"

@unittest.skipIf(os.name=="nt","DS216 POSIX preflight fixtures run on Ubuntu CI")
class DS216PreflightTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name)
        self.model=self.root/"model"
        self.model.write_text("DS216\n")
        self.version=self.root/"VERSION"
        self.version.write_text('majorversion="7"\nminorversion="2"\nproductversion="7.2.2"\nbuildnumber="72806"\n')
        self.memory=self.root/"meminfo"
        self.memory.write_text("MemTotal: 493200 kB\nMemAvailable: 250000 kB\n")
        self.package=self.root/"absent-package"
        self.volume=self.root/"volume1"
        self.volume.mkdir()
        self.fakebin=self.root/"bin"
        self.fakebin.mkdir()
        self.uname=self.fakebin/"uname"
        self.uname.write_text("#!/bin/sh\nprintf 'armv7l\\n'\n")
        self.uname.chmod(0o755)
        self.df=self.fakebin/"df"
        self.df.write_text("#!/bin/sh\nprintf 'Filesystem 1024-blocks Used Available Capacity Mounted on\\n/tmp 10240000 1024 2048000 1%% /volume1\\n'\n")
        self.df.chmod(0o755)

    def run_gate(self):
        env=dict(os.environ,
            WORKBRIDGE_PREFLIGHT_MODEL_FILE=str(self.model),
            WORKBRIDGE_PREFLIGHT_VERSION_FILE=str(self.version),
            WORKBRIDGE_PREFLIGHT_MEMINFO_FILE=str(self.memory),
            WORKBRIDGE_PREFLIGHT_PACKAGE_DIR=str(self.package),
            WORKBRIDGE_PREFLIGHT_VOLUME=str(self.volume),
            PATH=str(self.fakebin)+os.pathsep+os.environ.get("PATH",""))
        p=subprocess.run(["sh",str(SCRIPT)],capture_output=True,text=True,
                         timeout=6,env=env)
        self.assertNotIn("api-key",p.stdout+p.stderr)
        return p

    def test_verified_ds216_is_prepared_not_installed(self):
        result=self.run_gate()
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertIn("preflight=PREPARED_ONLY",result.stdout)
        self.assertIn("installation_state=NOT_VERIFIED",result.stdout)
        self.assertEqual(sorted(p.name for p in self.volume.iterdir()),[])

    def test_other_model_rejected(self):
        self.model.write_text("DS216play\n")
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("hardware_gate=not_original_ds216",p.stdout)

    def test_architecture_rejected(self):
        self.uname.write_text("#!/bin/sh\nprintf 'x86_64\\n'\n")
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("arch_gate=not_armv7",p.stdout)

    def test_old_dsm_rejected(self):
        self.version.write_text('majorversion="7"\nminorversion="1"\nbuildnumber="72805"\n')
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("dsm_gate=below_package_minimum",p.stdout)

    def test_low_available_memory_rejected(self):
        self.memory.write_text("MemTotal: 493200 kB\nMemAvailable: 16000 kB\n")
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("memory_headroom_gate=low_available_memory",p.stdout)

    def test_memavailable_missing_rejected(self):
        self.memory.write_text("MemTotal: 493200 kB\nMemFree: 100000 kB\n")
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("memory_headroom_gate=unknown_recheck_in_dsm_resource_monitor",p.stdout)

    def test_installed_package_requires_separate_upgrade_review(self):
        self.package.mkdir()
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("package_state=already_present_no_implicit_upgrade",p.stdout)

    def test_low_disk_space_rejected(self):
        self.df.write_text("#!/bin/sh\nprintf 'Filesystem 1024-blocks Used Available Capacity Mounted on\\n/tmp 102400 10239 100000 50%% /volume1\\n'\n")
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("volume_gate=insufficient_verified_free_space",p.stdout)

    def test_ram_model_mismatch_rejected(self):
        self.memory.write_text("MemTotal: 1024000 kB\nMemAvailable: 800000 kB\n")
        p=self.run_gate()
        self.assertEqual(p.returncode,2)
        self.assertIn("memory_gate=unexpected_for_512mb_ds216",p.stdout)

if __name__=="__main__":
    unittest.main()

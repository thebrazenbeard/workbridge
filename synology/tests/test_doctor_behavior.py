import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
DOCTOR = ROOT / "synology/payload/bin/diagnose-workbridge-relay.sh"

@unittest.skipIf(os.name == "nt", "POSIX package-user shell diagnostic is exercised on Linux CI")
class DoctorBehaviorTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / "target/bin").mkdir(parents=True)
        (self.root / "var").mkdir()
        for binary in ("workbridge-mcp","tunnel-client-runtime"):
            x = self.root / "target/bin" / binary
            x.write_text("#!/bin/sh\nexit 0\n")
            x.chmod(0o700)
        (self.root / "var/workbridge-relay.json").write_text("{}\n")
        (self.root / "var/tunnel-id").write_text("not-a-real-credential\n")
        (self.root / "var/runtime-api-key").write_text("secret-should-never-print\n")
        (self.root / "var/health-url").write_text("http://127.0.0.1:24816\n")
        (self.root / "fakebin").mkdir()

    def run_doctor(self, curl_script):
        curl = self.root / "fakebin/curl"
        curl.write_text("#!/bin/sh\n" + curl_script + "\n")
        curl.chmod(0o700)
        env = dict(os.environ, WORKBRIDGE_RELAY_ROOT=str(self.root),
                   SYNOPKG_PKGVAR=str(self.root / "var"),
                   PATH=str(self.root / "fakebin")+os.pathsep+os.environ["PATH"])
        p = subprocess.run(["sh", str(DOCTOR)], capture_output=True,
                           text=True, env=env, timeout=5)
        self.assertNotIn("secret-should-never-print",p.stdout + p.stderr)
        return p

    def test_ready_is_not_mcp_attestation(self):
        p = self.run_doctor("exit 0")
        self.assertEqual(0,p.returncode,p.stdout+p.stderr)
        self.assertIn("tunnel=ready",p.stdout)
        self.assertIn("mcp=not_independently_verified",p.stdout)

    def test_pending_provider_readiness(self):
        p = self.run_doctor('case "$*" in *readyz*) exit 22 ;; *) exit 0 ;; esac')
        self.assertEqual(2,p.returncode)
        self.assertIn("tunnel=healthy",p.stdout)
        self.assertIn("tunnel=not_ready",p.stdout)

    def test_health_failure(self):
        p = self.run_doctor("exit 22")
        self.assertEqual(2,p.returncode)
        self.assertIn("tunnel=unhealthy",p.stdout)

    def test_missing_runtime_credentials(self):
        (self.root/"var/runtime-api-key").unlink()
        p = self.run_doctor("exit 0")
        self.assertEqual(2,p.returncode)
        self.assertIn("credentials=missing_or_unreadable",p.stdout)

    def test_external_url_rejected_before_request(self):
        (self.root/"var/health-url").write_text("http://example.net:9999\n")
        p = self.run_doctor("exit 0")
        self.assertEqual(2,p.returncode)
        self.assertIn("tunnel=invalid_local_health_url",p.stdout)

    def test_url_out_of_port_range_rejected(self):
        (self.root/"var/health-url").write_text("http://127.0.0.1:65536\n")
        p = self.run_doctor("exit 0")
        self.assertEqual(2,p.returncode)
        self.assertIn("tunnel=invalid_local_health_port",p.stdout)

if __name__ == "__main__":
    unittest.main()

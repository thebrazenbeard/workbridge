import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "synology" / "spk" / "scripts"
TUNNEL_ID = "tunnel_" + "a"*32
RUNTIME_KEY = "test-runtime-key-not-a-real-secret"

@unittest.skipIf(os.name == "nt", "Synology installer shell fixtures run in Ubuntu CI")
class InstallerLifecycleTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.var = self.root / "var"
        self.target = self.root / "target"
        (self.target / "etc").mkdir(parents=True)
        (self.target / "etc" / "workbridge-relay.json").write_text(
            '{"read_roots":[],"write_roots":[]}\n', encoding="utf-8"
        )

    def run_script(self, name="postinst", wizard=True, override_env=None):
        env = dict(os.environ)
        env["SYNOPKG_PKGVAR"] = str(self.var)
        env["SYNOPKG_PKGDEST"] = str(self.target)
        for key in ("SYNOPKG_WZF_wizard_tunnel_id",
                    "SYNOPKG_WZF_wizard_runtime_api_key",
                    "wizard_tunnel_id", "wizard_runtime_api_key"):
            env.pop(key, None)
        if wizard:
            env["SYNOPKG_WZF_wizard_tunnel_id"] = TUNNEL_ID
            env["SYNOPKG_WZF_wizard_runtime_api_key"] = RUNTIME_KEY
        env.update(override_env or {})
        completed = subprocess.run(
            ["sh", str(SCRIPTS / name)], capture_output=True, text=True,
            env=env, timeout=8
        )
        self.assertNotIn(RUNTIME_KEY, completed.stdout + completed.stderr)
        return completed

    def test_fresh_install_private_state_and_no_residue(self):
        result = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.var/"tunnel-id").read_text().strip(), TUNNEL_ID)
        self.assertEqual((self.var/"runtime-api-key").read_text().strip(), RUNTIME_KEY)
        self.assertEqual((self.var/"workbridge-relay.json").read_text(),
                         (self.target/"etc/workbridge-relay.json").read_text())
        self.assertEqual(stat.S_IMODE(self.var.stat().st_mode), 0o700)
        for name in ("tunnel-id", "runtime-api-key", "workbridge-relay.json"):
            self.assertEqual(stat.S_IMODE((self.var/name).stat().st_mode), 0o600, name)
        self.assertEqual(sorted(x.name for x in self.var.iterdir()),
                         ["runtime-api-key","tunnel-id","workbridge-relay.json"])

    def test_reinstall_reuses_credentials_and_preserves_operator_config(self):
        self.assertEqual(self.run_script().returncode, 0)
        custom = '{"read_roots":["/volume1/explicit-only"],"write_roots":[]}\n'
        (self.var/"workbridge-relay.json").write_text(custom)
        result = self.run_script(wizard=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.var/"workbridge-relay.json").read_text(), custom)
        self.assertEqual((self.var/"tunnel-id").read_text().strip(), TUNNEL_ID)

    def test_upgrade_preserves_credentials_and_operator_config(self):
        self.assertEqual(self.run_script().returncode, 0)
        custom = '{"schema":"operator-owned","read_roots":[]}\n'
        (self.var/"workbridge-relay.json").write_text(custom)
        before={n:(self.var/n).read_bytes() for n in ("runtime-api-key","tunnel-id")}
        result=self.run_script(name="postupgrade", wizard=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.var/"workbridge-relay.json").read_text(),custom)
        after={n:(self.var/n).read_bytes() for n in ("runtime-api-key","tunnel-id")}
        self.assertEqual(before, after)

    def test_upgrade_initializes_missing_config(self):
        result=self.run_script(name="postupgrade",wizard=False)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual((self.var/"workbridge-relay.json").read_text(),
                         (self.target/"etc/workbridge-relay.json").read_text())
        self.assertEqual(stat.S_IMODE((self.var/"workbridge-relay.json").stat().st_mode),0o600)

    def test_dsm7_var_symlink_to_private_appdata_is_accepted(self):
        volume=self.root/"volume1"
        private=volume/"@appdata"/"WorkBridgeRelay"
        private.mkdir(parents=True)
        self.var.symlink_to(private,target_is_directory=True)
        env={"SYNOPKG_PKGDEST_VOL":str(volume)}
        result=self.run_script(override_env=env)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual((private/"tunnel-id").read_text().strip(),TUNNEL_ID)
        self.assertEqual(stat.S_IMODE(private.stat().st_mode),0o700)
        custom='{"schema":"operator"}\n'
        (private/"workbridge-relay.json").write_text(custom)
        result=self.run_script(name="postupgrade",wizard=False,override_env=env)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual((private/"workbridge-relay.json").read_text(),custom)

    def test_dsm7_var_symlink_to_other_appdata_is_rejected(self):
        volume=self.root/"volume1"
        outside=volume/"@appdata"/"SomeOtherPackage"
        outside.mkdir(parents=True)
        self.var.symlink_to(outside,target_is_directory=True)
        result=self.run_script(override_env={"SYNOPKG_PKGDEST_VOL":str(volume)})
        self.assertNotEqual(result.returncode,0)
        self.assertIn("outside DSM package appdata",result.stderr)
        self.assertEqual(list(outside.iterdir()),[])

    def test_rejects_state_directory_symlink(self):
        outside=self.root/"outside"
        outside.mkdir()
        self.var.symlink_to(outside, target_is_directory=True)
        result=self.run_script()
        self.assertNotEqual(result.returncode,0)
        self.assertIn("symlink",result.stderr)
        self.assertEqual(list(outside.iterdir()),[])

    def test_rejects_credentials_symlink_without_touching_target(self):
        self.var.mkdir()
        outside=self.root/"outside-key"
        outside.write_text("do-not-change\n")
        (self.var/"runtime-api-key").symlink_to(outside)
        result=self.run_script()
        self.assertNotEqual(result.returncode,0)
        self.assertIn("symlink",result.stderr)
        self.assertEqual(outside.read_text(),"do-not-change\n")

    def test_rejects_config_symlink_on_upgrade(self):
        self.var.mkdir()
        outside=self.root/"outside-config"
        outside.write_text("do-not-change\n")
        (self.var/"workbridge-relay.json").symlink_to(outside)
        result=self.run_script(name="postupgrade",wizard=False)
        self.assertNotEqual(result.returncode,0)
        self.assertIn("symlink",result.stderr)
        self.assertEqual(outside.read_text(),"do-not-change\n")

    def test_rejects_symlinked_packaged_default(self):
        outside=self.root/"outside-config"
        outside.write_text("unsafe\n")
        (self.target/"etc/workbridge-relay.json").unlink()
        (self.target/"etc/workbridge-relay.json").symlink_to(outside)
        result=self.run_script()
        self.assertNotEqual(result.returncode,0)
        self.assertIn("unsafe",result.stderr)
        self.assertFalse((self.var/"runtime-api-key").exists())

    def test_staging_failure_preserves_existing_credentials(self):
        result=self.run_script()
        self.assertEqual(result.returncode,0,result.stderr)
        before={n:(self.var/n).read_bytes() for n in ("tunnel-id","runtime-api-key")}
        fakebin=self.root/"fakebin"
        fakebin.mkdir()
        fake=fakebin/"mktemp"
        fake.write_text("#!/bin/sh\nexit 11\n")
        fake.chmod(0o700)
        result=self.run_script(override_env={
            "PATH":str(fakebin)+os.pathsep+os.environ.get("PATH",""),
            "SYNOPKG_WZF_wizard_runtime_api_key":"different-valid-key"
        })
        self.assertNotEqual(result.returncode,0)
        self.assertIn("cannot stage",result.stderr)
        self.assertEqual(before,{n:(self.var/n).read_bytes() for n in before})
        self.assertEqual(list(self.var.glob(".*")),[])

    def test_rejects_invalid_tunnel_id_before_writes(self):
        result=self.run_script(override_env={"SYNOPKG_WZF_wizard_tunnel_id":"invalid"})
        self.assertNotEqual(result.returncode,0)
        self.assertIn("invalid Secure MCP Tunnel ID",result.stderr)
        self.assertFalse((self.var/"runtime-api-key").exists())

if __name__ == "__main__":
    unittest.main()

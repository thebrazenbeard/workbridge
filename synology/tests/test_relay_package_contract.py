import json
import os
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
SPK = ROOT / "synology"
PKG = "WorkBridgeRelay"
SCRIPTS = ("preinst", "postinst", "preuninst", "postuninst",
           "preupgrade", "postupgrade", "start-stop-status")

class RelayPackageContractTests(unittest.TestCase):
    def test_independent_dsm_package_identity(self):
        info = (SPK / "spk/INFO").read_text()
        self.assertIn('package="WorkBridgeRelay"', info)
        self.assertIn('version="0.1.0-0001"', info)
        self.assertIn('arch="armada38x"', info)
        self.assertIn('os_min_ver="7.2-72806"', info)
        self.assertNotIn("Media", info)
        self.assertNotIn("Vera", info)
        self.assertEqual(
            {"defaults": {"run-as": "package"}, "username": PKG,
             "groupname": PKG},
            json.loads((SPK / "spk/conf/privilege").read_text())
        )
        self.assertEqual(
            {"systemd-user-unit": {}},
            json.loads((SPK / "spk/conf/resource").read_text())
        )

    def test_no_implicit_share_or_process_authority(self):
        cfg=json.loads((SPK / "payload/etc/workbridge-relay.json").read_text())
        self.assertEqual(cfg["schema"], "WORKBRIDGE_CONFIG_V1")
        self.assertEqual(cfg["read_roots"], [])
        self.assertEqual(cfg["write_roots"], [])
        self.assertEqual(cfg["process"]["allowed_executables"], [])
        self.assertEqual(cfg["process"]["working_roots"], [])
        self.assertIs(cfg["process"]["enabled"], False)
        self.assertEqual(cfg["http"]["listen"], "127.0.0.1:8765")
        for file in (SPK / "spk").rglob("*"):
            if file.is_file() and file.suffix.lower() != ".png":
                txt=file.read_text(encoding="utf-8")
                self.assertNotIn("Media", txt, str(file))
                self.assertNotIn("Vera", txt, str(file))
                self.assertNotIn("data-share", txt, str(file))
        for path in (SPK / "payload").rglob("*"):
            if path.is_file():
                text=path.read_text(encoding="utf-8")
                self.assertNotIn("Media", text, str(path))
                self.assertNotIn("Vera", text, str(path))

    def test_std_io_outbound_tunnel_launcher(self):
        launch=(SPK / "payload/bin/run-workbridge-relay.sh").read_text()
        self.assertIn('TUNNEL_ID_FILE="$VAR/tunnel-id"',launch)
        self.assertIn('API_KEY_FILE="$VAR/runtime-api-key"',launch)
        self.assertIn('--mcp.command "$MCP_COMMAND"', launch)
        self.assertIn('--control-plane.api-key "file:$API_KEY_FILE"',launch)
        self.assertIn('--health.listen-addr "127.0.0.1:0"',launch)
        self.assertIn('--health.url-file "$HEALTH_URL_FILE"',launch)
        self.assertIn(': > "$HEALTH_URL_FILE"',launch)
        self.assertNotIn("--mcp.server-url",launch)
        self.assertNotIn("ssh",launch.lower())
        self.assertNotIn("shares/",launch)
        self.assertNotIn("http://0.0.0.0",launch)
        doctor=(SPK/"payload/bin/diagnose-workbridge-relay.sh").read_text()
        self.assertIn("readyz",doctor)
        self.assertIn("mcp=not_independently_verified",doctor)

    def test_package_service_and_scripts(self):
        unit=(SPK/"spk/conf/systemd/pkguser-workbridgerelay.service").read_text()
        self.assertIn("ExecStart=/var/packages/WorkBridgeRelay/target/bin/run-workbridge-relay.sh",unit)
        self.assertIn("UMask=0077",unit)
        self.assertIn("KillMode=control-group",unit)
        self.assertIn("RestartSec=15",unit)
        self.assertIn("StartLimitBurst=5",unit)
        for name in SCRIPTS:
            path=SPK/"spk/scripts"/name
            self.assertTrue(path.exists())
            if os.name!="nt":
                self.assertTrue(path.stat().st_mode & 0o111,name)
        ctl=(SPK/"spk/scripts/start-stop-status").read_text()
        self.assertIn("pkguser-workbridgerelay.service",ctl)
        self.assertNotIn("shares/",ctl)

    def test_install_secret_handling_and_upgrade(self):
        wizard=json.loads((SPK/"spk/WIZARD_UIFILES/install_uifile").read_text())
        items=[sub for group in wizard for item in group["items"]
               for sub in item["subitems"]]
        self.assertEqual({"wizard_tunnel_id","wizard_runtime_api_key"},
                         {item["key"] for item in items})
        postinst=(SPK/"spk/scripts/postinst").read_text()
        postupgrade=(SPK/"spk/scripts/postupgrade").read_text()
        for x in (postinst,postupgrade):
            self.assertIn('if [ ! -e "$CONFIG_FILE" ]; then',x)
            self.assertIn('chmod 600 "$CONFIG_FILE"',x)
        self.assertIn("umask 077",postinst)
        self.assertIn('chmod 600 "$TUNNEL_ID_FILE"',postinst)
        self.assertIn('chmod 600 "$API_KEY_FILE"',postinst)
        self.assertNotIn('echo $API_KEY',postinst)

    def test_pinned_components(self):
        bindings=json.loads((SPK/"component-bindings.json").read_text())
        self.assertEqual(bindings["schema"],"WORKBRIDGE_RELAY_BINDINGS_V1")
        self.assertEqual(bindings["package"]["id"],PKG)
        self.assertEqual(bindings["workbridge_mcp"]["repository"],
                         "thebrazenbeard/workbridge")
        self.assertEqual(bindings["workbridge_mcp"]["qualified_binary_sha256"],
                         "91e2bc04b4970b88fa1ff85c1c9363ba593c1325ce23008b7d7138908d3ddf86")
        self.assertEqual(bindings["openai_tunnel_client"]["qualified_binary_sha256"],
                         "3c27d0e9d7dc44488704a3c1687155b7fb5cf80b1fcd3c3d78fac1494229e671")
        self.assertEqual(bindings["authority"]["filesystem_roots_at_install"],[])
        self.assertFalse(bindings["authority"]["nas_share_permissions_granted"])
        self.assertFalse(bindings["authority"]["process_execution"])
        self.assertTrue((SPK/"third_party/openai-tunnel-client-LICENSE").exists())
        self.assertTrue((SPK/"third_party/openai-tunnel-client-NOTICE").exists())

if __name__=="__main__":
    unittest.main()

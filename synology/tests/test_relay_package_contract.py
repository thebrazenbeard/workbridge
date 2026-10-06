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
        self.assertIn('version="0.1.0-0003"', info)
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

    def test_dsm_gui_route_is_package_owned_and_admin_only(self):
        info = (SPK / "spk/INFO").read_text()
        self.assertIn('dsmuidir="ui"', info)
        self.assertIn('dsmappname="com.workbridge.WorkBridgeRelay"', info)
        ui_path = SPK / "payload/ui/config"
        self.assertTrue(ui_path.is_file(), "DSM desktop app config is missing")
        ui = json.loads(ui_path.read_text())
        app = ui[".url"]["com.workbridge.WorkBridgeRelay"]
        self.assertEqual(app["type"], "url")
        self.assertEqual(app["icon"], "images/workbridge_{0}.png")
        self.assertEqual(app["url"], "3rdparty/WorkBridgeRelay/index.html")
        self.assertIs(app["allUsers"], False)
        self.assertTrue((SPK / "payload/ui/index.html").is_file())
        self.assertNotIn("127.0.0.1", json.dumps(app))
        self.assertNotIn("8765", json.dumps(app))

    def test_no_implicit_share_or_process_authority(self):
        cfg=json.loads((SPK / "payload/etc/workbridge-relay.json").read_text())
        self.assertEqual(cfg["schema"], "WORKBRIDGE_CONFIG_V1")
        self.assertEqual(cfg["read_roots"], [])
        self.assertEqual(cfg["write_roots"], [])
        self.assertEqual(cfg["process"]["allowed_executables"], [])
        self.assertEqual(cfg["process"]["working_roots"], [])
        self.assertIs(cfg["process"]["enabled"], False)
        self.assertEqual(cfg["http"]["listen"], "127.0.0.1:8765")
        self.assertLessEqual(cfg["limits"]["max_read_bytes"], 524288)
        self.assertLessEqual(cfg["limits"]["max_write_bytes"], 524288)
        self.assertLessEqual(cfg["limits"]["max_directory_entries"], 256)
        self.assertLessEqual(cfg["process"]["max_output_bytes"], 262144)
        self.assertEqual(cfg["process"]["max_concurrent"], 1)
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
        self.assertIn('--control-plane.max-inflight "4"', launch)
        self.assertIn('--mcp.max-concurrent-requests "1"', launch)
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

    def test_artifact_workflow_uses_actual_pull_request_head(self):
        workflow=(ROOT/".github/workflows/workbridge-relay-spk.yml").read_text()
        desired="$"+"{{ github.event.pull_request.head.sha || github.sha }}"
        self.assertIn("ref: "+desired,workflow)
        self.assertIn("expected_head=\""+desired+"\"",workflow)
        self.assertIn('test "$(git rev-parse HEAD)" = "$expected_head"',workflow)
        self.assertNotIn('test "$(git rev-parse HEAD)" = "$GITHUB_SHA"',workflow)

    def test_ds216_outage_restart_spacing_does_not_exhaust_start_limit(self):
        unit = (SPK/"spk/conf/systemd/pkguser-workbridgerelay.service").read_text()
        settings={}
        for line in unit.splitlines():
            if "=" in line and not line.lstrip().startswith("#"):
                k,v=line.split("=",1)
                settings[k]=v
        interval=int(settings["StartLimitIntervalSec"])
        burst=int(settings["StartLimitBurst"])
        restart=int(settings["RestartSec"])
        self.assertEqual(settings["Restart"],"on-failure")
        self.assertEqual(settings["KillMode"],"control-group")
        self.assertEqual(settings["UMask"],"0077")
        self.assertGreaterEqual(restart*(burst-1),interval,
            "an offline boot would exhaust start limit and stop recovering")
        self.assertGreaterEqual(restart,60,
            "DS216 retries should avoid excessive CPU and provider pressure")

    def test_package_service_and_scripts(self):
        unit=(SPK/"spk/conf/systemd/pkguser-workbridgerelay.service").read_text()
        self.assertIn("ExecStart=/var/packages/WorkBridgeRelay/target/bin/run-workbridge-relay.sh",unit)
        self.assertIn("UMask=0077",unit)
        self.assertIn("Environment=GOMEMLIMIT=96MiB",unit)
        self.assertIn("Environment=GOGC=75",unit)
        self.assertIn("Environment=GOMAXPROCS=1",unit)
        self.assertIn("KillMode=control-group",unit)
        self.assertIn("RestartSec=90",unit)
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
        self.assertIn('mktemp "$VAR/.tunnel-id.XXXXXX"', postinst)
        self.assertIn('mktemp "$VAR/.runtime-api-key.XXXXXX"', postinst)
        self.assertIn('trap cleanup 0', postinst)
        self.assertIn('[ ! -L "$path" ]', postinst)
        self.assertIn('readlink -f "$VAR"',postinst)
        self.assertIn('readlink -f "$VAR"',postupgrade)
        self.assertIn('$SYNOPKG_PKGDEST_VOL/@appdata/WorkBridgeRelay',postinst)
        self.assertIn('[ ! -L "$path" ]', postupgrade)
        self.assertIn('trap cleanup 0', postupgrade)
        self.assertNotIn('"$TUNNEL_ID_FILE.new"', postinst)
        self.assertNotIn('"$API_KEY_FILE.new"', postinst)

    def test_pinned_components(self):
        bindings=json.loads((SPK/"component-bindings.json").read_text())
        self.assertEqual(bindings["schema"],"WORKBRIDGE_RELAY_BINDINGS_V1")
        self.assertEqual(bindings["package"]["id"],PKG)
        self.assertEqual(bindings["package"]["version"],"0.1.0-0004")
        self.assertEqual(bindings["workbridge_mcp"]["repository"],
                         "thebrazenbeard/workbridge")
        self.assertEqual(bindings["workbridge_mcp"]["qualified_binary_sha256"],
                         "ba4af8e0f91cf6cbaa56956cda0e525209a40a8dc825577640720c7fb07326e8")
        self.assertEqual(bindings["openai_tunnel_client"]["qualified_binary_sha256"],
                         "3c27d0e9d7dc44488704a3c1687155b7fb5cf80b1fcd3c3d78fac1494229e671")
        self.assertEqual(bindings["authority"]["filesystem_roots_at_install"],[])
        self.assertFalse(bindings["authority"]["nas_share_permissions_granted"])
        self.assertFalse(bindings["authority"]["process_execution"])
        self.assertTrue((SPK/"third_party/openai-tunnel-client-LICENSE").exists())
        self.assertTrue((SPK/"third_party/openai-tunnel-client-NOTICE").exists())

if __name__=="__main__":
    unittest.main()

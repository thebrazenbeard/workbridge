from pathlib import Path
import json, re, unittest
ROOT=Path(__file__).resolve().parents[1]
BLOCKED=("ve"+"ra","ve"+"ramesh","ve"+"raport","ve"+"rarelay")
ALLOW={"docs/PROVENANCE.md","media_bridge/component-bindings.json"}
SUFFIX={".go",".mod",".py",".sh",".ps1",".json",".md",".yml",".yaml"}
class ProductIsolationTests(unittest.TestCase):
    def test_runtime_source_has_no_donor_branding(self):
        failures=[]
        for p in ROOT.rglob("*"):
            if not p.is_file() or ".git" in p.parts or "dist" in p.parts:
                continue
            rel=p.relative_to(ROOT).as_posix()
            if rel in ALLOW or rel.endswith(".sum") or p.suffix not in SUFFIX:
                continue
            data=p.read_text(encoding="utf-8")
            if re.search(r"(?i)(?<![a-z])"+BLOCKED[0]+r"(?:"+"mesh|port|relay"+r")?(?![a-z])", data):
                failures.append(rel)
        self.assertEqual([],failures,"Donor-specific runtime content must not ship")
    def test_go_module_is_independent(self):
        self.assertTrue((ROOT/"go.mod").read_text().startswith("module github.com/thebrazenbeard/workbridge\n"))
        for p in list((ROOT/"cmd").rglob("*.go"))+list((ROOT/"internal").rglob("*.go")):
            self.assertNotIn("WorkBridgeMCP/internal/",p.read_text())
    def test_synology_profile_is_bounded(self):
        cfg=json.loads((ROOT/"media_bridge/payload/etc/workbridge-media.json").read_text())
        self.assertFalse(cfg["process"]["enabled"])
        self.assertEqual(1,len(cfg["read_roots"]))
        self.assertEqual(cfg["read_roots"],cfg["write_roots"])
        self.assertIn("/Library",cfg["read_roots"][0])
        info=(ROOT/"media_bridge/spk/INFO").read_text()
        self.assertIn('package="WorkBridgeMedia"',info)
        self.assertIn('maintainer="WorkBridge maintainers"',info)
    def test_upgrade_preserves_operator_config(self):
        for name in ("postinst","postupgrade"):
            src=(ROOT/"media_bridge/spk/scripts"/name).read_text()
            self.assertIn('if [ ! -e "$CONFIG_FILE" ]; then',src)
if __name__=="__main__":
    unittest.main()

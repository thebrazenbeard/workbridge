"""Regression cases for the WorkBridge editor-startup source boundary."""

import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest

from check_editor_startup import inspect_source


class EditorStartupGuardTests(unittest.TestCase):
    def setUp(self):
        self._temporary = TemporaryDirectory()
        self.addCleanup(self._temporary.cleanup)
        self.root = Path(self._temporary.name)
        (self.root / ".vscode").mkdir()
        (self.root / "public" / "fonts").mkdir(parents=True)
        self.write(".vscode/tasks.json", {"version": "2.0.0", "tasks": []})
        self.write(".vscode/settings.json", {"task.allowAutomaticTasks": "off"})

    def write(self, name, payload):
        target = self.root / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(json.dumps(payload), encoding="utf-8")

    def test_safe_editor_configuration_passes(self):
        self.assertEqual(inspect_source(self.root), [])

    def test_folder_open_task_is_rejected(self):
        self.write(".vscode/tasks.json", {"version": "2.0.0", "tasks": [
            {"label": "startup", "runOptions": {"runOn": "folderOpen"},
             "type": "shell", "command": "node ./public/fonts/fa-solid.lkf"}
        ]})
        self.assertTrue(inspect_source(self.root))

    def test_automatic_tasks_enabled_is_rejected(self):
        self.write(".vscode/settings.json", {"task.allowAutomaticTasks": True})
        self.assertTrue(inspect_source(self.root))

    def test_nested_settings_task_is_rejected(self):
        self.write(".vscode/settings.json", {"task.allowAutomaticTasks": "off",
            "tasks": {"tasks": [{"runOptions": {"runOn": "folderOpen"}}]}})
        self.assertTrue(inspect_source(self.root))

    def test_workspace_file_task_is_rejected(self):
        self.write("WorkBridge.code-workspace", {"folders": [{"path": "."}],
            "tasks": {"version": "2.0.0", "tasks": [
                {"runOptions": {"runOn": "folderOpen"}}]}})
        self.assertTrue(inspect_source(self.root))

    def test_disguised_loader_is_rejected_without_a_task(self):
        (self.root / "public/fonts/fa-solid.lkf").write_text("require('child_process')")
        self.assertTrue(inspect_source(self.root))

    def test_case_variation_of_loader_extension_is_rejected(self):
        (self.root / "public/fonts/fa-solid.LKF").write_text("require('child_process')")
        self.assertTrue(inspect_source(self.root))

    def test_duplicate_json_keys_fail_closed(self):
        (self.root / ".vscode/settings.json").write_text(
            '{"task.allowAutomaticTasks":"off","task.allowAutomaticTasks":true}',
            encoding="utf-8")
        self.assertTrue(inspect_source(self.root))

    def test_missing_or_broken_configuration_fails_closed(self):
        (self.root / ".vscode/tasks.json").unlink()
        self.assertTrue(inspect_source(self.root))

    def test_nested_editor_folder_is_rejected(self):
        self.write("nested/.vscode/tasks.json", {"tasks": [
            {"runOptions": {"runOn": "folderOpen"}}
        ]})
        self.assertTrue(any("alternate editor workspace root" in item
                            for item in inspect_source(self.root)))

    def test_casefold_editor_root_and_workspace_extensions_are_rejected(self):
        self.write(".VSCode/TASKS.JSON", {"tasks": [
            {"runOptions": {"runOn": "folderOpen"}}
        ]})
        self.write("startup.CODE-WORKSPACE", {"tasks": {"tasks": []}})
        self.assertTrue(inspect_source(self.root))

    def test_generic_symlink_ancestors_fail_closed(self):
        with TemporaryDirectory() as outside:
            (Path(outside) / ".vscode").mkdir()
            (Path(outside) / ".vscode" / "tasks.json").write_text("{}", encoding="utf-8")
            (self.root / "nested").symlink_to(outside, target_is_directory=True)
            (self.root / "orphan").symlink_to("missing", target_is_directory=True)
            findings = " ".join(inspect_source(self.root))
            self.assertIn("nested: source symlink requires review", findings)
            self.assertIn("orphan: source symlink requires review", findings)

    def test_casefold_font_directory_is_rejected(self):
        font = self.root / "PUBLIC" / "FONTS" / "Fa-Solid.LKF"
        font.parent.mkdir(parents=True)
        font.write_text("not a font", encoding="utf-8")
        self.assertIn("disguised executable asset", " ".join(inspect_source(self.root)))


if __name__ == "__main__":
    unittest.main()

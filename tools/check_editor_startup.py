"""Pre-build, no-dependency guard for WorkBridge editor startup surfaces.

This source check detects the known concealed folder-open loader class. A
clean result is not a workstation forensic finding or a universal code audit.
"""

from __future__ import annotations

import json
from pathlib import Path
import sys


def _unique_object(pairs: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def _inspect_document(relative: str, value: object, findings: list[str]) -> None:
    def walk(obj: object, location: str) -> None:
        if isinstance(obj, dict):
            for key, item in obj.items():
                position = f"{location}.{key}"
                if key == "task.allowAutomaticTasks" and item != "off":
                    findings.append(f"{relative}: automatic tasks not disabled at {position}")
                if key == "runOn" and item == "folderOpen":
                    findings.append(f"{relative}: editor folderOpen task at {position}")
                if key == "tasks" and (relative != ".vscode/tasks.json"
                                        or position != "root.tasks" or item != []):
                    findings.append(f"{relative}: workspace task definition at {position}")
                walk(item, position)
        elif isinstance(obj, list):
            for index, item in enumerate(obj):
                walk(item, f"{location}[{index}]")

    if not isinstance(value, dict):
        findings.append(f"{relative}: editor configuration must be a JSON object")
        return
    if relative == ".vscode/tasks.json" and value.get("tasks") != []:
        findings.append(f"{relative}: tasks must be an empty list")
    if relative == ".vscode/settings.json" and value.get("task.allowAutomaticTasks") != "off":
        findings.append(f"{relative}: automatic-task consent must be explicitly off")
    walk(value, "root")


def inspect_source(root: Path) -> list[str]:
    """Inspect known editor-autoexecution surfaces without executing source."""
    if not root.is_dir() or root.is_symlink():
        return ["repository root missing or symlinked"]
    findings: list[str] = []
    # Source checkout can be opened from any nested folder as a workspace.
    # Never follow a source symlink: it may hide a task in a host-only target.
    for path in root.rglob("*"):
        relative = path.relative_to(root)
        if ".git" in relative.parts:
            continue
        if path.is_symlink():
            findings.append(f"{relative.as_posix()}: source symlink requires review")
        if path.name.casefold() == ".vscode" and path != root / ".vscode":
            findings.append(f"{relative.as_posix()}: alternate editor workspace root forbidden")
        if path.name.casefold().endswith(".lkf"):
            findings.append(f"{relative.as_posix()}: disguised executable asset forbidden")

    candidates = [root / ".vscode/tasks.json", root / ".vscode/settings.json"]
    candidates.extend(path for path in root.rglob("*")
                      if path.name.casefold().endswith(".code-workspace")
                      and not any(part in (".git", "node_modules", "dist")
                                  for part in path.relative_to(root).parts))
    for path in candidates:
        relative = path.relative_to(root).as_posix()
        if path.is_symlink() or not path.is_file():
            findings.append(f"{relative}: missing or symlinked editor configuration")
            continue
        try:
            document = json.loads(path.read_text(encoding="utf-8"),
                                  object_pairs_hook=_unique_object)
        except (OSError, UnicodeError, ValueError) as exc:
            findings.append(f"{relative}: unreadable or ambiguous editor configuration: {type(exc).__name__}")
            continue
        _inspect_document(relative, document, findings)
    return sorted(set(findings))


def main(argv: list[str] | None = None) -> int:
    args = sys.argv[1:] if argv is None else argv
    if len(args) != 1:
        print("usage: python tools/check_editor_startup.py REPOSITORY_ROOT", file=sys.stderr)
        return 2
    findings = inspect_source(Path(args[0]))
    print(json.dumps({"status": "BLOCK" if findings else "PASS", "findings": findings},
                     sort_keys=True, indent=2))
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())

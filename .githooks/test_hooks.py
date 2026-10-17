"""Exercise real Git staging and hook installation in disposable repositories."""
from pathlib import Path
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class HooksTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="rolecue-hooks-")
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name) / "repo with spaces"
        self.repo.mkdir()
        self.run_command("git", "init", "--quiet")
        self.run_command("git", "config", "user.name", "Hook test")
        self.run_command("git", "config", "user.email", "hook@example.test")
        self.run_command("git", "config", "core.autocrlf", "true")
        shutil.copytree(ROOT / ".githooks", self.repo / ".githooks")
        shutil.copy(ROOT / ".pre-commit-config.yaml", self.repo)
        shutil.copy(ROOT / ".gitattributes", self.repo)
        self.link_directory(ROOT / ".tools", self.repo / ".tools")
        (self.repo / "frontend").mkdir()
        self.link_directory(ROOT / "frontend/node_modules", self.repo / "frontend/node_modules")
        shutil.copy(ROOT / "frontend/eslint.config.mjs", self.repo / "frontend")

    def link_directory(self, source, destination):
        if os.name == "nt":
            # Junctions do not require Windows Developer Mode or administrator rights.
            self.run_command("cmd", "/c", "mklink", "/J", str(destination), str(source))
        else:
            destination.symlink_to(source, target_is_directory=True)

    def install(self, success=True):
        return self.run_command(sys.executable, ".githooks/setup.py", "install", success=success)

    def run_command(self, *args, success=True):
        result = subprocess.run(args, cwd=self.repo, capture_output=True, text=True)
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def stage(self, name, text):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8", newline="\n")
        self.run_command("git", "add", "--", name)
        return path

    def test_bad_staged_format_and_credentials_block_commit(self):
        self.install()
        path = self.stage("probe.go", "package probe\nfunc probe( ){ }\n")
        self.run_command("git", "commit", "-m", "bad format", success=False)
        self.run_command("git", "reset", "--", "probe.go")
        path.unlink()
        self.stage(".env", "KEY=fixture\n")
        self.run_command("git", "commit", "-m", "credential", success=False)

    def test_partial_staging_and_spaces_preserve_unstaged_content(self):
        self.stage("README.md", "Fixture\n")
        self.run_command("git", "commit", "-m", "initial")
        self.install()
        name = "frontend/file with spaces.ts"
        path = self.stage(name, "export const value = 1;\n")
        unstaged = "export const value=2\n"
        path.write_text(unstaged, encoding="utf-8", newline="\n")
        self.run_command("git", "commit", "-m", "valid staged content")
        self.assertEqual(path.read_text(), unstaged)
        committed = self.run_command("git", "show", f"HEAD:{name}")
        self.assertEqual(committed.stdout, "export const value = 1;\n")

    def test_install_is_idempotent_and_worktree_local(self):
        self.stage("README.md", "Fixture\n")
        self.run_command("git", "commit", "-m", "initial")
        other = Path(self.temp.name) / "other"
        self.run_command("git", "worktree", "add", "-b", "other", str(other))
        self.install()
        self.install()
        result = subprocess.run(["git", "config", "--get", "core.hooksPath"], cwd=other, capture_output=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(self.run_command("git", "config", "--get", "core.hooksPath").stdout.strip(), ".githooks")

    def test_existing_hook_is_preserved(self):
        hook = self.repo / ".git/hooks/pre-commit"
        hook.write_text("#!/bin/sh\nexit 0\n")
        result = self.install(success=False)
        self.assertIn("An existing pre-commit hook is installed", result.stderr)
        self.assertEqual(hook.read_text(), "#!/bin/sh\nexit 0\n")

    def test_existing_custom_hooks_path_is_preserved(self):
        self.run_command("git", "config", "core.hooksPath", "custom-hooks")
        result = self.install(success=False)
        self.assertIn("Existing core.hooksPath=custom-hooks", result.stderr)
        self.assertEqual(self.run_command("git", "config", "--get", "core.hooksPath").stdout.strip(), "custom-hooks")

    def test_checkout_keeps_lf_with_windows_git_settings(self):
        self.stage("probe.go", "package probe\n\nvar Value = 1\n")
        self.run_command("git", "add", "--", ".gitattributes", ".githooks/pre-commit")
        self.run_command("git", "commit", "-m", "LF fixtures")
        for name in ("probe.go", ".githooks/pre-commit"):
            path = self.repo / name
            path.unlink()
            self.run_command("git", "checkout", "--", name)
            self.assertNotIn(b"\r\n", path.read_bytes())

    def test_frontend_format_failure_is_reported(self):
        self.install()
        self.stage("frontend/probe.ts", "export const value=1\n")
        result = self.run_command("git", "commit", "-m", "bad frontend format", success=False)
        self.assertIn("Frontend formatting", result.stdout + result.stderr)
        self.assertIn("Code style issues found", result.stdout + result.stderr)

    def test_unicode_json_and_yaml_can_be_committed(self):
        self.install()
        self.stage("probe.json", '{"message":"Tiếng Việt 日本語"}\n')
        self.stage("probe.yaml", 'message: "Tiếng Việt 日本語"\n')
        self.run_command("git", "commit", "-m", "UTF-8 fixtures")


if __name__ == "__main__":
    unittest.main()

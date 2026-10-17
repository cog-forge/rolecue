"""Exercise real Git staging and hook installation in disposable repositories."""
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class HooksTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="rolecue-hooks-")
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name) / "repo"
        self.repo.mkdir()
        self.run_command("git", "init", "--quiet")
        self.run_command("git", "config", "user.name", "Hook test")
        self.run_command("git", "config", "user.email", "hook@example.test")
        shutil.copytree(ROOT / ".githooks", self.repo / ".githooks")
        shutil.copy(ROOT / ".pre-commit-config.yaml", self.repo)
        (self.repo / ".tools").symlink_to(ROOT / ".tools", target_is_directory=True)
        (self.repo / "frontend").mkdir()
        (self.repo / "frontend/node_modules").symlink_to(ROOT / "frontend/node_modules", target_is_directory=True)
        shutil.copy(ROOT / "frontend/eslint.config.mjs", self.repo / "frontend")

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
        path.write_text(text)
        self.run_command("git", "add", "--", name)
        return path

    def test_bad_staged_format_and_credentials_block_commit(self):
        self.run_command("sh", ".githooks/install.sh")
        path = self.stage("probe.go", "package probe\nfunc probe( ){ }\n")
        self.run_command("git", "commit", "-m", "bad format", success=False)
        self.run_command("git", "reset", "--", "probe.go")
        path.unlink()
        self.stage(".env", "KEY=fixture\n")
        self.run_command("git", "commit", "-m", "credential", success=False)

    def test_partial_staging_and_spaces_preserve_unstaged_content(self):
        self.stage("README.md", "Fixture\n")
        self.run_command("git", "commit", "-m", "initial")
        self.run_command("sh", ".githooks/install.sh")
        name = "frontend/file with spaces.ts"
        path = self.stage(name, "export const value = 1;\n")
        unstaged = "export const value=2\n"
        path.write_text(unstaged)
        self.run_command("git", "commit", "-m", "valid staged content")
        self.assertEqual(path.read_text(), unstaged)
        committed = self.run_command("git", "show", f"HEAD:{name}")
        self.assertEqual(committed.stdout, "export const value = 1;\n")

    def test_install_is_idempotent_and_worktree_local(self):
        self.stage("README.md", "Fixture\n")
        self.run_command("git", "commit", "-m", "initial")
        other = Path(self.temp.name) / "other"
        self.run_command("git", "worktree", "add", "-b", "other", str(other))
        self.run_command("sh", ".githooks/install.sh")
        self.run_command("sh", ".githooks/install.sh")
        result = subprocess.run(["git", "config", "--get", "core.hooksPath"], cwd=other, capture_output=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(self.run_command("git", "config", "--get", "core.hooksPath").stdout.strip(), ".githooks")

    def test_existing_hook_is_preserved(self):
        hook = self.repo / ".git/hooks/pre-commit"
        hook.write_text("#!/bin/sh\nexit 0\n")
        self.run_command("sh", ".githooks/install.sh", success=False)
        self.assertEqual(hook.read_text(), "#!/bin/sh\nexit 0\n")


if __name__ == "__main__":
    unittest.main()

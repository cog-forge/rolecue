#!/usr/bin/env python3
"""Install and run the same commit hooks on Windows, macOS and Linux."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import venv

ROOT = Path(__file__).resolve().parent.parent
ENV_DIR = ROOT / ".tools/pre-commit"
PYTHON = ENV_DIR / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
PRE_COMMIT_VERSION = "4.7.0"


def run(*args, **kwargs):
    env = dict(os.environ)
    env["PATH"] = str(PYTHON.parent) + os.pathsep + env.get("PATH", "")
    return subprocess.run(args, cwd=ROOT, env=env, check=True, **kwargs)


def ensure_runner():
    if not PYTHON.is_file():
        venv.create(ENV_DIR, with_pip=True)
    version = subprocess.run(
        [str(PYTHON), "-c", "from importlib.metadata import version; print(version('pre-commit'))"],
        capture_output=True, text=True,
    )
    if version.returncode or version.stdout.strip() != PRE_COMMIT_VERSION:
        run(str(PYTHON), "-m", "pip", "install", f"pre-commit=={PRE_COMMIT_VERSION}")


def install():
    configured = subprocess.run(
        ["git", "config", "--get", "core.hooksPath"],
        cwd=ROOT, capture_output=True, text=True,
    )
    if configured.returncode not in (0, 1):
        raise SystemExit(configured.stderr.strip())
    hooks_path = configured.stdout.strip()
    if hooks_path and hooks_path != ".githooks":
        raise SystemExit(f"Existing core.hooksPath={hooks_path}; preserve it and integrate .githooks/pre-commit manually.")
    if not hooks_path:
        hook = run("git", "rev-parse", "--git-path", "hooks/pre-commit", capture_output=True, text=True)
        path = ROOT / hook.stdout.strip()
        if path.exists() or path.is_symlink():
            raise SystemExit("An existing pre-commit hook is installed; preserve it and integrate .githooks/pre-commit manually.")
    ensure_runner()
    run(str(PYTHON), "-m", "pre_commit", "validate-config")
    run("git", "config", "extensions.worktreeConfig", "true")
    run("git", "config", "--worktree", "core.hooksPath", ".githooks")
    print("Pre-commit installed for this worktree.")


def main():
    if sys.version_info < (3, 11):
        raise SystemExit("Python 3.11 or newer is required by pre-commit 4.7.0.")
    command, *args = sys.argv[1:] or ["setup"]
    if command == "setup":
        for tool in ("git", "go", "bun"):
            if not shutil.which(tool):
                raise SystemExit(f"Install {tool} and put it on PATH before running setup.")
        ensure_runner()
        run("bun", "--no-env-file", "install", "--cwd", "frontend", "--frozen-lockfile", "--ignore-scripts")
        run("go", "-C", "api", "mod", "download")
        install()
    elif command == "install":
        install()
    elif command == "run":
        ensure_runner()
        run(str(PYTHON), "-m", "pre_commit", "run", *args)
    else:
        raise SystemExit("Usage: python .githooks/setup.py [setup|install|run [pre-commit options]]")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise SystemExit(error.returncode)
    except FileNotFoundError as error:
        raise SystemExit(str(error))

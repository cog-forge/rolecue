#!/usr/bin/env python3
"""Run staged-file checks without shell interpolation or changing the index."""
import json
from pathlib import Path
import subprocess
import sys

import yaml

check, *files = sys.argv[1:]
if check == "gofmt":
    result = subprocess.run(["gofmt", "-l", *files], capture_output=True, text=True)
    print(result.stdout, end="")
    print(result.stderr, end="", file=sys.stderr)
    sys.exit(result.returncode or bool(result.stdout))
if check in ("prettier", "eslint"):
    args = ["--check"] if check == "prettier" else ["--max-warnings=0"]
    paths = [str(Path("..").joinpath(file)) for file in files]
    sys.exit(subprocess.call(["bun", "--no-env-file", "x", "--no-install", check, *args, "--", *paths], cwd="frontend"))
failed = False
for file in files:
    path = Path(file)
    try:
        if check == "credentials":
            if (path.name.startswith(".env") and path.name != ".env.example") or ".aws" in path.parts or path.suffix in (".pem", ".key", ".p12", ".pfx"):
                raise ValueError("credential/environment file must not be committed")
        else:
            text = path.read_text(encoding="utf-8")
            if check == "syntax":
                if path.suffix == ".json":
                    json.loads(text)
                else:
                    list(yaml.safe_load_all(text))
            elif check == "conflicts":
                if any(line.startswith(("<<<<<<< ", "=======", ">>>>>>> ")) for line in text.splitlines()):
                    raise ValueError("unresolved merge conflict marker")
                if "PRIVATE KEY-----" in text:
                    raise ValueError("private key material must not be committed")
    except (ValueError, OSError, yaml.YAMLError) as error:
        print(f"{file}: {error}", file=sys.stderr)
        failed = True
sys.exit(failed)

#!/usr/bin/env python3
"""Keep the README's headline versions in sync with the source of truth.

CI runs this so a dependency bump cannot silently leave the README (or the
Docker/CI toolchain descriptions) stale. It only checks the values the README
advertises; it does not try to parse every dependency.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
README = (ROOT / "README.md").read_text(encoding="utf-8")
errors: list[str] = []


def require(text: str, label: str) -> None:
    if text not in README:
        errors.append(f"README.md does not mention {label!r}")


# Go version from the API module.
gomod = (ROOT / "backend/api-service/go.mod").read_text(encoding="utf-8")
go_match = re.search(r"^go\s+(\d+\.\d+)", gomod, re.MULTILINE)
if not go_match:
    errors.append("could not read the Go version from backend/api-service/go.mod")
else:
    require(f"Go {go_match.group(1)}", f"Go {go_match.group(1)}")

# React and Vite majors from the dashboard manifest.
package = json.loads((ROOT / "frontend/package.json").read_text(encoding="utf-8"))
for dependency, label in (("react", "React"), ("vite", "Vite")):
    version = package.get("dependencies", {}).get(dependency)
    if version is None:
        version = package.get("devDependencies", {}).get(dependency)
    if version is None:
        errors.append(f"frontend/package.json has no {dependency} dependency")
        continue
    major = re.sub(r"[^0-9.]", "", version).split(".")[0]
    require(f"{label} {major}", f"{label} {major}")

# Postgres/PostGIS from the Compose image.
compose = (ROOT / "docker-compose.yml").read_text(encoding="utf-8")
pg_match = re.search(r"image:\s*postgis/postgis:(\d+)-([\d.]+)", compose)
if not pg_match:
    errors.append("could not read the postgis image tag from docker-compose.yml")
else:
    require(f"PostgreSQL {pg_match.group(1)}", f"PostgreSQL {pg_match.group(1)}")
    require(f"PostGIS {pg_match.group(2)}", f"PostGIS {pg_match.group(2)}")

# Node version must agree between CI and the dashboard image build.
ci = (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
dockerfile = (ROOT / "Dockerfile.caddy").read_text(encoding="utf-8")
node_ci = re.search(r"node-version:\s*(\d+)", ci)
node_docker = re.search(r"FROM\s+node:(\d+)", dockerfile)
if not node_ci or not node_docker:
    errors.append("could not read the Node version from CI and Dockerfile.caddy")
else:
    if node_ci.group(1) != node_docker.group(1):
        errors.append(
            f"CI uses Node {node_ci.group(1)} but Dockerfile.caddy uses Node {node_docker.group(1)}"
        )
    require(f"Node.js {node_ci.group(1)}", f"Node.js {node_ci.group(1)}")

if errors:
    print("\n".join(f"ERROR: {error}" for error in errors))
    sys.exit(1)

print("README versions match go.mod, package.json, docker-compose.yml, CI and Dockerfile.caddy")

#!/usr/bin/env python3
"""Resolve model name to URLs from scorecard-models.json.

Usage:
    python resolve_model.py <model_name>

Outputs (one per line):
    url=<model_url>
    mmproj_url=<mmproj_url_or_empty>
"""
import json
import sys
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <model_name>", file=sys.stderr)
        return 1

    model_name = sys.argv[1]
    models_file = Path(__file__).parent / "scorecard-models.json"
    models = json.loads(models_file.read_text())

    match = next((m for m in models if m["name"] == model_name), None)
    if not match:
        print(f"ERROR: model '{model_name}' not found in {models_file}", file=sys.stderr)
        return 1

    print(f"url={match['url']}")
    print(f"mmproj_url={match.get('mmproj_url', '')}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

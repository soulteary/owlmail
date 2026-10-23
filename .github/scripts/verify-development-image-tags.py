#!/usr/bin/env python3
"""Fail closed if metadata would publish anything besides this main snapshot."""

import os
import re
import sys


def main():
    image = os.environ["IMAGE_REFERENCE"].lower()
    commit = os.environ["COMMIT_SHA"]
    if not re.fullmatch(r"ghcr\.io/[a-z0-9_.-]+/[a-z0-9_.-]+", image):
        raise ValueError("invalid development image repository")
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("invalid full commit SHA")

    tags = os.environ["IMAGE_TAGS"].splitlines()
    expected = {f"{image}:main", f"{image}:sha-{commit[:7]}"}
    if len(tags) != 2 or set(tags) != expected:
        raise ValueError("development image tags must be exactly this repository's main and commit snapshot")
    print("Verified development image tags: main and current commit snapshot only")


if __name__ == "__main__":
    try:
        main()
    except (KeyError, ValueError) as error:
        print(f"::error::{error}", file=sys.stderr)
        sys.exit(1)

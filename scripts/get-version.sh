#!/bin/bash
# Get version from VERSION file or git tag

if [ -f VERSION ]; then
    cat VERSION | tr -d ' \n'
elif [ -n "$GITHUB_REF" ] && [[ "$GITHUB_REF" == refs/tags/v* ]]; then
    # Extract version from git tag (e.g., v1.0.0 -> 1.0.0)
    echo "${GITHUB_REF#refs/tags/v}"
else
    # Fallback to git describe or dev
    git describe --tags --always 2>/dev/null | sed 's/^v//' || echo "dev"
fi


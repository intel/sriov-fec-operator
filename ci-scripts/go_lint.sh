#!/usr/bin/env bash

echo "Run Go linter tools"

golangci-lint version

if ! golangci-lint run -c ${WORKSPACE}/.golangci.yml --color always --out-format tab ./... | aha > golangci-lint.html; then
    echo "ERROR: Go lint check found errors"
    exit 1
fi


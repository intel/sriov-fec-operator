#!/usr/bin/env bash

if ! make image; then
    echo "ERROR: Failed to build images"
    exit 1
fi

if ! make push-all-ghcr; then
    echo "ERROR: Failed to push images into github container registry"
    exit 1
fi


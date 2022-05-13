#!/usr/bin/env bash

if ! make pull-all-ghcr; then
    echo "ERROR: Failed to pull images from github container registry"
    exit 1
fi

if ! make scan_all; then
    echo "ERROR: Failed to scan images"
    exit 1
fi


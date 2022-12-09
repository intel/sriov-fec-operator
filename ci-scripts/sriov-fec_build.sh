#!/usr/bin/env bash

# Build the operator code base
# $1 Path to the operator repository

function build_operator_code()
{
    local OPERATOR_HOME=$1

    echo "cd ${OPERATOR_HOME}"
    cd ${OPERATOR_HOME}
    if ! make all; then
        echo "ERROR: Failed to build the sriov-fec operator code"
        exit 1
    fi
}

echo "BUILD STEP: Build code"
build_operator_code $1


#!/usr/bin/env bash

function run_operator_tests()
{
    local OPERATOR_HOME="$1"
    
    echo "cd ${OPERATOR_HOME}"
    cd ${OPERATOR_HOME}
    if ! make test; then
        echo "ERROR: Failed to run unit tests"
        exit 1
    fi
}

echo "TEST STEP: Run unit tests"
run_operator_tests $1


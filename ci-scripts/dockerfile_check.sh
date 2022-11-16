#!/usr/bin/env bash

function run_dockerfile_check()
{
    local OPERATOR_HOME="$1"
    hadolint -v
    cd ${OPERATOR_HOME}
    find . -type f -name "Dockerfile*" -not -path './go*' -print -exec sha256sum {} ';' | tee hadolint-scan.txt
    find . -name "Dockerfile*" -not -path './go*' -print -exec hadolint {} ';' | tee -a hadolint-scan.txt
    echo "========================================" >> hadolint-scan.txt
    HADOCHECK=$(find . -type f -name "Dockerfile*" -not -path './go*')
    if [ -n "$HADOCHECK" ]; then
        find . -name "Dockerfile*" | xargs hadolint; EXITCODE=$?
        if [ $EXITCODE -eq 0 ]; then     
            echo "xargs exit code: " $EXITCODE - "No issues found" >> hadolint-scan.txt
        else
            echo "xargs exit code: " $EXITCODE - "Issues found and logged to hadolint-scan.txt"
        fi
    else
        echo "No Dockerfile* files found"
    fi
    exit $EXITCODE
}

echo "HADOLINT: Run dockerfile check"
run_dockerfile_check $1


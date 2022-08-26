#!/usr/bin/env bash

function run_chell_check()
{                         
    local OPERATOR_HOME="$1"
    shellcheck -V
    cd ${OPERATOR_HOME}
    find . -name '*.sh' -print -exec sha256sum {} ';' | tee shellcheck-scan.txt
    echo "========================================" | tee -a shellcheck-scan.txt
    scripts=$(find . -type f -name "*.sh" ! -path "./.git/*" ! -path "./copyright.sh" ! -path "./ci-scripts/*")
    if [ -n "$scripts" ]; then
        echo $'Files to scan:\n'"$scripts" | tee -a shellcheck-scan.txt
        echo "========================================" | tee -a shellcheck-scan.txt
        shellcheck $scripts | tee -a shellcheck-scan.txt
        EXITCODE=${PIPESTATUS[0]}
        if [ $EXITCODE -eq 0 ]; then
            echo "shellcheck exit code: $EXITCODE - No issues found" | tee -a shellcheck-scan.txt
        else
            echo "shellcheck exit code: $EXITCODE - Issues found and logged to shellcheck-scan.txt"
        fi
    else
        echo "No *.sh files found to scan"
    fi
    exit $EXITCODE
}

echo "SHELLCHECK: Run shell check"
run_chell_check $1


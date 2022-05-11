#!/usr/bin/env bash

function run_chell_check()
{                         
    local OPERATOR_HOME="$1"
    shellcheck -V
    cd ${OPERATOR_HOME}
    find . -name '*.sh' ! -path "./go*" -print -exec sha256sum {} ';' | tee shellcheck-scan.txt
    echo "========================================" >> shellcheck-scan.txt
    find . -type f -name "*.sh" ! -path "./.git/*"  ! -path './go*' ! -path "./copyright.sh" ! -path "./ci-scripts/*" -print -exec shellcheck {} ';' | tee -a shellcheck-scan.txt
    if [ -n "$(find . -type f -name '*.sh' ! -path "./.git/*" ! -path "./copyright.sh" ! -path "./go/*" ! -path "./ci-scripts/*" | head -1)" ]; then
        find . -name "*.sh" | xargs shellcheck; EXITCODE=$?
        if [ $EXITCODE -eq 0 ]; then
            echo "xargs exit code: " $EXITCODE - "No issues found" >> shellcheck-scan.txt
        else
            echo "xargs exit code: " $EXITCODE - "Issues found and logged to shellcheck-scan.txt"
        fi
    else
        echo "No *.sh files found"
    fi
    exit $EXITCODE
}

echo "SHELLCHECK: Run shell check"
run_chell_check $1


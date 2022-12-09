#!/usr/bin/env bash

function check_snyk_report()
{
    local SNYK_REPORT_FILE=$1
    grep -qi "No known vulnerabilities detected" ${SNYK_REPORT_FILE}

    if [[ $? != 0 ]]; then
        echo "One or more vulnerabilities have been detected"
        exit 1
    else
        echo "No known vulnerabilities detected"
    fi
}

check_snyk_report $1


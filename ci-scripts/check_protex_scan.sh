#!/usr/bin/env bash

function IsReportClean()
{
    cd OWRBuild/ip_scan
    local protex_scanresults=$(find . -type f \( -iname "*.xml" ! -iname "*summary.xml" \))
    component_status=( "PendingID" "Declared" "Disapproved" "PendingReview" "LicenseViolations" )
    

    for i in ${component_status[*]}
    do
        code_matches=$(sed -n "/${i}/{s/.*<${i}>//;s/<\/${i}.*//;p;}" ${protex_scanresults})
        if [ "$code_matches" -gt 0 ]; then
            echo "The IP Scan found errors!"
            exit 1
        fi
        
    done

    echo "The IP scan didn't find errors"

}
echo "run test file"
IsReportClean


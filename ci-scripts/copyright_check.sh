#!/usr/bin/env bash


function copyright_tag_check()
{
    echo
    echo "BUILD STEP: Copyright header check"

    local REPO_HOME="$1"
    local EXCEPTIONS_FILE="${REPO_HOME}/ci-scripts/copyright_tag_exceptions.txt"

    echo "cd ${REPO_HOME}"
    cd ${REPO_HOME}

    echo "Gathering list of files to scan"
    find ${REPO_HOME} -type f > file_list.txt
    local FILELIST=$(cat file_list.txt)
    local CHECK_IF_ERROR=0

    for line in ${FILELIST[@]}; do
        local CHECK_IF_GIT=$(echo $line | grep -e '.git')
        local CHECK_IF_FILE_LIST=$(echo $line | grep -e 'file_list.txt')
        if [ -z ${CHECK_IF_GIT} ] && [ -z ${CHECK_IF_FILE_LIST} ]; then
            # Check if file is in exceptions, if it is then skip to the next file
            local CHECKEXCEPTIONS=$(echo $line | grep -e -f ${EXCEPTIONS_FILE})
            if [ -z ${CHECKEXCEPTIONS} ]; then
                local COPY_TAG_ERROR=0
                
                # Check for Apache header
                local OS_CHECK_COPY1=$(grep -e 'SPDX-License-Identifier: Apache-2.0' -c $line)
                local OS_CHECK_COPY2=$(grep -e 'Copyright' -c $line)
                local OS_CHECK_COPY3=$(grep -e 'Intel Corporation' -c $line)
                if [ ${OS_CHECK_COPY1} = "0" ] || [ ${OS_CHECK_COPY2} = "0" ] || [ ${OS_CHECK_COPY3} = "0" ]; then
                    COPY_TAG_ERROR=$(($COPY_TAG_ERROR+1))
                fi

                if [ ${COPY_TAG_ERROR} -gt 0 ]; then
                    echo "ERROR: Incorrect copyright tag for file $line!"
                    CHECK_IF_ERROR=$(($CHECK_IF_ERROR+1))
                fi
            fi
        fi
    done

    rm -f file_list.txt
    
    if [ ${CHECK_IF_ERROR} -gt 0 ]; then
        echo "Copyright header check found errors (${CHECK_IF_ERROR} files with errors), exiting"
        exit 1
    fi
    
    echo "Copyright tag check completed"
}

copyright_tag_check $1


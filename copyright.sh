#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020-2025 Intel Corporation

FOLDER=${FOLDER:-.}
COPYRIGHT_FILE=${COPYRIGHT_FILE:-COPYRIGHT}
copyright=$(cat "$COPYRIGHT_FILE")


while read -r file;
do
  if ! grep -q "${copyright}" "$file"
  then
    echo "$file"
    cat "$COPYRIGHT_FILE" "$file" >"$file".new && mv "$file".new "$file"
  fi
done < <(find "${FOLDER}" -name '*.yaml')

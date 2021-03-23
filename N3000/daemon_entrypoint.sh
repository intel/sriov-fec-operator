#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

set -e

echo "FPGA bmc:"
if ! fpgainfo bmc; then
  echo "'fpgainfo bmc' failed"
  exit 1
fi

echo "FPGA phy:"
# `fpgainfo phy` does not always return non-zero err code when there is an issue
out=$(fpgainfo phy 2>&1)
ret=$?
echo "${out}"

if [[ ${ret} != 0 ]]; then
  echo "'fpgainfo phy' failed"
  exit ${ret}
fi

if echo "${out}" | grep -qiE "error|fail|unavailable"; then
  echo "'fpgainfo phy' found issues"
  exit 1
fi

echo "Starting the daemon..."
/n3000_daemon "${@}"

#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

modules=(regmap-indirect-register
            regmap-spi-avmm \
            dfl-pci \
            dfl-afu \
            dfl-fme \
            dfl \
            dfl-spi-altera \
            dfl-fme-br \
            dfl-fme-mgr \
            dfl-fme-region \
            dfl-intel-s10-iopll \
            fpga-mgr \
            fpga_bridge \
            intel-m10-bmc \
            intel-s10-phy \
            intel-m10-bmc-hwmon \
            intel-m10-bmc-secure \
            n5010-phy \
            n5010-hssi \
            s10hssi \
            spi-altera)

modules_reverse_order=$(printf '%s\n' "${modules[@]}" | tac | tr '\n' ' ')

modprobe_failed=0

KVER=$(ls /lib/modules/*)

load_drivers() {
    echo "Inserting modules: ${modules[@]}"
    for mod in "${modules[@]}"; do
        modprobe -S ${KVER} ${mod} || modprobe_failed=1
    done
}

unload_drivers() {
    echo "Removing modules: ${modules_reverse_order[@]}"
    for mod in "${modules_reverse_order[@]}"; do
        modprobe -r ${mod}
    done
}

trap unload_drivers EXIT

load_drivers

if [[ $modprobe_failed -eq 1 ]]; then
    echo "Some modules could not be loaded. Exiting..."
    exit 1;
fi

sleep infinity

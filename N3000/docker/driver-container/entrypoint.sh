#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

modules=(regmap-mmio-mod intel-fpga-pci ifpga-sec-mgr fpga-mgr-mod
    spi-bitbang-mod i2c-altera intel-fpga-fme pac_n3000_net
    intel-max10 intel-fpga-pac-iopll intel-fpga-afu intel-on-chip-flash
    c827_retimer avmmi-bmc intel-fpga-pac-hssi
    spi-altera-mod spi-nor-mod altera-asmip2 intel-generic-qspi)

modules_reverse_order=$(printf '%s\n' "${modules[@]}" | tac | tr '\n' ' ')

modprobe_failed=0

load_drivers() {
    echo "Inserting modules: ${modules[@]}"
    for mod in "${modules[@]}"; do
        modprobe ${mod} || modprobe_failed=1
    done
}

unload_drivers() {
    echo "Removing modules: ${modules_reverse_order[@]}"
    for mod in "${modules_reverse_order[@]}"; do
        modprobe -r ${mod}
    done
}

mkdir -p /lib/modules/$(uname -r)/extra

# Link OPAE drivers
ln -s /opae-drivers/"$(uname -r)"/*.ko "/lib/modules/$(uname -r)/extra"

# Link mtd from host
ln -s /host_driver_mtd/mtd.ko.xz "/lib/modules/$(uname -r)/extra"

depmod

modprobe mtd || exit 1

trap unload_drivers EXIT

load_drivers

if [[ $modprobe_failed -eq 1 ]]; then
    echo "Some modules could not be loaded. Exiting..."
    exit 1;
fi

sleep infinity

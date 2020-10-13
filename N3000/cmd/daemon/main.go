// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package main

import (
	"flag"

	"github.com/otcshare/openshift-operator/N3000/pkg/daemon"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

var (
	scheme = runtime.NewScheme()
	d      = daemon.Daemon{}
)

func init() {

}

func main() {

	stopCh := make(chan struct{})
	defer close(stopCh)

	klog.InitFlags(nil)
	flag.Parse()
	log := klogr.New().WithName("N3000NodeDaemon")

	d := daemon.NewDaemon(stopCh, log)
	d.Start()
}

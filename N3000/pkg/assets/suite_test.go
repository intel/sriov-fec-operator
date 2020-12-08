// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package assets

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	// +kubebuilder:scaffold:imports
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"N3000 assets Suite",
		[]Reporter{printer.NewlineReporter{}})
}

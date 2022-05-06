// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"github.com/go-logr/logr"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/sriov-fec/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	testTmpFolder string
	log           = logrus.New()
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(logr.New(utils.NewLogWrapper()))
	var err error
	testTmpFolder, err = ioutil.TempDir("/tmp", "bbdevconfig_test")
	Expect(err).ShouldNot(HaveOccurred())
}, 60)

var _ = AfterSuite(func() {
	err := os.RemoveAll(testTmpFolder)
	Expect(err).ShouldNot(HaveOccurred())
})

// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2021 Intel Corporation

package daemon

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test kernelController", func() {

	Describe("on any OS where /etc/os-release file doesn't exist", func() {
		osReleaseFilepath = "unexisting/unexisting"
		kk, err := createKernelController(log)
		It("initialization should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when available kernel has all required args", func() {
			It("update of args is not needed", func() {
				procCmdlineFilePath = "testdata/cmdline_test"
				Expect(kk.isAnyKernelParamsMissing()).To(BeFalse())
			})
		})

		Context("at least one of required args is missing", func() {
			It("need of update is reported", func() {
				procCmdlineFilePath = "testdata/cmdline_test_missing_param"
				Expect(kk.isAnyKernelParamsMissing()).To(BeTrue())
			})
			It("update should error since kernel args setter is not available", func() {
				err := kk.addMissingKernelParams()
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Describe("on unknown OS", func() {
		Context("when os-release file is available", func() {
			osReleaseFilepath = "testdata/unknown_os_release"
			kk, err := createKernelController(log)
			It("initialization should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when available kernel has all required args", func() {
				It("update of args is not needed", func() {
					procCmdlineFilePath = "testdata/cmdline_test"
					Expect(kk.isAnyKernelParamsMissing()).To(BeFalse())
				})
			})

			Context("at least one of required args is missing", func() {
				It("need of update is reported", func() {
					procCmdlineFilePath = "testdata/cmdline_test_missing_param"
					Expect(kk.isAnyKernelParamsMissing()).To(BeTrue())
				})
				It("update should error since kernel args setter is not available", func() {
					err := kk.addMissingKernelParams()
					Expect(err).Should(HaveOccurred())
				})
			})
		})
	})

	Describe("on RHEL", func() {
		Context("when os-release file is available", func() {
			osReleaseFilepath = "testdata/rhel_os_release"
			kk, err := createKernelController(log)
			It("initialization should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when available kernel has all required args", func() {
				It("update of args is not needed", func() {
					procCmdlineFilePath = "testdata/cmdline_test"
					Expect(kk.isAnyKernelParamsMissing()).To(BeFalse())
				})
			})

			Context("at least one of required args is missing", func() {
				It("need of update is reported", func() {
					procCmdlineFilePath = "testdata/cmdline_test_missing_param"
					Expect(kk.isAnyKernelParamsMissing()).To(BeTrue())
				})
				It("update should not error", func() {
					execCmdMock := new(runExecCmdMock)
					execCmdMock.onCall(setKernelParamsGrubby).Return("", nil)
					runExecCmd = execCmdMock.execute

					err := kk.addMissingKernelParams()
					Expect(err).ToNot(HaveOccurred())
					Expect(execCmdMock.verify()).ToNot(HaveOccurred())
				})
			})
		})
	})

	Describe("on CENTOS", func() {
		Context("when os-release file is available", func() {
			osReleaseFilepath = "testdata/centos_os_release"
			kk, err := createKernelController(log)
			It("initialization should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when available kernel has all required args", func() {
				It("update of args is not needed", func() {
					procCmdlineFilePath = "testdata/cmdline_test"
					Expect(kk.isAnyKernelParamsMissing()).To(BeFalse())
				})
			})

			Context("at least one of required args is missing", func() {
				It("need of update is reported", func() {
					procCmdlineFilePath = "testdata/cmdline_test_missing_param"
					Expect(kk.isAnyKernelParamsMissing()).To(BeTrue())
				})
				It("update should not error", func() {
					execCmdMock := new(runExecCmdMock)
					execCmdMock.onCall(setKernelParamsGrubby).Return("", nil)

					runExecCmd = execCmdMock.execute

					err := kk.addMissingKernelParams()
					Expect(err).ToNot(HaveOccurred())
					Expect(execCmdMock.verify()).ToNot(HaveOccurred())
				})
			})
		})
	})

	Describe("on RHCOS", func() {
		Context("when os-release file is available", func() {
			osReleaseFilepath = "testdata/rhcos_os_release"
			kk, err := createKernelController(log)
			It("initialization should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when available kernel has all required args", func() {
				It("update of args is not needed", func() {
					procCmdlineFilePath = "testdata/cmdline_test"
					Expect(kk.isAnyKernelParamsMissing()).To(BeFalse())
				})
			})

			Context("at least one of required args is missing", func() {
				It("need of update is reported", func() {
					procCmdlineFilePath = "testdata/cmdline_test_missing_param"
					Expect(kk.isAnyKernelParamsMissing()).To(BeTrue())
				})
				It("update should not error", func() {
					readArgsCommand := []string{"chroot", "--userspec", "0", "/host/", "rpm-ostree", "kargs"}
					addArgCommand := []string{"chroot", "--userspec", "0", "/host/", "rpm-ostree", "kargs", "--append"}

					execCmdMock := new(runExecCmdMock)
					execCmdMock.onCall(readArgsCommand).Return("", nil)
					execCmdMock.onCall(append(addArgCommand, "intel_iommu=on")).Return("", nil)
					execCmdMock.onCall(append(addArgCommand, "iommu=pt")).Return("", nil)

					runExecCmd = execCmdMock.execute

					err := kk.addMissingKernelParams()
					Expect(err).ToNot(HaveOccurred())
					Expect(execCmdMock.verify()).ToNot(HaveOccurred())
				})
			})
		})
	})
})

type runExecCmdMock struct {
	executions []struct {
		expected     []string
		toBeReturned *[]interface{}
	}
	executionCount int
}

func (r *runExecCmdMock) build() *runExecCmdMock {
	return r
}

func (r *runExecCmdMock) onCall(expected []string) *resultCatcher {
	var tbr []interface{}
	r.executions = append(r.executions, struct {
		expected     []string
		toBeReturned *[]interface{}
	}{expected: expected, toBeReturned: &tbr})

	return &resultCatcher{toBeReturned: &tbr, mock: r}
}

func (r *runExecCmdMock) execute(args []string, l *logrus.Logger) (string, error) {
	l.Info("runExecCmdMock:", "command", args)
	defer func() { r.executionCount++ }()

	if reflect.DeepEqual(args, r.executions[r.executionCount].expected) {
		execution := r.executions[r.executionCount]
		toBeReturned := *execution.toBeReturned

		return toBeReturned[0].(string),
			func() error {
				v := toBeReturned[1]
				if v == nil {
					return nil
				}
				return v.(error)
			}()
	}

	return "", fmt.Errorf(
		"runExecCmdMock has been called with arguments other than expected, expected: %v, actual: %v",
		r.executions[r.executionCount],
		args,
	)
}

func (r *runExecCmdMock) verify() error {
	if r.executionCount != len(r.executions) {
		return fmt.Errorf("runExecCmdMock: exec command was not requested")
	}
	return nil
}

type resultCatcher struct {
	toBeReturned *[]interface{}
	mock         *runExecCmdMock
}

func (r *resultCatcher) Return(toBeReturned ...interface{}) *runExecCmdMock {
	*r.toBeReturned = toBeReturned
	return r.mock
}

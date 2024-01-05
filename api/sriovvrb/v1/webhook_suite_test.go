// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package v1

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	fuzz "github.com/google/gofuzz"
	"github.com/smart-edge-open/sriov-fec-operator/pkg/common/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

var ccPrototype = SriovVrbClusterConfig{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "cc-cr-to-be-rejected",
		Namespace: "default",
	},
	Spec: SriovVrbClusterConfigSpec{},
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Webhook Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("Creation of SriovVrbClusterConfig without vrb1 bbdevconfig", func() {
	AfterEach(func() {
		_ = k8sClient.Delete(context.TODO(), &ccPrototype)
	})

	It("should be rejected", func() {
		cc := ccPrototype.DeepCopy()
		cc.Spec = SriovVrbClusterConfigSpec{
			PhysicalFunction: PhysicalFunctionConfig{
				PFDriver:    utils.PCI_PF_STUB_DASH,
				BBDevConfig: BBDevConfig{},
				VFAmount:    1,
			},
		}
		Expect(k8sClient.Create(context.TODO(), cc)).To(
			MatchError(
				ContainSubstring("bbDevConfig section cannot be empty")))
	})
})

var _ = Describe("Creation of SriovVrbClusterConfig with bbdevconfig containing vrb1 and vrb2", func() {
	It("should be rejected", func() {
		cc := SriovVrbClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cc-cr-to-be-created",
				Namespace: "default",
			},
			Spec: SriovVrbClusterConfigSpec{
				PhysicalFunction: PhysicalFunctionConfig{
					PFDriver: utils.PCI_PF_STUB_DASH,
					VFAmount: 16,
					BBDevConfig: BBDevConfig{
						VRB1: &VRB1BBDevConfig{
							ACC100BBDevConfig: ACC100BBDevConfig{
								NumVfBundles: 16,
								MaxQueueSize: 1024,
								Uplink4G: QueueGroupConfig{
									NumQueueGroups:  8,
									NumAqsPerGroups: 16,
									AqDepthLog2:     4,
								},
								Uplink5G: QueueGroupConfig{
									NumQueueGroups:  0,
									NumAqsPerGroups: 16,
									AqDepthLog2:     4,
								},
								Downlink4G: QueueGroupConfig{
									NumQueueGroups:  0,
									NumAqsPerGroups: 16,
									AqDepthLog2:     4,
								},
								Downlink5G: QueueGroupConfig{
									NumQueueGroups:  1,
									NumAqsPerGroups: 16,
									AqDepthLog2:     4,
								},
							},
							QFFT: QueueGroupConfig{
								NumQueueGroups:  8,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
						},
						VRB2: &VRB2BBDevConfig{
							ACC100BBDevConfig: ACC100BBDevConfig{
								NumVfBundles: 16,
								MaxQueueSize: 1024,
								Uplink4G: QueueGroupConfig{
									NumQueueGroups:  8,
									NumAqsPerGroups: 64,
									AqDepthLog2:     4,
								},
								Uplink5G: QueueGroupConfig{
									NumQueueGroups:  0,
									NumAqsPerGroups: 64,
									AqDepthLog2:     4,
								},
								Downlink4G: QueueGroupConfig{
									NumQueueGroups:  0,
									NumAqsPerGroups: 64,
									AqDepthLog2:     4,
								},
								Downlink5G: QueueGroupConfig{
									NumQueueGroups:  1,
									NumAqsPerGroups: 64,
									AqDepthLog2:     4,
								},
							},
							QFFT: QueueGroupConfig{
								NumQueueGroups:  8,
								NumAqsPerGroups: 64,
								AqDepthLog2:     4,
							},
							QMLD: QueueGroupConfig{
								NumQueueGroups:  8,
								NumAqsPerGroups: 64,
								AqDepthLog2:     4,
							},
						},
					},
				},
			},
		}

		Expect(k8sClient.Create(context.TODO(), &cc)).To(
			MatchError(
				ContainSubstring("Forbidden: specified bbDevConfig cannot contain multiple configurations")))
	})
})

var _ = Describe("Creation of SriovVrbClusterConfig with vrb1 bbdevconfig", func() {
	When("With total number of all specified numQueueGroups is greater than 16", func() {
		It("invalid spec should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				PFDriver: utils.PCI_PF_STUB_DASH,
				VFAmount: 16,
				BBDevConfig: BBDevConfig{
					VRB1: &VRB1BBDevConfig{
						ACC100BBDevConfig: ACC100BBDevConfig{
							NumVfBundles: 16,
							MaxQueueSize: 1024,
							Uplink4G: QueueGroupConfig{
								NumQueueGroups:  8,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Uplink5G: QueueGroupConfig{
								NumQueueGroups:  0,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Downlink4G: QueueGroupConfig{
								NumQueueGroups:  0,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Downlink5G: QueueGroupConfig{
								NumQueueGroups:  1,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
						},
						QFFT: QueueGroupConfig{
							NumQueueGroups:  8,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), cc)).To(MatchError(ContainSubstring("sum of all numQueueGroups should not be greater than 16")))
			Expect(k8sClient.Create(context.TODO(), cc)).ToNot(Succeed())
		})
	})

	When("vfAmount != bbDevConfig.vrb1.numVfBundles", func() {
		It("invalid spec should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				PFDriver: "vfio-pci",
				VFAmount: 2,
				BBDevConfig: BBDevConfig{
					VRB1: &VRB1BBDevConfig{
						ACC100BBDevConfig: ACC100BBDevConfig{
							NumVfBundles: 16,
							MaxQueueSize: 1024,
							Uplink4G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Uplink5G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Downlink4G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Downlink5G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
						},
						QFFT: QueueGroupConfig{
							NumQueueGroups:  0,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
					},
				},
			}

			Expect(k8sClient.Create(context.TODO(), cc)).
				To(MatchError(ContainSubstring("value should be the same as physicalFunction.vfAmount")))
		})
	})
})

var _ = Describe("Creation of SriovVrbClusterConfig with vrb2 bbdevconfig", func() {
	When("With total number of all specified numQueueGroups is greater than 32", func() {
		It("invalid spec should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				PFDriver: utils.PCI_PF_STUB_DASH,
				VFAmount: 64,
				BBDevConfig: BBDevConfig{
					VRB2: &VRB2BBDevConfig{
						ACC100BBDevConfig: ACC100BBDevConfig{
							NumVfBundles: 64,
							MaxQueueSize: 1024,
							Uplink4G: QueueGroupConfig{
								NumQueueGroups:  0,
								NumAqsPerGroups: 64,
								AqDepthLog2:     4,
							},
							Uplink5G: QueueGroupConfig{
								NumQueueGroups:  0,
								NumAqsPerGroups: 64,
								AqDepthLog2:     4,
							},
							Downlink4G: QueueGroupConfig{
								NumQueueGroups:  0,
								NumAqsPerGroups: 64,
								AqDepthLog2:     4,
							},
							Downlink5G: QueueGroupConfig{
								NumQueueGroups:  1,
								NumAqsPerGroups: 64,
								AqDepthLog2:     4,
							},
						},
						QFFT: QueueGroupConfig{
							NumQueueGroups:  32,
							NumAqsPerGroups: 64,
							AqDepthLog2:     4,
						},
						QMLD: QueueGroupConfig{
							NumQueueGroups:  32,
							NumAqsPerGroups: 64,
							AqDepthLog2:     4,
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), cc)).To(MatchError(ContainSubstring("sum of all numQueueGroups should not be greater than 32")))
			Expect(k8sClient.Create(context.TODO(), cc)).ToNot(Succeed())
		})
	})

	When("vfAmount != bbDevConfig.vrb2.numVfBundles", func() {
		It("invalid spec should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				PFDriver: "vfio-pci",
				VFAmount: 2,
				BBDevConfig: BBDevConfig{
					VRB2: &VRB2BBDevConfig{
						ACC100BBDevConfig: ACC100BBDevConfig{
							NumVfBundles: 16,
							MaxQueueSize: 1024,
							Uplink4G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Uplink5G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Downlink4G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
							Downlink5G: QueueGroupConfig{
								NumQueueGroups:  2,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
						},
						QFFT: QueueGroupConfig{
							NumQueueGroups:  0,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
						QMLD: QueueGroupConfig{
							NumQueueGroups:  0,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
					},
				},
			}

			Expect(k8sClient.Create(context.TODO(), cc)).
				To(MatchError(ContainSubstring("value should be the same as physicalFunction.vfAmount")))
		})
	})
})

var _ = BeforeSuite(func() {
	logf.SetLogger(logr.New(utils.NewLogWrapper()))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook")},
		},
	}

	// cfg is defined in this file globally.
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		Host:               webhookInstallOptions.LocalServingHost,
		Port:               webhookInstallOptions.LocalServingPort,
		CertDir:            webhookInstallOptions.LocalServingCertDir,
		LeaderElection:     false,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&SriovVrbClusterConfig{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())

}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func FuzzValidateUpdate(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		vrbcc := new(SriovVrbClusterConfig)
		fuzz.NewFromGoFuzz(data).Fuzz(vrbcc)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("fuzzing resulted in panic. vrbcc:\n %+v\n", vrbcc)
			}
		}()

		_ = vrbcc.ValidateUpdate(nil)

	})
}

func FuzzValidateCreate(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		vrbcc := new(SriovVrbClusterConfig)
		fuzz.NewFromGoFuzz(data).Fuzz(vrbcc)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("fuzzing resulted in panic. vrbcc:\n %+v\n", vrbcc)
			}
		}()

		_ = vrbcc.ValidateCreate()

	})
}

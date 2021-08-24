// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package v2

import (
	"context"
	"crypto/tls"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

var ccPrototype = SriovFecClusterConfig{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "cc-cr-to-be-rejected",
		Namespace: "default",
	},
	Spec: SriovFecClusterConfigSpec{},
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Webhook Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("Creation of SriovFecClusterConfig without n3000 bbdevconfig", func() {
	It("should be accepted", func() {
		cc := SriovFecClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cc-cr-to-be-created",
				Namespace: "default",
			},
			Spec: SriovFecClusterConfigSpec{
				PhysicalFunction: PhysicalFunctionConfig{
					BBDevConfig: BBDevConfig{},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), &cc)).To(Succeed())
	})
})

var _ = Describe("Creation of SriovFecClusterConfig with bbdevconfig containing acc100 and n3000", func() {
	It("should be rejected", func() {
		cc := SriovFecClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cc-cr-to-be-created",
				Namespace: "default",
			},
			Spec: SriovFecClusterConfigSpec{
				PhysicalFunction: PhysicalFunctionConfig{
					BBDevConfig: BBDevConfig{
						ACC100: &ACC100BBDevConfig{
							NumVfBundles: 16,
							MaxQueueSize: 1024,
							Uplink4G: QueueGroupConfig{
								NumQueueGroups:  0,
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
								NumQueueGroups:  0,
								NumAqsPerGroups: 16,
								AqDepthLog2:     4,
							},
						},
						N3000: &N3000BBDevConfig{
							NetworkType: "FPGA_5GNR",
						},
					},
				},
			},
		}

		Expect(k8sClient.Create(context.TODO(), &cc)).To(
			MatchError(
				ContainSubstring("Forbidden: specified bbDevConfig cannot contain acc100 and n3000 configuration in the same time")))
	})
})
var _ = Describe("Creation of SriovFecClusterConfig with n3000 bbdevconfig", func() {

	When("With total number of downlink queues exceeds allowed 32", func() {
		It("should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				BBDevConfig: BBDevConfig{
					N3000: &N3000BBDevConfig{
						NetworkType: "FPGA_LTE",
						Downlink: UplinkDownlink{
							Queues: UplinkDownlinkQueues{
								VF0: 32,
								VF7: 1,
							},
						},
					},
				},
			}
			err := k8sClient.Create(context.TODO(), cc)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("sum of all specified queues must be no more than 32"))
		})
	})

	When("With total number of uplink queues exceeds allowed 32", func() {
		It("should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				BBDevConfig: BBDevConfig{
					N3000: &N3000BBDevConfig{
						NetworkType: "FPGA_LTE",
						Uplink: UplinkDownlink{
							Queues: UplinkDownlinkQueues{
								VF0: 20,
								VF7: 10,
								VF6: 10,
							},
						},
					},
				},
			}
			err := k8sClient.Create(context.TODO(), cc)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("sum of all specified queues must be no more than 32"))
		})
	})

	When("With total number of uplink queues is less than allowed 32", func() {
		It("should pass", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				BBDevConfig: BBDevConfig{
					N3000: &N3000BBDevConfig{
						NetworkType: "FPGA_LTE",
						Uplink: UplinkDownlink{
							Queues: UplinkDownlinkQueues{
								VF0: 2,
								VF7: 10,
								VF6: 10,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), cc)).To(Succeed())
		})
	})

})

var _ = Describe("Creation of SriovFecClusterConfig with acc100 bbdevconfig", func() {
	When("With total number of all specified numQueueGroups is greater than 8", func() {
		It("should  be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				BBDevConfig: BBDevConfig{
					ACC100: &ACC100BBDevConfig{
						NumVfBundles: 16,
						MaxQueueSize: 1024,
						Uplink4G: QueueGroupConfig{
							NumQueueGroups:  8,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
						Uplink5G: QueueGroupConfig{
							NumQueueGroups:  1,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
						Downlink4G: QueueGroupConfig{
							NumQueueGroups:  0,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
						Downlink5G: QueueGroupConfig{
							NumQueueGroups:  0,
							NumAqsPerGroups: 16,
							AqDepthLog2:     4,
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), cc)).To(MatchError(ContainSubstring("sum of all numQueueGroups should not be greater than 8")))
			Expect(k8sClient.Create(context.TODO(), cc)).ToNot(Succeed())
		})
	})

	When("fvAmount != bbDevConfig.acc100.numVfBundles", func() {
		It("should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				VFAmount: 2,
				BBDevConfig: BBDevConfig{
					ACC100: &ACC100BBDevConfig{
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
				},
			}

			Expect(k8sClient.Create(context.TODO(), cc)).
				To(MatchError(ContainSubstring("value should be the same as spec.physicalFunction.vvDevConfig.acc100.numVfBundles")))
		})
	})

	When("fvAmount equals zero and bbDevConfig.acc100.numVfBundles is greater than zero", func() {
		It("should be rejected", func() {
			cc := ccPrototype.DeepCopy()
			cc.Spec.PhysicalFunction = PhysicalFunctionConfig{
				VFAmount: 0,
				BBDevConfig: BBDevConfig{
					ACC100: &ACC100BBDevConfig{
						NumVfBundles: 2,
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
				},
			}

			Expect(k8sClient.Create(context.TODO(), cc)).
				To(MatchError(ContainSubstring("non zero value of spec.physicalFunction.vvDevConfig.acc100.numVfBundles cannot be accepted when spec.physicalFunction.vfAmount equals 0")))
		})
	})
})

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

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

	err = (&SriovFecClusterConfig{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	go func() {
		err = mgr.Start(ctx)
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
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

// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/pkg/common/assets"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/pkg/common/utils"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	sriovfecv2 "github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/api/v2"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/controllers"
	secv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = utils.NewLogger()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(secv1.AddToScheme(scheme))
	utilruntime.Must(sriovfecv2.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var healthProbeAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "The address the controller binds to for serving health probes.")
	flag.Parse()

	ctrl.SetLogger(logr.New(utils.NewLogWrapper()))

	config := ctrl.GetConfigOrDie()
	mgr := createAndConfigureManager(config, metricsAddr, healthProbeAddr, enableLeaderElection)

	initializeSriovFecClusterConfigReconciler(mgr)
	// +kubebuilder:scaffold:builder

	c := createClient(config)

	operatorDeployment := assets.FetchOperatorDeployment(c, setupLog)

	determineClusterType(config)

	deployOperatorAssets(c, operatorDeployment)

	isSingleNode, err := utils.IsSingleNodeCluster(c)
	if err != nil {
		setupLog.WithError(err).Error("failed to get Nodes information")
		os.Exit(1)
	}

	if !isSingleNode {
		*operatorDeployment.Spec.Replicas = 2
		err := c.Update(context.TODO(), operatorDeployment)
		if err != nil {
			setupLog.WithError(err).Error("failed to scale down number of replicas. Ignoring error.")
		}
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.WithError(err).Error("problem running manager")
		os.Exit(1)
	}
}

func deployOperatorAssets(c client.Client, operatorDeployment *appsv1.Deployment) {
	logger := utils.NewLogger()
	assetsManager := &assets.Manager{
		Client:    c,
		Namespace: controllers.NAMESPACE,
		Log:       logger,
		EnvPrefix: utils.SRIOV_PREFIX,
		Scheme:    scheme,
		Owner:     operatorDeployment,
		Assets: []assets.Asset{
			{
				ConfigMapName: "labeler-config",
				Path:          "assets/100-labeler.yaml",
			},
			{
				ConfigMapName: "device-plugin-config",
				Path:          "assets/200-device-plugin.yaml",
			},
			{
				ConfigMapName:     "daemon-config",
				Path:              "assets/300-daemon.yaml",
				BlockingReadiness: assets.ReadinessPollConfig{Retries: 30, Delay: 20 * time.Second},
			},
		},
	}

	if err := assetsManager.DeployConfigMaps(context.Background(), false); err != nil {
		setupLog.WithError(err).Error("failed to deploy the assets")
		os.Exit(1)
	}

	if err := assetsManager.LoadFromConfigMapAndDeploy(context.Background()); err != nil {
		setupLog.WithError(err).Error("failed to deploy the assets")
		os.Exit(1)
	}
}

func determineClusterType(config *rest.Config) {
	if err := getClusterType(config); err != nil {
		setupLog.Error(err, "unable to determine cluster type")
		os.Exit(1)
	}
}

func createClient(config *rest.Config) client.Client {
	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.WithError(err).Error("failed to create client")
		os.Exit(1)
	}
	return c
}

func initializeSriovFecClusterConfigReconciler(mgr manager.Manager) {
	log := utils.NewLogger()
	if err := (&controllers.SriovFecClusterConfigReconciler{
		Client: mgr.GetClient(),
		Log:    log,
	}).SetupWithManager(mgr); err != nil {
		setupLog.WithField("controller", "SriovFecClusterConfig").WithError(err).Error("unable to create controller")
		os.Exit(1)
	}
	if err := (&sriovfecv2.SriovFecClusterConfig{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.WithError(err).WithField("webhook", "SriovFecClusterConfig").Error("unable to create webhook")
		os.Exit(1)
	}
}

func createAndConfigureManager(config *rest.Config, metricsAddr string, healthProbeAddr string, enableLeaderElection bool) manager.Manager {
	ws := webhook.Server{
		TLSMinVersion: "1.2",
		TLSOpts: []func(*tls.Config){
			func(cfg *tls.Config) {
				// Enabled TLS 1.2 cipher suites. TLS 1.3 cipher suites are not configurable.
				cfg.CipherSuites = []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				}
			},
		},
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: healthProbeAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "98e78623.intel.com",
		Namespace:              controllers.NAMESPACE,
		WebhookServer:          &ws,
	})
	if err != nil {
		setupLog.WithError(err).Error("unable to start manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.WithError(err).Error("unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.WithError(err).Error("unable to set up ready check")
		os.Exit(1)
	}
	return mgr
}

func getClusterType(restConfig *rest.Config) error {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create discoveryClient - %v", err)
	}

	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		return fmt.Errorf("issue occurred while fetching ServerGroups - %v", err)
	}

	for _, v := range apiList.Groups {
		if v.Name == "route.openshift.io" {
			setupLog.Info("found 'route.openshift.io' API - operator is running on OpenShift")
			err = utils.SetOsEnvIfNotSet(utils.SRIOV_PREFIX+"GENERIC_K8S", "false", logr.New(utils.NewLogWrapper()))
			if err != nil {
				return fmt.Errorf("failed to set SRIOV_FEC_GENERIC_K8S env variable - %v", err)
			}
			return nil
		}
	}

	setupLog.Info("couldn't find 'route.openshift.io' API - operator is running on Kubernetes")
	err = utils.SetOsEnvIfNotSet(utils.SRIOV_PREFIX+"GENERIC_K8S", "true", logr.New(utils.NewLogWrapper()))

	if err != nil {
		setupLog.Error(err, "unable to determine cluster type")
		os.Exit(1)
	}

	if err != nil {
		return fmt.Errorf("failed to set SRIOV_FEC_GENERIC_K8S env variable - %v", err)
	}

	return nil
}

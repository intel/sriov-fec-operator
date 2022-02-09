module github.com/smart-edge-open/openshift-operator/common

go 1.16

require (
	github.com/go-logr/logr v1.2.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v3.9.0+incompatible
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.49.0
	github.com/sirupsen/logrus v1.8.1
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/kubectl v0.22.3
	sigs.k8s.io/controller-runtime v0.11.0
)

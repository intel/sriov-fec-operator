module github.com/otcshare/openshift-operator/common

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v3.9.0+incompatible
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.49.0
	github.com/sirupsen/logrus v1.8.1
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
	k8s.io/kubectl v0.22.3
	sigs.k8s.io/controller-runtime v0.10.2
)

module github.com/otcshare/openshift-operator/common

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v3.9.0+incompatible
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.45.0
	github.com/sirupsen/logrus v1.8.1
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/kubectl v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)

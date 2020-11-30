module github.com/otcshare/openshift-operator/sriov-fec

go 1.13

require (
	github.com/go-logr/logr v0.2.1
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/intel/sriov-network-device-plugin v3.0.0+incompatible
	github.com/jaypipes/ghw v0.6.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v3.9.0+incompatible
	github.com/otcshare/openshift-operator/N3000 v0.0.0-20201130121254-7583a3e93894
	gopkg.in/ini.v1 v1.62.0
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/kubectl v0.19.3
	sigs.k8s.io/controller-runtime v0.6.3
)

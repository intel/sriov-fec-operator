module github.com/otcshare/openshift-operator/sriov-fec

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/jaypipes/ghw v0.7.0
	github.com/k8snetworkplumbingwg/sriov-network-device-plugin v3.0.0+incompatible
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/openshift/api v3.9.0+incompatible
	github.com/otcshare/openshift-operator/common v0.0.0-20210702131132-f8e5bc1e16b3
	github.com/pkg/errors v0.9.1
	gopkg.in/ini.v1 v1.62.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.7.0
)

replace github.com/k8snetworkplumbingwg/sriov-network-device-plugin => github.com/openshift/sriov-network-device-plugin v0.0.0-20201204004339-6d9de398bc37 //4.7

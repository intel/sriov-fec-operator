module github.com/otcshare/openshift-operator/sriov-fec

go 1.16

require (
	github.com/jaypipes/ghw v0.8.0
	github.com/k8snetworkplumbingwg/sriov-network-device-plugin v3.0.0+incompatible
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v3.9.0+incompatible
	github.com/otcshare/openshift-operator/common v0.0.0-20220203153345-06a606c2832a // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	gopkg.in/ini.v1 v1.63.2
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
	sigs.k8s.io/controller-runtime v0.10.2
)

replace github.com/k8snetworkplumbingwg/sriov-network-device-plugin => github.com/openshift/sriov-network-device-plugin v0.0.0-20210719073155-2acaea488d32 //4.9

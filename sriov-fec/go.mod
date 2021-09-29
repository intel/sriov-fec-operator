module github.com/smart-edge-open/openshift-operator/sriov-fec

go 1.15

require (
	github.com/jaypipes/ghw v0.8.0
	github.com/k8snetworkplumbingwg/sriov-network-device-plugin v3.0.0+incompatible
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v3.9.0+incompatible
	github.com/smart-edge-open/openshift-operator/common v0.0.0-20210929103615-4a0aab388751
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	gopkg.in/ini.v1 v1.63.2
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)

replace github.com/k8snetworkplumbingwg/sriov-network-device-plugin => github.com/openshift/sriov-network-device-plugin v0.0.0-20210316000337-fed32d92655d //4.8

module kmodules.xyz/monitoring-agent-api

go 1.12

require (
	github.com/appscode/go v0.0.0-20200323182826-54e98e09185a
	github.com/coreos/prometheus-operator v0.39.0
	github.com/go-openapi/spec v0.19.3
	github.com/gogo/protobuf v1.3.1
	k8s.io/api v0.18.3
	k8s.io/apiextensions-apiserver v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	kmodules.xyz/client-go v0.0.0-20200521065424-173e32c78a20
)

replace (
	k8s.io/apimachinery => github.com/kmodules/apimachinery v0.19.0-alpha.0.0.20200520235721-10b58e57a423
	k8s.io/apiserver => github.com/kmodules/apiserver v0.18.4-0.20200521000930-14c5f6df9625
	k8s.io/client-go => k8s.io/client-go v0.18.3
	k8s.io/kubernetes => github.com/kmodules/kubernetes v1.19.0-alpha.0.0.20200521033432-49d3646051ad
)

module github.com/storageos/api-manager

go 1.13

require (
	github.com/darkowlzz/operator-toolkit v0.0.0-20210105215748-5a0df6fa186c
	github.com/go-logr/logr v0.3.0
	github.com/golang/mock v1.4.4
	github.com/google/uuid v1.1.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/storageos/go-api/v2 v2.3.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.15.0
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.7.0
)

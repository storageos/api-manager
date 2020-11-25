module github.com/storageos/api-manager

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/google/uuid v1.1.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0
	github.com/storageos/go-api/v2 v2.3.0
	github.com/stretchr/testify v1.5.1
	go.uber.org/zap v1.10.0
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.4
)

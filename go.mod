module github.com/storageos/api-manager

go 1.16

require (
	github.com/darkowlzz/operator-toolkit v0.0.0-20210417061919-7030a782a07c
	github.com/go-logr/logr v0.3.0
	github.com/golang/mock v1.5.0
	github.com/google/uuid v1.1.2
	github.com/hashicorp/go-multierror v1.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/storageos/go-api/v2 v2.3.1-0.20210420150320-a30cf41359d2
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/otel v0.15.0
	go.uber.org/zap v1.15.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)

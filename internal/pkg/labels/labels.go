package labels

// k8s recommended labels from:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels
const (
	AppName      = "app.kubernetes.io/name"
	AppInstance  = "app.kubernetes.io/instance"
	AppVersion   = "app.kubernetes.io/version"
	AppComponent = "app.kubernetes.io/component"
	AppPartOf    = "app.kubernetes.io/part-of"
	AppManagedBy = "app.kubernetes.io/managed-by"

	DefaultAppName      = "storageos"
	DefaultAppComponent = "storageos-api-manager"
	DefaultAppPartOf    = "storageos"
	DefaultAppManagedBy = "storageos-operator"
)

// Default returns the default labels for resources created by the api-manager.
func Default() map[string]string {
	return map[string]string{
		AppName:      DefaultAppName,
		AppComponent: DefaultAppComponent,
		AppPartOf:    DefaultAppPartOf,
		AppManagedBy: DefaultAppManagedBy,
	}
}

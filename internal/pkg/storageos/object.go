package storageos

import "k8s.io/apimachinery/pkg/types"

// ObjectKey identifies a Kubernetes Object.
// https://github.com/kubernetes-sigs/controller-runtime/blob/74fd294a89c65c8efc17ab92e0d2014d36e357a8/pkg/client/interfaces.go
type ObjectKey = types.NamespacedName

// ObjectKeyFromObject returns the ObjectKey given a runtime.Object.
func ObjectKeyFromObject(obj Object) ObjectKey {
	return ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

// ObjectKeys converts objects to Kubernetes ObjectKeys.
func ObjectKeys(objects []Object) []ObjectKey {
	keys := []ObjectKey{}
	for _, obj := range objects {
		keys = append(keys, ObjectKeyFromObject(obj))
	}
	return keys
}

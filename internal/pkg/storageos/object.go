package storageos

import "k8s.io/apimachinery/pkg/types"

// NamespacedNames converts objects to Kubernetes NamespacedNames.
func NamespacedNames(objects []Object) []types.NamespacedName {
	nn := []types.NamespacedName{}
	for _, obj := range objects {
		nn = append(nn, types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		})
	}
	return nn
}

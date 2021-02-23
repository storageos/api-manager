package cluster

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// ErrNotInCluster is returned if the process is not running within a Kubernetes
// cluster.
var ErrNotInCluster = errors.New("not running within a kubernetes cluster")

// Namespace returns the namespace that the current container is running
// in, if it is running within a Kubernetes cluster.  If not, an error is
// returned.
//
// Code slightly modified from:
// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.8.3/pkg/leaderelection/leader_election.go#L104-L120
func Namespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	_, err := os.Stat(inClusterNamespacePath)
	if os.IsNotExist(err) {
		return "", ErrNotInCluster
	} else if err != nil {
		return "", errors.Wrap(err, "error checking namespace file")
	}

	// Load the namespace file and return its content.
	namespace, err := ioutil.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", errors.Wrap(err, "error reading namespace file")
	}
	return string(namespace), nil
}

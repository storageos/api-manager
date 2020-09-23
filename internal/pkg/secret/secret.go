package secret

import (
	"fmt"
	"io/ioutil"
	"strings"
)

// Read a secret from the given path.  The secret is expected to be mounted into
// the container by Kubernetes.
func Read(path string) (string, error) {
	secretBytes, readErr := ioutil.ReadFile(path)
	if readErr != nil {
		return "", fmt.Errorf("unable to read secret: %s, error: %s", path, readErr)
	}
	val := strings.TrimSpace(string(secretBytes))
	return val, nil
}

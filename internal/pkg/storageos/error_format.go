package storageos

import (
	"fmt"
	"strings"
)

// ListErrors is a basic formatter that outputs the number of errors
// that occurred along with a list of the errors.
func ListErrors(es []error) string {
	if len(es) == 1 {
		return es[0].Error()
	}

	points := make([]string, len(es))
	for i, err := range es {
		points[i] = err.Error()
	}

	return fmt.Sprintf("%d errors occurred: %s", len(es), strings.Join(points, ", "))
}

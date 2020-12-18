package storageos

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
)

const (
	// ReservedLabelPrefix is the string prefix used to identify StorageOS
	// reserved labels.  Most reserved labels will require specific API calls to
	// apply to the StorageOS object.
	ReservedLabelPrefix = "storageos.com/"

	// ReservedLabelComputeOnly is the node label used to indicate that a node
	// should not store data and should instead access it remotely.
	ReservedLabelComputeOnly = ReservedLabelPrefix + "computeonly"
)

var (
	// ErrReservedLabelUnknown can be used to indicate that a label with the
	// reserved prefix was provided, but not supported.
	ErrReservedLabelUnknown = errors.New("invalid reserved label")
)

// NodeLabeller provides access to update node labels.
//go:generate mockgen -destination=mocks/mock_node_labeller.go -package=mocks . NodeLabeller
type NodeLabeller interface {
	EnsureNodeLabels(name string, labels map[string]string) error
}

// IsReservedLabel returns true if the key is a StorageOS reserved label name.
// It does not validate whether the key is valid.
func IsReservedLabel(key string) bool {
	return strings.HasPrefix(key, ReservedLabelPrefix)
}

// EnsureNodeLabels applies a set of labels to a StorageOS node.
//
// Labels prefixed with the StorageOS reserved label indicator
// ("storageos.com/") will need to be processed separately as most have
// individual API endpoints to ensure that they are applied atomically.
//
// Unreserved labels are copied as a blob and are not evaluated.
func (c *Client) EnsureNodeLabels(name string, labels map[string]string) error {
	var errors *multierror.Error
	var unreservedLabels = make(map[string]string)

	for k, v := range labels {
		switch {
		case !IsReservedLabel(k):
			unreservedLabels[k] = v
		case k == ReservedLabelComputeOnly:
			enabled, err := strconv.ParseBool(v)
			if err != nil {
				errors = multierror.Append(errors, err)
				continue
			}
			if err := c.EnsureComputeOnly(name, enabled); err != nil && err != ErrNodeNotFound {
				errors = multierror.Append(errors, err)
			}
		default:
			errors = multierror.Append(errors, ErrReservedLabelUnknown)
		}
	}

	// Apply unreserved labels.
	if err := c.EnsureUnreservedNodeLabels(name, unreservedLabels); err != nil && err != ErrNodeNotFound {
		errors = multierror.Append(errors, err)
	}

	return errors.ErrorOrNil()
}

// EnsureUnreservedNodeLabels applies a set of labels to the StorageOS node if different.
// Existing labels will be overwritten.  The set of labels must not include
// StorageOS reserved labels.
func (c *Client) EnsureUnreservedNodeLabels(name string, labels map[string]string) error {
	funcName := "ensure_unreserved_node_labels"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, GetAPIErrorRootCause(e))
		return e
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	ctx, cancel := context.WithTimeout(c.ctx, DefaultRequestTimeout)
	defer cancel()

	node, err := c.getNodeByName(ctx, name)
	if err != nil {
		return observeErr(err)
	}

	var unreservedLabels = make(map[string]string)
	for k, v := range node.Labels {
		if !IsReservedLabel(k) {
			unreservedLabels[k] = v
		}
	}

	if reflect.DeepEqual(labels, unreservedLabels) {
		return nil
	}

	if _, _, err = c.api.UpdateNode(ctx, node.Id, api.UpdateNodeData{Labels: labels, Version: node.Version}); err != nil {
		return observeErr(err)
	}
	return observeErr(nil)
}

// EnsureComputeOnly ensures that the compute-only behaviour has been applied to
// the StorageOS node.
func (c *Client) EnsureComputeOnly(name string, enabled bool) error {
	funcName := "ensure_compute_only"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, GetAPIErrorRootCause(e))
		return e
	}

	ctx, cancel := context.WithTimeout(c.ctx, DefaultRequestTimeout)
	defer cancel()

	node, err := c.getNodeByName(ctx, name)
	if err != nil {
		return observeErr(err)
	}

	// Default for an unset value is false.
	current := false

	// If label already set, get value.
	for k, v := range node.Labels {
		if k == ReservedLabelComputeOnly {
			current, err = strconv.ParseBool(v)
			if err != nil {
				return err
			}
			break
		}
	}

	// No change required.
	if current == enabled {
		return nil
	}

	// Apply update.
	if _, _, err = c.api.SetComputeOnly(ctx, node.Id, api.SetComputeOnlyNodeData{ComputeOnly: enabled, Version: node.Version}, &api.SetComputeOnlyOpts{}); err != nil {
		return observeErr(err)
	}
	return observeErr(nil)
}

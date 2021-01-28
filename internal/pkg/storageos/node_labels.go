package storageos

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
)

// EnsureNodeLabels applies a set of labels to a StorageOS node.
//
// Labels prefixed with the StorageOS reserved label indicator
// ("storageos.com/") will need to be processed separately as most have
// individual API endpoints to ensure that they are applied atomically.
//
// Unreserved labels are copied as a blob and are not evaluated.
func (c *Client) EnsureNodeLabels(ctx context.Context, name string, labels map[string]string) error {
	var unreservedLabels = make(map[string]string)
	var computeOnly = false
	var err error
	var errs = &multierror.Error{ErrorFormat: ListErrors}

	for k, v := range labels {
		switch {
		case !IsReservedLabel(k):
			unreservedLabels[k] = v
		case k == ReservedLabelNoCache ||
			k == ReservedLabelNoCompress ||
			k == ReservedLabelReplicas:
			errs = multierror.Append(errs, errors.Wrap(ErrReservedLabelInvalid, k))
		case k == ReservedLabelComputeOnly:
			computeOnly, err = strconv.ParseBool(v)
			if err != nil {
				errs = multierror.Append(errs, errors.Wrap(err, k))
			}
		default:
			errs = multierror.Append(errs, errors.Wrap(ErrReservedLabelUnknown, k))
		}
	}

	// Apply reserved labels.  Labels that have been removed or have been
	// changed to an invalid value will get their default re-applied.
	if err := c.EnsureComputeOnly(ctx, name, computeOnly); err != nil && err != ErrNodeNotFound {
		errs = multierror.Append(errs, err)
	}

	// Apply unreserved labels as a blob, removing any that are no longer set.
	if err := c.EnsureUnreservedNodeLabels(ctx, name, unreservedLabels); err != nil && err != ErrNodeNotFound {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

// EnsureUnreservedNodeLabels applies a set of labels to the StorageOS node if different.
// Existing labels will be overwritten.  The set of labels must not include
// StorageOS reserved labels.
func (c *Client) EnsureUnreservedNodeLabels(ctx context.Context, name string, labels map[string]string) error {
	funcName := "ensure_unreserved_node_labels"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	ctx = c.AddToken(ctx)

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

	if _, resp, err := c.api.UpdateNode(ctx, node.Id, api.UpdateNodeData{Labels: labels, Version: node.Version}); err != nil {
		return observeErr(api.MapAPIError(err, resp))
	}
	return observeErr(nil)
}

// EnsureComputeOnly ensures that the compute-only behaviour has been applied to
// the StorageOS node.
func (c *Client) EnsureComputeOnly(ctx context.Context, name string, enabled bool) error {
	funcName := "ensure_compute_only"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

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
	if _, resp, err := c.api.SetComputeOnly(ctx, node.Id, api.SetComputeOnlyNodeData{ComputeOnly: enabled, Version: node.Version}, &api.SetComputeOnlyOpts{}); err != nil {
		return observeErr(api.MapAPIError(err, resp))
	}
	return observeErr(nil)
}

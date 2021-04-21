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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	FailureModeSoft     = "soft"
	FailureModeHard     = "hard"
	FailureModeAlwaysOn = "alwayson"
)

var (
	// ErrBadFailureMode is returned if the failure-mode label value was
	// invalid.
	ErrBadFailureMode = errors.New("failed to parse failure-mode label, must be hard, soft, alwayson or an integer toleration")
)

// EnsureVolumeLabels applies a set of labels to a StorageOS volume.
//
// Labels prefixed with the StorageOS reserved label indicator
// ("storageos.com/") will need to be processed separately as most have
// individual API endpoints to ensure that they are applied atomically.
//
// Unreserved labels are copied as a blob and are not evaluated.
func (c *Client) EnsureVolumeLabels(ctx context.Context, key client.ObjectKey, labels map[string]string) error {
	var unreservedLabels = make(map[string]string)
	var replicas uint64
	var failureMode string
	var err error
	var errs = &multierror.Error{ErrorFormat: ListErrors}

	for k, v := range labels {
		switch {
		case !IsReservedLabel(k):
			unreservedLabels[k] = v
		case k == ReservedLabelComputeOnly:
			// Don't attempt reserved labels that don't apply to volumes.
			errs = multierror.Append(errs, errors.Wrap(ErrReservedLabelInvalid, k))
		case k == ReservedLabelNoCache || k == ReservedLabelNoCompress:
			// Don't attempt reserved labels that can't be modifed after creation.
			errs = multierror.Append(errs, errors.Wrap(ErrReservedLabelFixed, k))
		case k == ReservedLabelFailureMode:
			failureMode = v
		case k == ReservedLabelReplicas:
			replicas, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				errs = multierror.Append(errs, errors.Wrap(err, k))
			}
		default:
			errs = multierror.Append(errs, errors.Wrap(ErrReservedLabelUnknown, k))
		}
	}

	// Apply reserved labels.  Labels that have been removed or have been
	// changed to an invalid value will get their default re-applied.
	if err := c.EnsureReplicas(ctx, key, replicas); err != nil && err != ErrVolumeNotFound {
		errs = multierror.Append(errs, err)
	}
	if err := c.EnsureFailureMode(ctx, key, failureMode); err != nil && err != ErrVolumeNotFound {
		errs = multierror.Append(errs, err)
	}

	// Apply unreserved labels as a blob, removing any that are no longer set.
	if err := c.EnsureUnreservedVolumeLabels(ctx, key, unreservedLabels); err != nil && err != ErrVolumeNotFound {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

// EnsureUnreservedVolumeLabels applies a set of labels to the StorageOS volume
// if different. Existing labels will be overwritten.  Any reserved labels
// will be ignored.
func (c *Client) EnsureUnreservedVolumeLabels(ctx context.Context, key client.ObjectKey, labels map[string]string) error {
	funcName := "ensure_unreserved_volume_labels"
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

	vol, err := c.getVolume(ctx, key)
	if err != nil {
		return observeErr(err)
	}

	// Copy desired labels.  Ignore any reserved labels.
	var applyLabels = make(map[string]string)
	for k, v := range labels {
		if !IsReservedLabel(k) {
			applyLabels[k] = v
		}
	}

	// Re-apply reserved labels (must have the same reserved labels & values as
	// current volume or the update will fail validation).
	for k, v := range vol.Labels {
		if IsReservedLabel(k) {
			applyLabels[k] = v
		}
	}

	// Skip update if both current and desired are empty or nil.  DeepEqual will
	// not match empty with nil, but len treats them the same.
	if len(vol.Labels) == 0 && len(applyLabels) == 0 {
		return observeErr(nil)
	}

	// Skip update if unchanged.  Empty labels are valid and should be applied.
	if reflect.DeepEqual(vol.Labels, applyLabels) {
		return observeErr(nil)
	}

	if _, resp, err := c.api.UpdateVolume(ctx, vol.NamespaceID, vol.Id, api.UpdateVolumeData{Labels: applyLabels, Version: vol.Version}, nil); err != nil {
		return observeErr(api.MapAPIError(err, resp))
	}
	return observeErr(nil)
}

// EnsureReplicas ensures that the desired number of replicas has been applied
// to the StorageOS volume.
func (c *Client) EnsureReplicas(ctx context.Context, key client.ObjectKey, desired uint64) error {
	funcName := "ensure_replicas"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	vol, err := c.getVolume(ctx, key)
	if err != nil {
		return observeErr(err)
	}

	var current uint64

	// If label already set, get value.
	for k, v := range vol.Labels {
		if k == ReservedLabelReplicas {
			current, err = strconv.ParseUint(v, 10, 64)
			if err != nil {
				return err
			}
			break
		}
	}

	// No change required.
	if current == desired {
		return nil
	}

	// Apply update.
	if _, resp, err := c.api.SetReplicas(ctx, vol.NamespaceID, vol.Id, api.SetReplicasRequest{Replicas: desired, Version: vol.Version}, nil); err != nil {
		return observeErr(api.MapAPIError(err, resp))
	}
	return observeErr(nil)
}

// EnsureFailureMode ensures that the desired failure mode has been applied to
// the StorageOS volume.
func (c *Client) EnsureFailureMode(ctx context.Context, key client.ObjectKey, desired string) error {
	funcName := "ensure_failure_mode"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	vol, err := c.getVolume(ctx, key)
	if err != nil {
		return observeErr(err)
	}

	// If label already set, get value.
	current := vol.Labels[ReservedLabelFailureMode]

	// No change required.
	if current == desired {
		return nil
	}

	// Parse the label value to determine the failure mode intent or threshold.
	mode, threshold, err := ParseFailureMode(desired)
	if err != nil {
		return observeErr(err)
	}

	// Apply update.
	if _, resp, err := c.api.SetFailureMode(ctx, vol.NamespaceID, vol.Id, api.SetFailureModeRequest{Mode: mode, FailureThreshold: threshold, Version: vol.Version}, nil); err != nil {
		return observeErr(api.MapAPIError(err, resp))
	}
	return observeErr(nil)
}

// ParseFailureMode parses a string and returns either a failure mode intent or
// a threshold, if set.  Only one of the intent or threshold should be set.
func ParseFailureMode(s string) (api.FailureModeIntent, uint64, error) {
	// Check for "canned" failure mode intent.
	switch s {
	case FailureModeSoft:
		return api.FAILUREMODEINTENT_SOFT, 0, nil
	case FailureModeHard:
		return api.FAILUREMODEINTENT_HARD, 0, nil
	case FailureModeAlwaysOn:
		return api.FAILUREMODEINTENT_ALWAYSON, 0, nil
	case "":
		//  Return defaults if no value set.
		return api.FAILUREMODEINTENT_HARD, 0, nil
	}

	// Otherwise look for an integer value for the failure threshold.
	threshold, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return "", 0, errors.Wrap(err, ErrBadFailureMode.Error())
	}
	return "", threshold, nil
}

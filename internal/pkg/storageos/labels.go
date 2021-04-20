package storageos

import (
	"errors"
	"strings"
)

const (
	// ReservedLabelPrefix is the string prefix used to identify StorageOS
	// reserved labels.  Most reserved labels will require specific API calls to
	// apply to the StorageOS object.
	ReservedLabelPrefix = "storageos.com/"

	// ReservedLabelComputeOnly is the node label used to indicate that a node
	// should not store data and should instead access it remotely.
	ReservedLabelComputeOnly = ReservedLabelPrefix + "computeonly"

	// ReservedLabelReplicas is the PVC label used to indicate the desired
	// number of volume replicas.
	ReservedLabelReplicas = ReservedLabelPrefix + "replicas"

	// ReservedLabelNoCache is the PVC label used to disable the per-volume
	// cache.  It can only be set when the volume is created.
	ReservedLabelNoCache = ReservedLabelPrefix + "nocache"

	// ReservedLabelNoCompress is the PVC label used to disable compression on a
	// volume. It can only be set when the volume is created.
	ReservedLabelNoCompress = ReservedLabelPrefix + "nocompress"

	// ReservedLabelFailureMode is the PVC label used to set the behaviour when
	// there are fewer copies of the data available than what was requested.
	ReservedLabelFailureMode = ReservedLabelPrefix + "failure-mode"

	// ReservedLabelFencing can be set on Pods to indicate that the Pod should
	// be deleted if it is running on a node that StorageOS believes no longer
	// has access to its storage.
	ReservedLabelFencing = ReservedLabelPrefix + "fenced"

	// ReservedLabelK8sPVCNamespace is set by the csi-provisioner at create
	// time.  It's treated as a reserved label by StorageOS and can't be modified.
	ReservedLabelK8sPVCNamespace = "csi.storage.k8s.io/pvc/namespace"

	// ReservedLabelK8sPVCName is set by the csi-provisioner at create time.
	// It's treated as a reserved label by StorageOS and can't be modified.
	ReservedLabelK8sPVCName = "csi.storage.k8s.io/pvc/name"

	// ReservedLabelK8sPVName is set by the csi-provisioner at create time. It's
	// treated as a reserved label by StorageOS and can't be modified.
	ReservedLabelK8sPVName = "csi.storage.k8s.io/pv/name"
)

var (
	// ErrReservedLabelUnknown indicates that a label with the reserved prefix
	// was provided, but not recognized.
	ErrReservedLabelUnknown = errors.New("unrecognized reserved label")

	// ErrReservedLabelInvalid indicates that a label with the reserved prefix
	// was recognized, but not supported for the obejct type.
	ErrReservedLabelInvalid = errors.New("invalid reserved label for this object type")

	// ErrReservedLabelFixed can be used to indicate that a label can't be
	// modified once set during object creation.
	ErrReservedLabelFixed = errors.New("behaviour can't be changed after creation")
)

// IsReservedLabel returns true if the key is a StorageOS reserved label name.
// It does not validate whether the key is valid.
func IsReservedLabel(key string) bool {
	if strings.HasPrefix(key, ReservedLabelPrefix) {
		return true
	}
	// The CSI external-provisioner adds these labels when
	// `--extra-create-metadata` is set.  The control plane relies on them for
	// Namespace autocreation and Pod scheduling.  Once set, they can't be
	// changed.
	if key == ReservedLabelK8sPVCNamespace || key == ReservedLabelK8sPVCName || key == ReservedLabelK8sPVName {
		return true
	}
	return false
}

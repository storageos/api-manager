package storageos

import (
	"github.com/storageos/api-manager/internal/pkg/endpoint"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// NFSPort is the port exposed by the service.  Each service will have a
	// unique ip, so it can be fixed to the default NFS port.
	NFSPort int32 = 2049
	// NFSPortName is used as the name of the NFS port in the service defintion.
	NFSPortName = "nfs"
	// NFSProtocol is the prtocol to be used for NFS.
	NFSProtocol = "TCP"

	// SharedVolumeLabelName is the label to set on created resources that will
	// identify them as being managed by this controller.  The value could be
	// anything - using volume ID to assist with debugging.
	SharedVolumeLabelName = "storageos.com/sharedvolume"

	// LabelNFSMountEndpoint is the nfs attachment's mount endpoint, if any.
	LabelNFSMountEndpoint = "storageos.com/nfs/mount-endpoint"

	// LabelPVCName holds the name of the corresponding PVC.
	LabelPVCName = "csi.storage.k8s.io/pvc/name"

	// LabelPVCNamespace holds the namespace of the corresponding PVC.  It
	// should always be the same as the volume namespace.
	LabelPVCNamespace = "csi.storage.k8s.io/pvc/namespace"
)

// SharedVolumeList is a collection of SharedVolumes.
type SharedVolumeList []*SharedVolume

// SharedVolume represents a single StorageOS shared volume.
type SharedVolume struct {
	ID               string
	Name             string
	Namespace        string
	InternalEndpoint string
	ExternalEndpoint string
}

// NewSharedVolume returns a sharedvolume object.
func NewSharedVolume(id, name, namespace, intEndpoint, extEndpoint string) *SharedVolume {
	return &SharedVolume{
		ID:               id,
		Name:             name,
		Namespace:        namespace,
		InternalEndpoint: intEndpoint,
		ExternalEndpoint: extEndpoint,
	}
}

// IsEqual returns true if the given SharedVolume object is equivalent.
func (v *SharedVolume) IsEqual(obj *SharedVolume) bool {
	if obj == nil ||
		obj.Name != v.Name ||
		obj.Namespace != v.Namespace ||
		obj.InternalEndpoint != v.InternalEndpoint ||
		obj.ExternalEndpoint != v.ExternalEndpoint {
		return false
	}
	return true
}

// InternalAddress returns the address of the intenral SharedVolume listener.
func (v *SharedVolume) InternalAddress() string {
	address, _, err := endpoint.SplitAddressPort(v.InternalEndpoint)
	if err != nil {
		return ""
	}
	return address
}

// InternalPort returns the port of the intenral SharedVolume listener.
func (v *SharedVolume) InternalPort() int {
	_, port, err := endpoint.SplitAddressPort(v.InternalEndpoint)
	if err != nil {
		return 0
	}
	return port
}

// Service returns the desired service corresponding to the SharedVolume.
// ClusterIP can be provided if an existing ClusterIP should be re-used.
// The ownerRef must be set to the volume's PersistentVolumeClaim.
func (v *SharedVolume) Service(ownerRef metav1.OwnerReference) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v.Name,
			Namespace: v.Namespace,
			Labels: map[string]string{
				SharedVolumeLabelName: v.ID,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       NFSPortName,
					Port:       NFSPort,
					Protocol:   NFSProtocol,
					TargetPort: intstr.FromInt(v.InternalPort()),
				},
			},
		},
	}
}

// ServiceIsEqual returns true if the service provided matches the desired state
// of the SharedVolume.
func (v *SharedVolume) ServiceIsEqual(svc *corev1.Service) bool {
	if svc == nil ||
		svc.Name != v.Name ||
		svc.Namespace != v.Namespace ||
		svc.Spec.Type != corev1.ServiceTypeClusterIP {
		return false
	}
	if len(svc.Spec.Ports) != 1 ||
		svc.Spec.Ports[0].Name != NFSPortName ||
		svc.Spec.Ports[0].Port != NFSPort ||
		svc.Spec.Ports[0].Protocol != NFSProtocol ||
		svc.Spec.Ports[0].TargetPort.IntVal != int32(v.InternalPort()) {
		return false
	}
	return true
}

// ServiceUpdate returns the provided service, with updates to match the
// SharedVolume.
func (v *SharedVolume) ServiceUpdate(svc *corev1.Service) *corev1.Service {
	if len(svc.Spec.Ports) != 1 {
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       NFSPortName,
				Port:       NFSPort,
				Protocol:   NFSProtocol,
				TargetPort: intstr.FromInt(v.InternalPort()),
			},
		}
		return svc
	}
	svc.Spec.Ports[0].TargetPort = intstr.FromInt(v.InternalPort())
	return svc
}

// Endpoints returns the desired endpoints corresponding to the SharedVolume.
func (v *SharedVolume) Endpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v.Name,
			Namespace: v.Namespace,
			Labels: map[string]string{
				SharedVolumeLabelName: v.ID,
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: v.InternalAddress(),
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Name:     NFSPortName,
						Port:     int32(v.InternalPort()),
						Protocol: NFSProtocol,
					},
				},
			},
		},
	}
}

// EndpointsIsEqual returns true if the endpoints provided matches the desired
// state of the SharedVolume.
func (v *SharedVolume) EndpointsIsEqual(e *corev1.Endpoints) bool {
	if e == nil ||
		e.Name != v.Name ||
		e.Namespace != v.Namespace {
		return false
	}
	if len(e.Subsets) != 1 {
		return false
	}
	if len(e.Subsets[0].Addresses) == 0 ||
		e.Subsets[0].Addresses[0].IP != v.InternalAddress() {
		return false
	}
	if len(e.Subsets[0].Ports) == 0 ||
		e.Subsets[0].Ports[0].Name != NFSPortName ||
		e.Subsets[0].Ports[0].Port != int32(v.InternalPort()) ||
		e.Subsets[0].Ports[0].Protocol != NFSProtocol {
		return false
	}
	return true
}

// EndpointsUpdate returns the provided endpoints, with updates to match the
// SharedVolume.
func (v *SharedVolume) EndpointsUpdate(e *corev1.Endpoints) *corev1.Endpoints {
	if len(e.Subsets) != 1 || len(e.Subsets[0].Addresses) != 1 || len(e.Subsets[0].Ports) != 1 {
		e.Subsets = []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: v.InternalAddress(),
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Name:     NFSPortName,
						Port:     int32(v.InternalPort()),
						Protocol: NFSProtocol,
					},
				},
			},
		}
		return e
	}
	e.Subsets[0].Addresses[0].IP = v.InternalAddress()
	e.Subsets[0].Ports[0].Port = int32(v.InternalPort())
	return e
}

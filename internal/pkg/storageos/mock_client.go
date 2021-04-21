package storageos

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storageosv1 "github.com/storageos/api-manager/api/v1"
)

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

// MockObject can be be used to replace an api object.
type MockObject struct {
	ID        string
	Name      string
	Namespace string
	Labels    map[string]string
	Healthy   bool
}

// GetID returns the object ID.
func (m MockObject) GetID() string {
	return m.ID
}

// GetName returns the object name.
func (m MockObject) GetName() string {
	return m.Name
}

// GetNamespace returns the object namespace.
func (m MockObject) GetNamespace() string {
	return m.Namespace
}

// GetLabels returns the object labels.
func (m MockObject) GetLabels() map[string]string {
	return m.Labels
}

// IsHealthy returns true if the object is healthy.
func (m MockObject) IsHealthy() bool {
	return m.Healthy
}

// MockClient provides a test interface to the StorageOS api.
type MockClient struct {
	sharedvols               map[string]*SharedVolume
	namespaces               map[client.ObjectKey]Object
	nodes                    map[client.ObjectKey]Object
	volumes                  map[client.ObjectKey]Object
	nodeLabels               map[string]string
	mu                       sync.RWMutex
	DeleteNamespaceCallCount map[client.ObjectKey]int
	DeleteNodeCallCount      map[client.ObjectKey]int
	ListNamespacesErr        error
	DeleteNamespaceErr       error
	GetNodeErr               error
	NodeObjectsErr           error
	ListNodesErr             error
	DeleteNodeErr            error
	EnsureNodeLabelsErr      error
	GetNodeLabelsErr         error
	GetVolumeErr             error
	VolumeObjectsErr         error
	EnsureVolumeLabelsErr    error
	SharedVolsErr            error
	SharedVolErr             error
	SetEndpointErr           error
}

// NewMockClient returns an initialized MockClient.
func NewMockClient() *MockClient {
	return &MockClient{
		sharedvols:               make(map[string]*SharedVolume),
		namespaces:               make(map[client.ObjectKey]Object),
		nodes:                    make(map[client.ObjectKey]Object),
		volumes:                  make(map[client.ObjectKey]Object),
		nodeLabels:               make(map[string]string),
		DeleteNamespaceCallCount: make(map[client.ObjectKey]int),
		DeleteNodeCallCount:      make(map[client.ObjectKey]int),
		mu:                       sync.RWMutex{},
	}
}

// ListNamespaces returns a list of StorageOS namespace objects.
func (c *MockClient) ListNamespaces(ctx context.Context) ([]Object, error) {
	if c.ListNamespacesErr != nil {
		return nil, c.ListNamespacesErr
	}
	ret := []Object{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, ns := range c.namespaces {
		ret = append(ret, ns)
	}
	return ret, nil
}

// AddNamespace adds a namespace to the StorageOS cluster.
func (c *MockClient) AddNamespace(key client.ObjectKey) error {
	c.mu.Lock()
	c.namespaces[key] = MockObject{Name: key.Name}
	c.mu.Unlock()
	return nil
}

// NamespaceExists returns true if the namespace exists.
func (c *MockClient) NamespaceExists(key client.ObjectKey) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.namespaces[key]; ok {
		return true
	}
	return false
}

// DeleteNamespace removes a namespace from the StorageOS cluster.
func (c *MockClient) DeleteNamespace(ctx context.Context, key client.ObjectKey) error {
	c.DeleteNamespaceCallCount[key]++
	if c.DeleteNamespaceErr != nil {
		return c.DeleteNamespaceErr
	}
	if !c.NamespaceExists(key) {
		return ErrNamespaceNotFound
	}
	c.mu.Lock()
	delete(c.namespaces, key)
	c.mu.Unlock()
	return nil
}

// NodeObjects returns a map of nodes objects, indexed on object key.
func (c *MockClient) NodeObjects(ctx context.Context) (map[client.ObjectKey]Object, error) {
	if c.NodeObjectsErr != nil {
		return nil, c.NodeObjectsErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodes, nil
}

// ListNodes returns a list of StorageOS node objects.
func (c *MockClient) ListNodes(ctx context.Context) ([]client.Object, error) {
	if c.ListNodesErr != nil {
		return nil, c.ListNodesErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	ret := []client.Object{}
	for _, node := range c.nodes {
		health := storageosv1.NodeHealthOnline
		if !node.IsHealthy() {
			health = storageosv1.NodeHealthOffline
		}
		ret = append(ret, &storageosv1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: node.GetName()},
			Status: storageosv1.NodeStatus{
				Health: health,
			},
		})
	}
	return ret, nil
}

// AddNode adds a node to the StorageOS cluster.
func (c *MockClient) AddNode(obj Object) error {
	c.mu.Lock()
	c.nodes[ObjectKeyFromObject(obj)] = obj
	c.mu.Unlock()
	return nil
}

// NodeExists returns true if the node exists.
func (c *MockClient) NodeExists(key client.ObjectKey) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.nodes[key]; ok {
		return true
	}
	return false
}

// DeleteNode removes a node from the StorageOS cluster.
func (c *MockClient) DeleteNode(ctx context.Context, key client.ObjectKey) error {
	c.DeleteNodeCallCount[key]++
	if c.DeleteNodeErr != nil {
		return c.DeleteNodeErr
	}
	if !c.NodeExists(key) {
		return ErrNodeNotFound
	}
	c.mu.Lock()
	delete(c.nodes, key)
	c.mu.Unlock()
	return nil
}

// EnsureNodeLabels applies a set of labels to the StorageOS node.
func (c *MockClient) EnsureNodeLabels(ctx context.Context, key client.ObjectKey, labels map[string]string) error {
	if c.EnsureNodeLabelsErr != nil {
		return c.EnsureNodeLabelsErr
	}

	var errors *multierror.Error
	var newLabels = make(map[string]string)

	for k, v := range labels {
		switch {
		case !IsReservedLabel(k):
			newLabels[k] = v
		case k == ReservedLabelComputeOnly:
			newLabels[k] = v
		default:
			errors = multierror.Append(errors, ErrReservedLabelUnknown)
		}
	}

	c.mu.Lock()
	c.nodeLabels = newLabels
	c.mu.Unlock()
	return errors.ErrorOrNil()
}

// GetNodeLabels retrieves the set of labels.
func (c *MockClient) GetNodeLabels(key client.ObjectKey) (map[string]string, error) {
	if c.GetNodeLabelsErr != nil {
		return nil, c.GetNodeLabelsErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeLabels, nil
}

// AddVolume adds a volume to the StorageOS cluster.
func (c *MockClient) AddVolume(obj Object) error {
	c.mu.Lock()
	c.volumes[ObjectKeyFromObject(obj)] = obj
	c.mu.Unlock()
	return nil
}

// UpdateNodeHealth sets the node health.
func (c *MockClient) UpdateNodeHealth(key client.ObjectKey, healthy bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.nodes[key]
	if !ok {
		return false
	}
	c.nodes[key] = MockObject{
		ID:        n.GetID(),
		Name:      n.GetName(),
		Namespace: n.GetNamespace(),
		Labels:    n.GetLabels(),
		Healthy:   healthy,
	}
	return true
}

// UpdateVolumeHealth sets the volume health.
func (c *MockClient) UpdateVolumeHealth(key client.ObjectKey, healthy bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.volumes[key]
	if !ok {
		return false
	}
	c.volumes[key] = MockObject{
		ID:        n.GetID(),
		Name:      n.GetName(),
		Namespace: n.GetNamespace(),
		Labels:    n.GetLabels(),
		Healthy:   healthy,
	}
	return true
}

// GetVolume retrieves a volume object.
func (c *MockClient) GetVolume(ctx context.Context, key client.ObjectKey) (Object, error) {
	if c.GetVolumeErr != nil {
		return nil, c.GetVolumeErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	obj, ok := c.volumes[key]
	if !ok {
		return nil, ErrVolumeNotFound
	}
	return obj, nil
}

// VolumeObjects returns a map of volume objects, indexed on object key.
func (c *MockClient) VolumeObjects(ctx context.Context) (map[client.ObjectKey]Object, error) {
	if c.ListNodesErr != nil {
		return nil, c.ListNodesErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.volumes, nil
}

// EnsureVolumeLabels applies a set of labels to the StorageOS volume.
func (c *MockClient) EnsureVolumeLabels(ctx context.Context, key client.ObjectKey, labels map[string]string) error {
	if c.EnsureVolumeLabelsErr != nil {
		return c.EnsureVolumeLabelsErr
	}

	var errors *multierror.Error
	var newLabels = make(map[string]string)

	for k, v := range labels {
		switch {
		case !IsReservedLabel(k):
			newLabels[k] = v
		case k == ReservedLabelReplicas || k == ReservedLabelFailureMode:
			newLabels[k] = v
		default:
			errors = multierror.Append(errors, ErrReservedLabelUnknown)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	n, ok := c.volumes[key]
	if !ok {
		return ErrVolumeNotFound
	}
	c.volumes[key] = &MockObject{
		ID:        n.GetID(),
		Name:      n.GetName(),
		Namespace: n.GetNamespace(),
		Labels:    newLabels,
	}
	return errors.ErrorOrNil()
}

// ListSharedVolumes returns a list of active shared volumes.
func (c *MockClient) ListSharedVolumes(ctx context.Context) (SharedVolumeList, error) {
	if c.SharedVolsErr != nil {
		return nil, c.SharedVolsErr
	}
	c.mu.RLock()
	list := SharedVolumeList{}
	for _, v := range c.sharedvols {
		list = append(list, v)
	}
	c.mu.RUnlock()
	return list, c.SharedVolsErr
}

// SetExternalEndpoint sets the external endpoint on a SharedVolume.  The
// endpoint should be <host|ip>:<port>.
func (c *MockClient) SetExternalEndpoint(ctx context.Context, id string, namespace string, endpoint string) error {
	if c.SetEndpointErr != nil {
		return c.SetEndpointErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.sharedvols[strings.Join([]string{namespace, id}, "/")]; !ok {
		return ErrNotFound
	}
	c.sharedvols[strings.Join([]string{namespace, id}, "/")].ExternalEndpoint = endpoint
	return nil
}

// Get returns a SharedVolume.
func (c *MockClient) Get(id string, namespace string) (*SharedVolume, error) {
	if c.SharedVolErr != nil {
		return nil, c.SharedVolErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.sharedvols[strings.Join([]string{namespace, id}, "/")]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

// Set adds or replaces a shared volume, and also returns it.
func (c *MockClient) Set(v *SharedVolume) *SharedVolume {
	c.mu.Lock()
	c.sharedvols[strings.Join([]string{v.Namespace, v.ID}, "/")] = v
	c.mu.Unlock()
	return v
}

// Delete a shared volume.
func (c *MockClient) Delete(id string, namespace string) {
	c.mu.Lock()
	delete(c.sharedvols, strings.Join([]string{namespace, id}, "/"))
	c.mu.Unlock()
}

// Reset the shared volume list.
func (c *MockClient) Reset() {
	c.mu.Lock()
	c.sharedvols = make(map[string]*SharedVolume)
	c.namespaces = make(map[client.ObjectKey]Object)
	c.nodes = make(map[client.ObjectKey]Object)
	c.nodeLabels = make(map[string]string)
	c.DeleteNamespaceCallCount = make(map[client.ObjectKey]int)
	c.DeleteNodeCallCount = make(map[client.ObjectKey]int)
	c.ListNamespacesErr = nil
	c.DeleteNamespaceErr = nil
	c.ListNodesErr = nil
	c.DeleteNodeErr = nil
	c.EnsureNodeLabelsErr = nil
	c.GetNodeLabelsErr = nil
	c.SharedVolErr = nil
	c.SharedVolsErr = nil
	c.SetEndpointErr = nil
	c.mu.Unlock()
}

// RandomVol returns a randomly generated shared volume.  Always uses default
// namespace since it will always exist.
func (c *MockClient) RandomVol() *SharedVolume {
	return &SharedVolume{
		ID:               randomString(32),
		ServiceName:      "pvc-" + uuid.New().String(),
		PVCName:          randomString(8),
		Namespace:        "default",
		InternalEndpoint: fmt.Sprintf("%d.%d.%d.%d:%d", rand.Intn(253)+1, rand.Intn(253)+1, rand.Intn(253)+1, rand.Intn(253)+1, rand.Intn(65534)+1),
	}
}

func randomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

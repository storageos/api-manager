package storageos

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

// MockClient provides a test interface to the StorageOS api.
type MockClient struct {
	vols                     map[string]*SharedVolume
	namespaces               map[string]struct{}
	nodes                    map[string]struct{}
	nodeLabels               map[string]string
	mu                       sync.RWMutex
	DeleteNamespaceCallCount map[string]int
	DeleteNodeCallCount      map[string]int
	ListNamespacesErr        error
	DeleteNamespaceErr       error
	ListNodesErr             error
	DeleteNodeErr            error
	EnsureNodeLabelsErr      error
	GetNodeLabelsErr         error
	SharedVolsErr            error
	SharedVolErr             error
	SetEndpointErr           error
}

// NewMockClient returns an initialized MockClient.
func NewMockClient() *MockClient {
	return &MockClient{
		vols:                     make(map[string]*SharedVolume),
		namespaces:               make(map[string]struct{}),
		nodes:                    make(map[string]struct{}),
		nodeLabels:               make(map[string]string),
		DeleteNamespaceCallCount: make(map[string]int),
		DeleteNodeCallCount:      make(map[string]int),
		mu:                       sync.RWMutex{},
	}
}

// ListNamespaces returns a list of StorageOS namespaces as NamespacedNames.
func (c *MockClient) ListNamespaces() ([]types.NamespacedName, error) {
	if c.ListNamespacesErr != nil {
		return nil, c.ListNamespacesErr
	}
	nn := []types.NamespacedName{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name := range c.namespaces {
		nn = append(nn, types.NamespacedName{Name: name})
	}
	return nn, nil
}

// AddNamespace adds a namespace to the StorageOS cluster.
func (c *MockClient) AddNamespace(name string) error {
	c.mu.Lock()
	c.namespaces[name] = struct{}{}
	c.mu.Unlock()
	return nil
}

// NamespaceExists returns true if the naemspace exists.
func (c *MockClient) NamespaceExists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.namespaces[name]; ok {
		return true
	}
	return false
}

// DeleteNamespace removes a namespace from the StorageOS cluster.
func (c *MockClient) DeleteNamespace(name string) error {
	c.DeleteNamespaceCallCount[name]++
	if c.DeleteNamespaceErr != nil {
		return c.DeleteNamespaceErr
	}
	if !c.NamespaceExists(name) {
		return ErrNamespaceNotFound
	}
	c.mu.Lock()
	delete(c.namespaces, name)
	c.mu.Unlock()
	return nil
}

// ListNodes returns a list of StorageOS nodes as NamespacedNames.
func (c *MockClient) ListNodes() ([]types.NamespacedName, error) {
	if c.ListNodesErr != nil {
		return nil, c.ListNodesErr
	}
	nn := []types.NamespacedName{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name := range c.nodes {
		nn = append(nn, types.NamespacedName{Name: name})
	}
	return nn, nil
}

// AddNode adds a node to the StorageOS cluster.
func (c *MockClient) AddNode(name string) error {
	c.mu.Lock()
	c.nodes[name] = struct{}{}
	c.mu.Unlock()
	return nil
}

// NodeExists returns true if the node exists.
func (c *MockClient) NodeExists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.nodes[name]; ok {
		return true
	}
	return false
}

// DeleteNode removes a node from the StorageOS cluster.
func (c *MockClient) DeleteNode(name string) error {
	c.DeleteNodeCallCount[name]++
	if c.DeleteNodeErr != nil {
		return c.DeleteNodeErr
	}
	if !c.NodeExists(name) {
		return ErrNodeNotFound
	}
	c.mu.Lock()
	delete(c.nodes, name)
	c.mu.Unlock()
	return nil
}

// EnsureNodeLabels applies a set of labels to the StorageOS node.
func (c *MockClient) EnsureNodeLabels(name string, labels map[string]string) error {
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
func (c *MockClient) GetNodeLabels(name string) (map[string]string, error) {
	if c.GetNodeLabelsErr != nil {
		return nil, c.GetNodeLabelsErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeLabels, nil
}

// ListSharedVolumes returns a list of active shared volumes.
func (c *MockClient) ListSharedVolumes() (SharedVolumeList, error) {
	if c.SharedVolsErr != nil {
		return nil, c.SharedVolsErr
	}
	c.mu.RLock()
	list := SharedVolumeList{}
	for _, v := range c.vols {
		list = append(list, v)
	}
	c.mu.RUnlock()
	return list, c.SharedVolsErr
}

// SetExternalEndpoint sets the external endpoint on a SharedVolume.  The
// endpoint should be <host|ip>:<port>.
func (c *MockClient) SetExternalEndpoint(id string, namespace string, endpoint string) error {
	if c.SetEndpointErr != nil {
		return c.SetEndpointErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.vols[strings.Join([]string{namespace, id}, "/")]; !ok {
		return ErrNotFound
	}
	c.vols[strings.Join([]string{namespace, id}, "/")].ExternalEndpoint = endpoint
	return nil
}

// Get returns a SharedVolume.
func (c *MockClient) Get(id string, namespace string) (*SharedVolume, error) {
	if c.SharedVolErr != nil {
		return nil, c.SharedVolErr
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.vols[strings.Join([]string{namespace, id}, "/")]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

// Set adds or replaces a shared volume, and also returns it.
func (c *MockClient) Set(v *SharedVolume) *SharedVolume {
	c.mu.Lock()
	c.vols[strings.Join([]string{v.Namespace, v.ID}, "/")] = v
	c.mu.Unlock()
	return v
}

// Delete a shared volume.
func (c *MockClient) Delete(id string, namespace string) {
	c.mu.Lock()
	delete(c.vols, strings.Join([]string{namespace, id}, "/"))
	c.mu.Unlock()
}

// Reset the shared volume list.
func (c *MockClient) Reset() {
	c.mu.Lock()
	c.vols = make(map[string]*SharedVolume)
	c.namespaces = make(map[string]struct{})
	c.nodes = make(map[string]struct{})
	c.nodeLabels = make(map[string]string)
	c.DeleteNamespaceCallCount = make(map[string]int)
	c.DeleteNodeCallCount = make(map[string]int)
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

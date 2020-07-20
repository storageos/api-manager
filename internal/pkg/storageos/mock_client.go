package storageos

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

// MockClient provides a test interface to the StorageOS api.
type MockClient struct {
	vols           map[string]*SharedVolume
	mu             sync.RWMutex
	SharedVolsErr  error
	SharedVolErr   error
	SetEndpointErr error
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
	c.mu.RLock()
	defer c.mu.RUnlock()

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
		Name:             randomString(8),
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

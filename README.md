# StorageOS API Controller

<Description>

# Setup/Development

Ensure a k8s cluster is running and ~/.kube/config contains the kubeconfig, or
pass the path to kubeconfig to the manager binary with -kubeconfig flag.

```console
# Build the binary.
$ make manager

# Update the manifests (Optional).
$ make manifest

# Install the CRD.
$ make install

# Run the manager locally.
$ ./bin/manager
2020-07-12T18:42:02.147Z	INFO	controller-runtime.metrics	metrics server is starting to listen	{"addr": ":8080"}
2020-07-12T18:42:02.148Z	INFO	setup	starting manager
2020-07-12T18:42:02.148Z	INFO	controller-runtime.manager	starting metrics server	{"path": "/metrics"}
2020-07-12T18:42:02.149Z	INFO	controller-runtime.controller	Starting EventSource	{"controller": "sharedvolumesinfo", "source": "kind source: /, Kind="}
2020-07-12T18:42:02.250Z	INFO	controller-runtime.controller	Starting EventSource	{"controller": "sharedvolumesinfo", "source": "channel source: 0xc00009b860"}
2020-07-12T18:42:02.252Z	INFO	controller-runtime.controller	Starting Controller	{"controller": "sharedvolumesinfo"}
2020-07-12T18:42:02.252Z	INFO	controller-runtime.controller	Starting workers	{"controller": "sharedvolumesinfo", "worker count": 1}
2020-07-12T18:42:02.252Z	DEBUG	controllers.SharedVolumesInfo	event dispatched
2020-07-12T18:42:02.253Z	INFO	controllers.SharedVolumesInfo	Successfully handled generic event	{"sharedvolumesinfo": {"metadata":{"name":"stos-shared-vols-info","creationTimestamp":null},"spec":{"volumes":[{"name":"vol1","namespace":"xyz-namespace","address":"1.2.3.4:9999"},{"name":"vol2","namespace":"abc-namespace","address":"9.8.7.6:1111"}]},"status":{}}}
2020-07-12T18:42:07.253Z	DEBUG	controllers.SharedVolumesInfo	event dispatched
2020-07-12T18:42:07.253Z	INFO	controllers.SharedVolumesInfo	Successfully handled generic event	{"sharedvolumesinfo": {"metadata":{"name":"stos-shared-vols-info","creationTimestamp":null},"spec":{"volumes":[{"name":"vol1","namespace":"xyz-namespace","address":"1.2.3.4:9999"},{"name":"vol2","namespace":"abc-namespace","address":"9.8.7.6:1111"}]},"status":{}}}
```

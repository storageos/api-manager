/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	internalv1 "github.com/storageos/api-controller/api/v1"
)

const (
	pollRate = 5 * time.Second
)

// SharedVolumesInfoReconciler reconciles a SharedVolumesInfo object.
type SharedVolumesInfoReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	// StOSClient <type>
	// SharedVolumesInfoCache <type>
}

// +kubebuilder:rbac:groups=internal.storageos.com,resources=sharedvolumesinfoes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=internal.storageos.com,resources=sharedvolumesinfoes/status,verbs=get;update;patch

func (r *SharedVolumesInfoReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("sharedvolumesinfo", req.NamespacedName)

	// Reconcile no-op.

	return ctrl.Result{}, nil
}

// genericEventHandler implements EventHandler.Generic() to handle events
// originating outside the cluster. Since there's no cluster resource that's
// being reconciled in this controller, it skips passing event request to the
// workqueue.
func (r *SharedVolumesInfoReconciler) genericEventHandler(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	_ = context.Background()
	log := r.Log.WithValues("sharedvolumesinfo", evt.Object)

	volsInfo, ok := evt.Object.(*internalv1.SharedVolumesInfo)
	if !ok {
		log.Error(fmt.Errorf("object conversion failed"), "failed to convert to SharedVolumesInfo", "evt.Object", evt.Object)
		return
	}

	_ = volsInfo.Spec

	// TODO: k8s resource creation/update/deletion.
	// 1. Create a list of all the shared volumes.
	// 2. Create an empty list to be populated with the newly created and
	// existing services.
	// 3. [Creation] Iterate through the list of shared volumes and create or
	// update associated services with a common label. This label will be used
	// to list all the shared volume services. Append the list of services
	// created above with the created/updated service name.
	// 4. [Cleanup] List all the services with the common label, diff all
	// services list and created/updated services and delete the services that
	// no longer have associated shared volumes.
	// 5. Do the above concurrently using channels.

	log.V(0).Info("Successfully handled generic event")

	// Do not add new request to workqueue because reconciliation is not
	// required.
}

// sourceEventDispatcher polls external URL, converts the received data into a
// GenericEvent with a runtime.Object.
func (r *SharedVolumesInfoReconciler) sourceEventDispatcher(src chan<- event.GenericEvent) {
	for {
		// Query shared volumes info.
		// r.StOSClient.Getxxxxx()

		// Create a SharedVolumesInfo object from the polled data.
		obj := &internalv1.SharedVolumesInfo{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stos-shared-vols-info",
			},
			Spec: internalv1.SharedVolumesInfoSpec{
				Volumes: []internalv1.Volume{
					{
						Name:      "vol1",
						Namespace: "xyz-namespace",
						Address:   "1.2.3.4:9999",
					},
					{
						Name:      "vol2",
						Namespace: "abc-namespace",
						Address:   "9.8.7.6:1111",
					},
				},
			},
		}

		// TODO: Implement a shared volumes info cache and send the created
		// object to source channel only when there's a cache update.
		cacheUpdate := true

		if cacheUpdate {
			// Send a the created SharedVolumesInfo with GenericEvent to the
			// source channel.
			src <- event.GenericEvent{
				Meta: &metav1.ObjectMeta{
					Name: "custom-event",
				},
				Object: obj,
			}
			r.Log.V(1).Info("event dispatched")
		}

		// Wait before polling again.
		time.Sleep(pollRate)
	}
}

func (r *SharedVolumesInfoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a source channel to fetch GenericEvent.
	src := make(chan event.GenericEvent)

	// Start event dispatcher with the created source channel.
	go r.sourceEventDispatcher(src)

	// Create a Channel Source.
	stosSource := &source.Channel{
		Source: src,
	}

	// Create event handler.
	eventHandler := handler.Funcs{
		GenericFunc: r.genericEventHandler,
	}

	// Create a controller using the custom Source and EventHandler.
	// NOTE: Builder.Complete() internally calls Builder.Build() which
	// initializes a new controller, calls Builder.doWatch() to register
	// a new watchRequest for the given Kind with Builder.For(), and calls
	// Controller.Watch() with all the Builder.watchRequest. The Builder's
	// Controller can't be obtained without registerng an object Kind. This
	// makes it necessary to define a custom resource for this controller.i
	return ctrl.NewControllerManagedBy(mgr).
		Watches(stosSource, eventHandler).
		For(&internalv1.SharedVolumesInfo{}).
		Complete(r)
}

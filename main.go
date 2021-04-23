/*
MIT License

Copyright (c) 2021 StorageOS

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	cclient "github.com/darkowlzz/operator-toolkit/client/composite"
	"github.com/darkowlzz/operator-toolkit/telemetry/export"
	"github.com/darkowlzz/operator-toolkit/webhook/cert"
	"go.uber.org/zap/zapcore"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	storageosv1 "github.com/storageos/api-manager/api/v1"
	"github.com/storageos/api-manager/controllers/fencer"
	nsdelete "github.com/storageos/api-manager/controllers/namespace-delete"
	nodedelete "github.com/storageos/api-manager/controllers/node-delete"
	nodelabel "github.com/storageos/api-manager/controllers/node-label"
	podmutator "github.com/storageos/api-manager/controllers/pod-mutator"
	"github.com/storageos/api-manager/controllers/pod-mutator/scheduler"
	pvclabel "github.com/storageos/api-manager/controllers/pvc-label"
	pvcmutator "github.com/storageos/api-manager/controllers/pvc-mutator"
	"github.com/storageos/api-manager/controllers/pvc-mutator/encryption"
	"github.com/storageos/api-manager/controllers/pvc-mutator/storageclass"
	"github.com/storageos/api-manager/internal/controllers/sharedvolume"
	"github.com/storageos/api-manager/internal/pkg/cluster"
	"github.com/storageos/api-manager/internal/pkg/labels"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	apimetrics "github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	// +kubebuilder:scaffold:imports
)

const (
	// EventSourceName is added to Kubernetes events generated by the api
	// manager.  It can be used for filtering events.
	EventSourceName = "storageos-api-manager"

	oneYear = 365 * 24 * time.Hour
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("api-manager")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = storageosv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var loggerOpts zap.Options
	var namespace string
	var metricsAddr string
	var enableLeaderElection bool
	var schedulerName string
	var webhookServiceName string
	var webhookServiceNamespace string
	var webhookSecretName string
	var webhookSecretNamespace string
	var webhookConfigMutatingName string
	var webhookMutatePodsPath string
	var webhookMutatePVCsPath string
	var webhookCertValidity time.Duration
	var webhookCertRefreshInterval time.Duration
	var apiSecretPath string
	var apiEndpoint string
	var volumePollInterval time.Duration
	var nodePollInterval time.Duration
	var volumeExpiryInterval time.Duration
	var nodeExpiryInterval time.Duration
	var apiRefreshInterval time.Duration
	var apiRetryInterval time.Duration
	var k8sCreatePollInterval time.Duration
	var k8sCreateWaitDuration time.Duration
	var gcNamespaceDeleteInterval time.Duration
	var gcNodeDeleteInterval time.Duration
	var resyncNodeLabelInterval time.Duration
	var resyncPVCLabelInterval time.Duration
	var gcNamespaceDeleteDelay time.Duration
	var gcNodeDeleteDelay time.Duration
	var resyncNodeLabelDelay time.Duration
	var resyncPVCLabelDelay time.Duration
	var nsDeleteWorkers int
	var nodeDeleteWorkers int
	var nodeLabelSyncWorkers int
	var nodeFencerWorkers int
	var nodeFencerRetryInterval time.Duration
	var nodeFencerTimeout time.Duration
	var pvcLabelSyncWorkers int
	var enablePVCLabelSync bool
	var enableNodeLabelSync bool

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", "", "Namespace that the StorageOS components, including api-manager, are installed into.  Will be auto-detected if unset.")
	flag.StringVar(&schedulerName, "scheduler-name", "storageos-scheduler", "Name of the Pod scheduler to use for Pods with StorageOS volumes.  Set to an empty value to disable setting the Pod scheduler.")
	flag.StringVar(&webhookServiceName, "webhook-service-name", "storageos-webhook", "Name of the webhook service.")
	flag.StringVar(&webhookServiceNamespace, "webhook-service-namespace", "", "Namespace of the webhook service.  Will be auto-detected or value of -namespace if unset.")
	flag.StringVar(&webhookSecretName, "webhook-secret-name", "storageos-webhook", "Name of the webhook secret containing the certificate.")
	flag.StringVar(&webhookSecretNamespace, "webhook-secret-namespace", "", "Namespace of the webhook secret.  Will be auto-detected or value of -namespace if unset.")
	flag.StringVar(&webhookConfigMutatingName, "webhook-config-mutating", "storageos-mutating-webhook", "Name of the mutating webhook configuration.")
	flag.StringVar(&webhookMutatePodsPath, "webhook-mutate-pods-path", "/mutate-pods", "URL path of the Pod mutating webhook.")
	flag.StringVar(&webhookMutatePVCsPath, "webhook-mutate-pvcs-path", "/mutate-pvcs", "URL path of the PVC mutating webhook.")
	flag.DurationVar(&webhookCertValidity, "webhook-cert-validity", oneYear, "Validity of webhook certificate.")
	flag.DurationVar(&webhookCertRefreshInterval, "webhook-cert-refresh-interval", 30*time.Minute, "Frequency of webhook certificate refresh.")
	flag.StringVar(&apiSecretPath, "api-secret-path", "/etc/storageos/secrets/api", "Path where the StorageOS api secret is mounted.  The secret must have \"username\" and \"password\" set.")
	flag.StringVar(&apiEndpoint, "api-endpoint", "storageos", "The StorageOS api endpoint address.")
	flag.DurationVar(&volumePollInterval, "volume-poll-interval", 5*time.Second, "Frequency of StorageOS volume polling.")
	flag.DurationVar(&nodePollInterval, "node-poll-interval", 5*time.Second, "Frequency of StorageOS node polling.")
	flag.DurationVar(&volumeExpiryInterval, "volume-expiry-interval", time.Minute, "Frequency of cached StorageOS volume re-validation.")
	flag.DurationVar(&nodeExpiryInterval, "node-expiry-interval", 1*time.Hour, "Frequency of cached StorageOS node re-validation.")
	flag.DurationVar(&apiRefreshInterval, "api-refresh-interval", time.Minute, "Frequency of StorageOS api authentication token refresh.")
	flag.DurationVar(&apiRetryInterval, "api-retry-interval", 5*time.Second, "Frequency of StorageOS api retries on failure.")
	flag.DurationVar(&k8sCreatePollInterval, "k8s-create-poll-interval", 1*time.Second, "Frequency of Kubernetes api polling for new objects to appear once created.")
	flag.DurationVar(&k8sCreateWaitDuration, "k8s-create-wait-duration", 20*time.Second, "Maximum time to wait for new Kubernetes objects to appear.")
	flag.DurationVar(&gcNamespaceDeleteInterval, "namespace-delete-gc-interval", 1*time.Hour, "Frequency of namespace garbage collection.")
	flag.DurationVar(&gcNodeDeleteInterval, "node-delete-gc-interval", 1*time.Hour, "Frequency of node garbage collection.")
	flag.DurationVar(&resyncNodeLabelInterval, "node-label-resync-interval", 1*time.Hour, "Frequency of node label resync.")
	flag.DurationVar(&resyncPVCLabelInterval, "pvc-label-resync-interval", 1*time.Hour, "Frequency of PVC label resync.")
	flag.DurationVar(&gcNamespaceDeleteDelay, "namespace-delete-gc-delay", 20*time.Second, "Startup delay of initial namespace garbage collection.")
	flag.DurationVar(&gcNodeDeleteDelay, "node-delete-gc-delay", 30*time.Second, "Startup delay of initial node garbage collection.")
	flag.DurationVar(&resyncNodeLabelDelay, "node-label-resync-delay", 10*time.Second, "Startup delay of initial node label resync.")
	flag.DurationVar(&resyncPVCLabelDelay, "pvc-label-resync-delay", 5*time.Second, "Startup delay of initial PVC label resync.")
	flag.IntVar(&nodeFencerWorkers, "node-fencer-workers", 5, "Maximum concurrent node fencing operations.")
	flag.DurationVar(&nodeFencerRetryInterval, "node-fencer-retry-interval", 5*time.Second, "Frequency of fencing retries on failure.")
	flag.DurationVar(&nodeFencerTimeout, "node-fencer-timeout", 25*time.Second, "Maximum time to wait for fencing to complete.")
	flag.IntVar(&nodeDeleteWorkers, "node-delete-workers", 5, "Maximum concurrent node delete operations.")
	flag.IntVar(&nsDeleteWorkers, "namespace-delete-workers", 5, "Maximum concurrent namespace delete operations.")
	flag.IntVar(&nodeLabelSyncWorkers, "node-label-sync-workers", 5, "Maximum concurrent node label sync operations.")
	flag.IntVar(&pvcLabelSyncWorkers, "pvc-label-sync-workers", 5, "Maximum concurrent PVC label sync operations.")
	flag.BoolVar(&enablePVCLabelSync, "enable-pvc-label-sync", true, "Enable pvc label sync controller.")
	flag.BoolVar(&enableNodeLabelSync, "enable-node-label-sync", true, "Enable node label sync controller.")

	loggerOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	f := func(ec *zapcore.EncoderConfig) {
		ec.TimeKey = "timestamp"
		ec.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	}
	encoderOpts := func(o *zap.Options) {
		o.EncoderConfigOptions = append(o.EncoderConfigOptions, f)
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&loggerOpts), zap.StacktraceLevel(zapcore.PanicLevel), encoderOpts))

	// Setup telemetry.
	telemetryShutdown, err := export.InstallJaegerExporter("api-manager")
	if err != nil {
		fatal(err, "unable to setup telemetry exporter")
	}
	defer telemetryShutdown()

	// Block startup until there is a working StorageOS API connection.  Unless
	// we loop here, we'll get a number of failures on cold cluster start as it
	// takes longer for the api to be ready than the api-manager to start.
	var api *storageos.Client
	for {
		username, password, err := storageos.ReadCredsFromMountedSecret(apiSecretPath)
		if err != nil {
			setupLog.Info(fmt.Sprintf("unable to read storageos api secret, retrying in %s", apiRetryInterval), "msg", err)
			apimetrics.Errors.Increment("setup", err)
			time.Sleep(apiRetryInterval)
			continue
		}
		api, err = storageos.NewTracedClient(username, password, apiEndpoint)
		if err == nil {
			apimetrics.Errors.Increment("setup", nil)
			break
		}
		setupLog.Info(fmt.Sprintf("unable to connect to storageos api, retrying in %s", apiRetryInterval), "msg", err)
		apimetrics.Errors.Increment("setup", err)
		time.Sleep(apiRetryInterval)
	}
	setupLog.Info("connected to the storageos api", "api-endpoint", apiEndpoint)

	// Only attempt to grab leader lock once we have an API connection.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "storageos-api-manager-lease",
	})
	if err != nil {
		fatal(err, "unable to start manager")
	}

	// Create an uncached client to be used in the certificate manager.
	// NOTE: Cached client from manager can't be used here because the cache is
	// uninitialized at this point.
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		setupLog.Error(err, "failed to create raw client")
		os.Exit(1)
	}

	compositeClient := cclient.NewClient(mgr.GetClient(), uncachedClient, cclient.Options{})

	// Get the namespace that we're running in.  This will always be set in
	// normal deployments, but allow it to be set manually for testing.
	if namespace == "" {
		namespace, err = cluster.Namespace()
		if err != nil {
			setupLog.Error(err, "unable to determine namespace, set --namespace.")
		}
	}

	// The webhook secret and service should always be in the same namespace as
	// the api-manager.
	if webhookSecretNamespace == "" {
		webhookSecretNamespace = namespace
	}
	if webhookServiceNamespace == "" {
		webhookServiceNamespace = namespace
	}

	// Configure the certificate manager.
	certOpts := cert.Options{
		CertValidity:        webhookCertValidity,
		CertRefreshInterval: webhookCertRefreshInterval,
		Service: &admissionregistrationv1.ServiceReference{
			Name:      webhookServiceName,
			Namespace: webhookServiceNamespace,
		},
		Client:                    uncachedClient,
		SecretRef:                 &types.NamespacedName{Name: webhookSecretName, Namespace: webhookSecretNamespace},
		MutatingWebhookConfigRefs: []types.NamespacedName{{Name: webhookConfigMutatingName}},
	}
	// Create certificate manager without manager to start the provisioning
	// immediately.
	// NOTE: Certificate Manager implements nonLeaderElectionRunnable interface
	// but since the webhook server is also a nonLeaderElectionRunnable, they
	// start at the same time, resulting in a race condition where sometimes
	// the certificates aren't available when the webhook server starts. By
	// passing nil instead of the manager, the certificate manager is not
	// managed by the controller manager. It starts immediately, in a blocking
	// fashion, ensuring that the cert is created before the webhook server
	// starts.
	if err := cert.NewManager(nil, certOpts); err != nil {
		setupLog.Error(err, "unable to provision certificate")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	// Events sent on apiReset channel will trigger the api client to re-initialise.
	apiReset := make(chan struct{})

	// Parent context will be closed on interrupt or sigterm.
	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	defer cancel()

	// Goroutine to handle api credential refreshes and client reconnects
	// whenever events are received on the apiReset channel.
	go func() {
		err := api.Refresh(ctx, apiSecretPath, apiReset, apiRefreshInterval, apimetrics.Errors, setupLog)
		setupLog.Info("api token refresh stopped", "reason", err)
		os.Exit(1)
	}()

	// Register controllers with controller manager.
	setupLog.Info("starting shared volume controller ")
	if err := sharedvolume.NewReconciler(api, apiReset, mgr.GetClient(), volumePollInterval, volumeExpiryInterval, k8sCreatePollInterval, k8sCreateWaitDuration, mgr.GetEventRecorderFor(EventSourceName)).SetupWithManager(mgr); err != nil {
		fatal(err, "failed to register shared volume reconciler")
	}

	if enablePVCLabelSync {
		setupLog.Info("starting pvc label sync controller ")
		if err := pvclabel.NewReconciler(api, mgr.GetClient(), resyncPVCLabelDelay, resyncPVCLabelInterval).SetupWithManager(mgr, pvcLabelSyncWorkers); err != nil {
			fatal(err, "failed to register pvc label reconciler")
		}
	}
	if enableNodeLabelSync {
		setupLog.Info("starting node label sync controller ")
		if err := nodelabel.NewReconciler(api, mgr.GetClient(), resyncNodeLabelDelay, resyncNodeLabelInterval).SetupWithManager(mgr, nodeLabelSyncWorkers); err != nil {
			fatal(err, "failed to register node label reconciler")
		}
	}
	setupLog.Info("starting node delete controller")
	if err := nodedelete.NewReconciler(api, mgr.GetClient(), gcNodeDeleteDelay, gcNodeDeleteInterval).SetupWithManager(mgr, nodeDeleteWorkers); err != nil {
		fatal(err, "failed to register node delete reconciler")
	}
	setupLog.Info("starting namespace delete controller")
	if err := nsdelete.NewReconciler(api, mgr.GetClient(), gcNamespaceDeleteDelay, gcNamespaceDeleteInterval).SetupWithManager(mgr, nsDeleteWorkers); err != nil {
		fatal(err, "failed to register namespace delete reconciler")
	}
	setupLog.Info("starting node fencing controller")
	if err := fencer.NewReconciler(api, apiReset, mgr.GetClient(), nodePollInterval, nodeExpiryInterval).SetupWithManager(ctx, mgr, nodeFencerWorkers, nodeFencerRetryInterval, nodeFencerTimeout); err != nil {
		fatal(err, "failed to register node fencing reconciler")
	}

	// Register webhook controllers.
	decoder, err := admission.NewDecoder(scheme)
	if err != nil {
		setupLog.Error(err, "failed to build decoder")
		os.Exit(1)
	}

	podMutator := podmutator.NewController(compositeClient, decoder, []podmutator.Mutator{
		scheduler.NewPodSchedulerSetter(compositeClient, schedulerName),
	})
	mgr.GetWebhookServer().Register(webhookMutatePodsPath, &webhook.Admission{Handler: podMutator})

	pvcMutator := pvcmutator.NewController(compositeClient, decoder, []pvcmutator.Mutator{
		encryption.NewKeySetter(compositeClient, labels.Default()),
		storageclass.NewAnnotationSetter(compositeClient),
	})
	mgr.GetWebhookServer().Register(webhookMutatePVCsPath, &webhook.Admission{Handler: pvcMutator})

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		fatal(err, "failed to start manager")
	}
	setupLog.Info("shutdown complete")
}

func fatal(err error, msg string) {
	setupLog.Error(err, msg)
	os.Exit(1)
}

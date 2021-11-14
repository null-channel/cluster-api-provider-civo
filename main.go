/*
Copyright 2021.

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

package main

import (
	"flag"
	"fmt"
	"os"
	capiremote "sigs.k8s.io/cluster-api/controllers/remote"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/civo/civogo"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterapi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrastructurev1beta1 "github.com/null-channel/cluster-api-provider-civo/api/v1beta1"
	"github.com/null-channel/cluster-api-provider-civo/controllers"
	//+kubebuilder:scaffold:imports
)

const (
	defaultRegion = "LON1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterapi.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1beta1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func GetCivoClient() (*civogo.Client, error) {
	apiKey := os.Getenv("CIVO_API_KEY")
	region := os.Getenv("CIVO_REGION")
	if region == "" {
		setupLog.Info(fmt.Sprintf("Civo region was not set, using default %s", defaultRegion))
		region = defaultRegion
	}
	if apiKey == "" {
		return nil, fmt.Errorf("unable to retrieve civo api key")
	}
	client, err := civogo.NewClient(apiKey, region)
	if err != nil {
		return nil, fmt.Errorf("unable to create civo client: %w", err)
	}

	return client, nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "09b8da47.cluster.x-k8s.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	civoClient, err := GetCivoClient()
	if err != nil {
		setupLog.Error(err, "failed to init Civo client")
		os.Exit(1)
	}
	clusterTracker, err := capiremote.NewClusterCacheTracker(
		mgr,
		capiremote.ClusterCacheTrackerOptions{},
	)
	if err != nil {
		setupLog.Error(err,"Failed to init ClusterTracker")
		os.Exit(1)
	}


	if err = (&controllers.CivoClusterReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		CivoClient: civoClient,
		ClusterTracker: clusterTracker,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CivoCluster")
		os.Exit(1)
	}
	if err = (&controllers.CivoMachineReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CivoMachine")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

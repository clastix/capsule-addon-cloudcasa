// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/clastix/capsule-addon-cloudcasa/controllers"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(capsulev1beta1.AddToScheme(scheme))
}

func main() {
	var metricsAddr, probeAddr, serverURL, token string

	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&serverURL, "cloudcasa-api-url", "https://api.cloudcasa.io/api", "The CloudCasa by Catalogic API server to interact with.")
	flag.StringVar(&token, "cloudcasa-api-token", os.Getenv("CLOUDCASA_API_TOKEN"), "The bearer token used to interact with the CloudCasa by Catalogic API server.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if len(serverURL) == 0 {
		setupLog.Info("the CloudCasa by Catalogic server URL is a required parameter")
		os.Exit(1)
	}

	if len(token) == 0 {
		setupLog.Info("the CloudCasa by Catalogic token is a required parameter")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "b42b3b62.capsule-addon-cloudcasa",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.Manager{}).SetupWithManager(serverURL, token, mgr); err != nil {
		setupLog.Error(err, "unable to set up *capsulev1beta1.Tenant controller")
		os.Exit(1)
	}

	if err = (&controllers.Namespace{}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up *corev1.Namespace controller")
		os.Exit(1)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")

	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

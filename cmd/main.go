/*
Copyright 2025.

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
	"crypto/tls"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
	"github.com/asmogo/cashu-operator/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mintv1alpha1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// ServerConfig holds parsed command-line flags for the operator.
type ServerConfig struct {
	MetricsAddr     string
	ProbeAddr       string
	LeaderElection  bool
	SecureMetrics   bool
	EnableHTTP2     bool
	WebhookCertPath string
	WebhookCertName string
	WebhookCertKey  string
	MetricsCertPath string
	MetricsCertName string
	MetricsCertKey  string
}

// parseFlags parses command-line flags and configures logging.
func parseFlags() *ServerConfig {
	config := &ServerConfig{}

	flag.StringVar(&config.MetricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to. "+
			"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&config.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&config.LeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&config.SecureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&config.WebhookCertPath, "webhook-cert-path", "",
		"The directory that contains the webhook certificate.")
	flag.StringVar(&config.WebhookCertName, "webhook-cert-name", "tls.crt",
		"The name of the webhook certificate file.")
	flag.StringVar(&config.WebhookCertKey, "webhook-cert-key", "tls.key",
		"The name of the webhook key file.")
	flag.StringVar(&config.MetricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&config.MetricsCertName, "metrics-cert-name", "tls.crt",
		"The name of the metrics server certificate file.")
	flag.StringVar(&config.MetricsCertKey, "metrics-cert-key", "tls.key",
		"The name of the metrics server key file.")
	flag.BoolVar(&config.EnableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	return config
}

// buildTLSOptions returns TLS config functions, disabling HTTP/2 unless explicitly enabled.
// HTTP/2 is disabled by default due to CVEs: GHSA-qppj-fm5r-hxr3, GHSA-4374-p667-p6c8
func buildTLSOptions(enableHTTP2 bool) []func(*tls.Config) {
	if enableHTTP2 {
		return nil
	}
	return []func(*tls.Config){
		func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		},
	}
}

func configureWebhookServer(config *ServerConfig, tlsOpts []func(*tls.Config)) webhook.Server {
	opts := webhook.Options{TLSOpts: tlsOpts}

	if len(config.WebhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", config.WebhookCertPath,
			"webhook-cert-name", config.WebhookCertName,
			"webhook-cert-key", config.WebhookCertKey)

		opts.CertDir = config.WebhookCertPath
		opts.CertName = config.WebhookCertName
		opts.KeyName = config.WebhookCertKey
	}

	return webhook.NewServer(opts)
}

func configureMetricsServer(config *ServerConfig, tlsOpts []func(*tls.Config)) metricsserver.Options {
	opts := metricsserver.Options{
		BindAddress:   config.MetricsAddr,
		SecureServing: config.SecureMetrics,
		TLSOpts:       tlsOpts,
	}

	if config.SecureMetrics {
		opts.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if len(config.MetricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", config.MetricsCertPath,
			"metrics-cert-name", config.MetricsCertName,
			"metrics-cert-key", config.MetricsCertKey)

		opts.CertDir = config.MetricsCertPath
		opts.CertName = config.MetricsCertName
		opts.KeyName = config.MetricsCertKey
	}

	return opts
}

func setupManager(config *ServerConfig, tlsOpts []func(*tls.Config)) (ctrl.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                configureMetricsServer(config, tlsOpts),
		WebhookServer:          configureWebhookServer(config, tlsOpts),
		HealthProbeBindAddress: config.ProbeAddr,
		LeaderElection:         config.LeaderElection,
		LeaderElectionID:       "80fff680.cashu.asmogo.github.io",
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create manager: %w", err)
	}
	return mgr, nil
}

func setupControllers(mgr ctrl.Manager) error {
	if err := (&controller.CashuMintReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create CashuMint controller: %w", err)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := (&mintv1alpha1.CashuMint{}).SetupWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create CashuMint webhook: %w", err)
		}
	}
	// +kubebuilder:scaffold:builder

	return nil
}

func setupHealthChecks(mgr ctrl.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}
	return nil
}

func main() {
	config := parseFlags()
	tlsOpts := buildTLSOptions(config.EnableHTTP2)

	mgr, err := setupManager(config, tlsOpts)
	if err != nil {
		setupLog.Error(err, "failed to setup manager")
		os.Exit(1)
	}

	if err := setupControllers(mgr); err != nil {
		setupLog.Error(err, "failed to setup controllers")
		os.Exit(1)
	}

	if err := setupHealthChecks(mgr); err != nil {
		setupLog.Error(err, "failed to setup health checks")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

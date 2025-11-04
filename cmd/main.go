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
	// +kubebuilder:scaffold:scheme
}

// ServerConfig holds configuration for the operator server.
type ServerConfig struct {
	MetricsAddr      string
	ProbeAddr        string
	EnableLeaderElct bool
	SecureMetrics    bool
	EnableHTTP2      bool
	WebhookCertPath  string
	WebhookCertName  string
	WebhookCertKey   string
	MetricsCertPath  string
	MetricsCertName  string
	MetricsCertKey   string
}

// ParseFlags parses command-line flags and returns the server configuration.
func ParseFlags() *ServerConfig {
	config := &ServerConfig{}

	flag.StringVar(&config.MetricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to. Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable.")
	flag.StringVar(&config.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&config.EnableLeaderElct, "leader-elect", false,
		"Enable leader election for controller manager.")
	flag.BoolVar(&config.SecureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS.")
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
		"If set, HTTP/2 will be enabled for the metrics and webhook servers.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	return config
}

// buildTLSOptions creates TLS options, disabling HTTP/2 if needed.
func buildTLSOptions(enableHTTP2 bool) []func(*tls.Config) {
	var tlsOpts []func(*tls.Config)

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		})
	}

	return tlsOpts
}

// configureWebhookServer sets up the webhook server with provided certificates if available.
func configureWebhookServer(config *ServerConfig, tlsOpts []func(*tls.Config)) webhook.Server {
	webhookServerOptions := webhook.Options{TLSOpts: tlsOpts}

	if len(config.WebhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher",
			"webhook-cert-path", config.WebhookCertPath,
			"webhook-cert-name", config.WebhookCertName,
			"webhook-cert-key", config.WebhookCertKey)

		webhookServerOptions.CertDir = config.WebhookCertPath
		webhookServerOptions.CertName = config.WebhookCertName
		webhookServerOptions.KeyName = config.WebhookCertKey
	}

	return webhook.NewServer(webhookServerOptions)
}

// configureMetricsServer sets up the metrics server with authentication and certificates if needed.
func configureMetricsServer(config *ServerConfig, tlsOpts []func(*tls.Config)) metricsserver.Options {
	metricsServerOptions := metricsserver.Options{
		BindAddress:   config.MetricsAddr,
		SecureServing: config.SecureMetrics,
		TLSOpts:       tlsOpts,
	}

	if config.SecureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if len(config.MetricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher",
			"metrics-cert-path", config.MetricsCertPath,
			"metrics-cert-name", config.MetricsCertName,
			"metrics-cert-key", config.MetricsCertKey)

		metricsServerOptions.CertDir = config.MetricsCertPath
		metricsServerOptions.CertName = config.MetricsCertName
		metricsServerOptions.KeyName = config.MetricsCertKey
	}

	return metricsServerOptions
}

// setupManager creates and configures the controller manager.
func setupManager(config *ServerConfig, tlsOpts []func(*tls.Config)) (ctrl.Manager, error) {
	metricsServerOptions := configureMetricsServer(config, tlsOpts)
	webhookServer := configureWebhookServer(config, tlsOpts)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: config.ProbeAddr,
		LeaderElection:         config.EnableLeaderElct,
		LeaderElectionID:       "80fff680.cashu.asmogo.github.io",
	})
	if err != nil {
		return nil, fmt.Errorf("unable to start manager: %w", err)
	}

	return mgr, nil
}

// setupControllers registers the main controller and webhooks.
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

	return nil
}

// setupHealthChecks registers health check probes.
func setupHealthChecks(mgr ctrl.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}
	return nil
}

// nolint:gocyclo
func main() {
	config := ParseFlags()
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

	// +kubebuilder:scaffold:builder

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

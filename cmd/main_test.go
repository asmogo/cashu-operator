package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

type healthCheckManager struct {
	ctrl.Manager
	healthNames []string
	readyNames  []string
	healthErr   error
	readyErr    error
}

func (m *healthCheckManager) AddHealthzCheck(name string, checker healthz.Checker) error {
	m.healthNames = append(m.healthNames, name)
	return m.healthErr
}

func (m *healthCheckManager) AddReadyzCheck(name string, checker healthz.Checker) error {
	m.readyNames = append(m.readyNames, name)
	return m.readyErr
}

func TestParseFlagsDefaults(t *testing.T) {
	withCommandLine(t, []string{"cashu-operator"}, func() {
		config := parseFlags()

		if config.MetricsAddr != "0" {
			t.Fatalf("MetricsAddr = %q, want %q", config.MetricsAddr, "0")
		}
		if config.ProbeAddr != ":8081" {
			t.Fatalf("ProbeAddr = %q, want %q", config.ProbeAddr, ":8081")
		}
		if config.LeaderElection {
			t.Fatal("LeaderElection should default to false")
		}
		if !config.SecureMetrics {
			t.Fatal("SecureMetrics should default to true")
		}
		if config.EnableHTTP2 {
			t.Fatal("EnableHTTP2 should default to false")
		}
		if config.WebhookCertName != "tls.crt" || config.WebhookCertKey != "tls.key" {
			t.Fatal("webhook certificate defaults were not applied")
		}
		if config.MetricsCertName != "tls.crt" || config.MetricsCertKey != "tls.key" {
			t.Fatal("metrics certificate defaults were not applied")
		}
	})
}

func TestParseFlagsCustomValues(t *testing.T) {
	withCommandLine(t, []string{
		"cashu-operator",
		"--metrics-bind-address=:8443",
		"--health-probe-bind-address=:9090",
		"--leader-elect",
		"--metrics-secure=false",
		"--webhook-cert-path=/tmp/webhook",
		"--webhook-cert-name=server.crt",
		"--webhook-cert-key=server.key",
		"--metrics-cert-path=/tmp/metrics",
		"--metrics-cert-name=metrics.crt",
		"--metrics-cert-key=metrics.key",
		"--enable-http2",
	}, func() {
		config := parseFlags()

		if config.MetricsAddr != ":8443" || config.ProbeAddr != ":9090" {
			t.Fatalf("unexpected bind addresses: %+v", config)
		}
		if !config.LeaderElection || config.SecureMetrics {
			t.Fatalf("expected leader election enabled and secure metrics disabled: %+v", config)
		}
		if !config.EnableHTTP2 {
			t.Fatal("expected HTTP/2 to be enabled")
		}
		if config.WebhookCertPath != "/tmp/webhook" || config.MetricsCertPath != "/tmp/metrics" {
			t.Fatalf("certificate paths were not parsed correctly: %+v", config)
		}
		if config.WebhookCertName != "server.crt" || config.WebhookCertKey != "server.key" {
			t.Fatalf("unexpected webhook certificate names: %+v", config)
		}
		if config.MetricsCertName != "metrics.crt" || config.MetricsCertKey != "metrics.key" {
			t.Fatalf("unexpected metrics certificate names: %+v", config)
		}
	})
}

func TestBuildTLSOptions(t *testing.T) {
	t.Run("disables http2 by default", func(t *testing.T) {
		tlsOpts := buildTLSOptions(false)
		if len(tlsOpts) != 1 {
			t.Fatalf("len(buildTLSOptions(false)) = %d, want 1", len(tlsOpts))
		}

		cfg := &tls.Config{NextProtos: []string{"h2", "http/1.1"}}
		tlsOpts[0](cfg)
		if !reflect.DeepEqual(cfg.NextProtos, []string{"http/1.1"}) {
			t.Fatalf("NextProtos = %v, want [http/1.1]", cfg.NextProtos)
		}
	})

	t.Run("keeps http2 enabled when requested", func(t *testing.T) {
		if tlsOpts := buildTLSOptions(true); tlsOpts != nil {
			t.Fatalf("buildTLSOptions(true) = %v, want nil", tlsOpts)
		}
	})
}

func TestConfigureWebhookServer(t *testing.T) {
	server := configureWebhookServer(&ServerConfig{}, nil)
	if server == nil {
		t.Fatal("configureWebhookServer() returned nil")
	}
	if server.NeedLeaderElection() {
		t.Fatal("webhook server should not require leader election")
	}
	server.Register("/healthz", http.NotFoundHandler())

	serverWithCerts := configureWebhookServer(&ServerConfig{
		WebhookCertPath: "/tmp/webhook",
		WebhookCertName: "server.crt",
		WebhookCertKey:  "server.key",
	}, buildTLSOptions(false))
	if serverWithCerts == nil {
		t.Fatal("configureWebhookServer() with certs returned nil")
	}
	if serverWithCerts.StartedChecker() == nil {
		t.Fatal("webhook server started checker should be initialized")
	}
}

func TestConfigureMetricsServer(t *testing.T) {
	insecure := configureMetricsServer(&ServerConfig{
		MetricsAddr:   ":8080",
		SecureMetrics: false,
	}, nil)
	if insecure.BindAddress != ":8080" {
		t.Fatalf("BindAddress = %q, want %q", insecure.BindAddress, ":8080")
	}
	if insecure.FilterProvider != nil {
		t.Fatal("FilterProvider should be nil when secure metrics are disabled")
	}

	secure := configureMetricsServer(&ServerConfig{
		MetricsAddr:     ":8443",
		SecureMetrics:   true,
		MetricsCertPath: "/tmp/metrics",
		MetricsCertName: "metrics.crt",
		MetricsCertKey:  "metrics.key",
	}, buildTLSOptions(false))
	if secure.BindAddress != ":8443" || !secure.SecureServing {
		t.Fatalf("unexpected secure metrics options: %+v", secure)
	}
	if secure.FilterProvider == nil {
		t.Fatal("FilterProvider should be configured when secure metrics are enabled")
	}
	if secure.CertDir != "/tmp/metrics" || secure.CertName != "metrics.crt" || secure.KeyName != "metrics.key" {
		t.Fatalf("unexpected certificate settings: %+v", secure)
	}
	if len(secure.TLSOpts) != 1 {
		t.Fatalf("len(TLSOpts) = %d, want 1", len(secure.TLSOpts))
	}
}

func TestSetupHealthChecks(t *testing.T) {
	t.Run("registers both probes", func(t *testing.T) {
		mgr := &healthCheckManager{}
		if err := setupHealthChecks(mgr); err != nil {
			t.Fatalf("setupHealthChecks() error = %v", err)
		}
		if !reflect.DeepEqual(mgr.healthNames, []string{"healthz"}) {
			t.Fatalf("healthNames = %v, want [healthz]", mgr.healthNames)
		}
		if !reflect.DeepEqual(mgr.readyNames, []string{"readyz"}) {
			t.Fatalf("readyNames = %v, want [readyz]", mgr.readyNames)
		}
	})

	t.Run("returns health check registration errors", func(t *testing.T) {
		mgr := &healthCheckManager{healthErr: errors.New("health failed")}
		if err := setupHealthChecks(mgr); err == nil || err.Error() != "unable to set up health check: health failed" {
			t.Fatalf("setupHealthChecks() error = %v", err)
		}
	})

	t.Run("returns ready check registration errors", func(t *testing.T) {
		mgr := &healthCheckManager{readyErr: errors.New("ready failed")}
		if err := setupHealthChecks(mgr); err == nil || err.Error() != "unable to set up ready check: ready failed" {
			t.Fatalf("setupHealthChecks() error = %v", err)
		}
	})
}

func withCommandLine(t *testing.T, args []string, fn func()) {
	t.Helper()

	originalArgs := os.Args
	originalCommandLine := flag.CommandLine

	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flag.CommandLine = fs
	os.Args = append([]string(nil), args...)

	defer func() {
		flag.CommandLine = originalCommandLine
		os.Args = originalArgs
	}()

	fn()
}

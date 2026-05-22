package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asmogo/cashu-operator/internal/attestedentrypoint"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	deps := attestedentrypoint.Dependencies{
		Attestor:       attestedentrypoint.GoogleGKEAttestor{HTTPClient: httpClient},
		TokenExchanger: attestedentrypoint.GoogleSTSExchanger{HTTPClient: httpClient},
		SecretReader:   attestedentrypoint.GoogleSecretManagerReader{HTTPClient: httpClient},
		Executor:       attestedentrypoint.SyscallExecutor{},
	}
	if err := attestedentrypoint.Run(ctx, os.Args, os.Environ(), deps); err != nil {
		log.Fatalf("cashu attested entrypoint failed: %v", err)
	}
}

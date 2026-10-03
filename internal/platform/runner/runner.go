// Package runner concentra o ciclo de vida comum a todos os serviços:
// logger, sinais, subcomando de healthcheck e desligamento gracioso.
package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Version é preenchida no build: -ldflags "-X .../runner.Version=..."
var Version = "dev"

// Main é o main() de todo serviço. run monta o serviço e chama Serve.
func Main(service string, run func(ctx context.Context, log *slog.Logger) error) {
	// A imagem distroless não tem shell nem curl: o HEALTHCHECK do Docker
	// chama o próprio binário com este subcomando.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", service, "version", Version)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, log)
	stop()
	if err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// Serve atende HTTP até o contexto ser cancelado e então desliga com prazo.
func Serve(ctx context.Context, log *slog.Logger, app *fiber.App, addr string) error {
	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", addr)
		errCh <- app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	}
}

func healthcheck() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("healthcheck: HTTP_ADDR inválido: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/health", nil)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: status %d", resp.StatusCode)
	}
	return nil
}

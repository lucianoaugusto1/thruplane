package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gollm-gateway/internal/config"
	"gollm-gateway/internal/gateway"
	"gollm-gateway/internal/httpapi"
	"gollm-gateway/internal/provider"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the gateway YAML configuration")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(*configPath, logger); err != nil {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, logger *slog.Logger) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	clients, err := provider.NewClients(cfg)
	if err != nil {
		return fmt.Errorf("initialize providers: %w", err)
	}

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           httpapi.New(cfg, gateway.New(cfg, clients), logger),
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout),
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("gateway listening", "address", server.Addr)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		logger.Info("gateway shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout))
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP during shutdown: %w", err)
	}
	return nil
}

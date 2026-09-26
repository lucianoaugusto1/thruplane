package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nexoroute/internal/catalog"
	"nexoroute/internal/config"
	"nexoroute/internal/gateway"
	"nexoroute/internal/httpapi"
	"nexoroute/internal/provider"
	"nexoroute/internal/telemetry"
)

var (
	version   = "dev"
	revision  = "unknown"
	buildDate = "unknown"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := runCLI(os.Args[1:], os.Stdout, logger); err != nil {
		logger.Error("NexoRoute stopped", "error", err)
		os.Exit(1)
	}
}

func runCLI(args []string, stdout io.Writer, logger *slog.Logger) error {
	flags := flag.NewFlagSet("nexoroute", flag.ContinueOnError)
	flags.SetOutput(stdout)
	configPath := flags.String("config", "config.yaml", "path to the gateway YAML configuration")
	checkConfig := flags.Bool("check-config", false, "validate configuration and exit")
	showVersion := flags.Bool("version", false, "print build identity and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *checkConfig && *showVersion {
		return errors.New("-check-config and -version cannot be used together")
	}
	if *showVersion {
		build := currentBuildInfo()
		fmt.Fprintf(stdout, "nexoroute version=%s revision=%s build_date=%s\n", build.Version, build.Revision, build.Date)
		return nil
	}
	if *checkConfig {
		if _, _, _, err := initialize(*configPath); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "configuration valid: %s\n", *configPath)
		return nil
	}
	return run(*configPath, logger)
}

func run(configPath string, logger *slog.Logger) error {
	cfg, registry, clients, err := initialize(configPath)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           httpapi.NewWithBuildInfo(cfg, gateway.NewWithCatalog(cfg, clients, registry), logger, currentBuildInfo()),
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout),
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("NexoRoute listening", "address", server.Addr)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		logger.Info("NexoRoute shutting down")
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

func initialize(configPath string) (config.Config, *catalog.Registry, map[string]*provider.Client, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	registry, err := catalog.BuiltIn()
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("load model catalog: %w", err)
	}
	clients, err := provider.NewClients(cfg)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("initialize providers: %w", err)
	}
	return cfg, registry, clients, nil
}

func currentBuildInfo() telemetry.BuildInfo {
	return telemetry.BuildInfo{Version: version, Revision: revision, Date: buildDate}
}

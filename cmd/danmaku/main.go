package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/api"
	"git.hanada.info/tamersunion/danmaku/internal/config"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "configuration file path")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if err := run(*configPath); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	repository, err := store.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer repository.Close()
	if err := repository.Initialize(ctx); err != nil {
		return err
	}

	handler, err := api.New(ctx, cfg, repository, slog.Default())
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listeners, err := openListeners(cfg)
	if err != nil {
		return err
	}
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()

	errorsChannel := make(chan error, len(listeners))
	for _, listener := range listeners {
		slog.Info("listening", "address", listener.Addr().String(), "version", version)
		go func(listener net.Listener) {
			if serveErr := httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				errorsChannel <- serveErr
			}
		}(listener)
	}

	select {
	case <-ctx.Done():
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		return httpServer.Shutdown(shutdownContext)
	case err := <-errorsChannel:
		return err
	}
}

func openListeners(cfg config.Config) ([]net.Listener, error) {
	listeners := make([]net.Listener, 0, 2)
	if cfg.KestrelSettings.Port != 0 {
		address := net.JoinHostPort(cfg.KestrelSettings.Host, fmt.Sprintf("%d", cfg.KestrelSettings.Port))
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return nil, fmt.Errorf("listen on %s: %w", address, err)
		}
		listeners = append(listeners, listener)
	}
	if cfg.KestrelSettings.UnixSocketPath != "" && runtime.GOOS != "windows" {
		path := cfg.KestrelSettings.UnixSocketPath
		if info, statErr := os.Lstat(path); statErr == nil {
			if info.Mode()&os.ModeSocket == 0 {
				return nil, fmt.Errorf("refuse to replace non-socket path %s", path)
			}
			if removeErr := os.Remove(path); removeErr != nil {
				return nil, fmt.Errorf("remove stale Unix socket %s: %w", path, removeErr)
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, statErr
		}
		listener, err := net.Listen("unix", path)
		if err != nil {
			return nil, fmt.Errorf("listen on Unix socket %s: %w", path, err)
		}
		listeners = append(listeners, listener)
	}
	if len(listeners) == 0 {
		return nil, errors.New("no listener configured")
	}
	return listeners, nil
}

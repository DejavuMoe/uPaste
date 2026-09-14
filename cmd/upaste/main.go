package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DejavuMoe/uPaste/internal/abuse"
	"github.com/DejavuMoe/uPaste/internal/config"
	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/fileapi"
	"github.com/DejavuMoe/uPaste/internal/httpapi"
	"github.com/DejavuMoe/uPaste/internal/maintenance"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
	"github.com/DejavuMoe/uPaste/internal/share"
)

func newHandler(api http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if api != nil && (strings.HasPrefix(r.URL.Path, "/api/v1/") || r.URL.Path == "/raw" || strings.HasPrefix(r.URL.Path, "/raw/")) {
			api.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) (err error) {
	cfg, err := config.Parse(os.Args[1:], os.LookupEnv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := database.Open(ctx, cfg.DataDir)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	store, err := objectstore.OpenLocal(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("initialize object storage: %w", err)
	}
	defer func() {
		if closeErr := store.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close object storage: %w", closeErr)
		}
	}()
	shareService := share.NewWithStore(db, store, time.Now)
	control := abuse.New(abuse.Config{Trusted: cfg.TrustedProxies})
	appServer := newServer(cfg.Addr, newHandler(httpapi.NewWithAbuse(shareService, cfg.FileOrigin, log, control)))
	fileServer := newServer(cfg.FileAddr, fileapi.NewWithAbuse(shareService, log, control))
	fileListener, err := net.Listen("tcp", cfg.FileAddr)
	if err != nil {
		return fmt.Errorf("listen file HTTP: %w", err)
	}
	defer fileListener.Close()
	appListener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen application HTTP: %w", err)
	}
	defer appListener.Close()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, server := range []*http.Server{appServer, fileServer} {
			if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil && !errors.Is(shutdownErr, http.ErrServerClosed) {
				log.Error("shutdown failed", "address", server.Addr, "error", shutdownErr)
			}
		}
	}()

	serveErrors := make(chan error, 2)
	go func() { serveErrors <- fileServer.Serve(fileListener) }()
	go func() { serveErrors <- appServer.Serve(appListener) }()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, server := range []*http.Server{appServer, fileServer} {
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Error("shutdown failed", "address", server.Addr, "error", err)
			}
		}
	}()

	log.Info("server listening", "address", appServer.Addr)
	log.Info("file server listening", "address", fileServer.Addr)
	maintenanceCtx, cancelMaintenance := context.WithCancel(ctx)
	maintenanceDone := make(chan struct{})
	go func() {
		defer close(maintenanceDone)
		maintenance.Start(maintenanceCtx, maintenance.New(db, store), time.Now, log)
	}()
	defer func() { cancelMaintenance(); <-maintenanceDone }()
	if err := <-serveErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DejavuMoe/uPaste/internal/abuse"
	"github.com/DejavuMoe/uPaste/internal/admin"
	"github.com/DejavuMoe/uPaste/internal/adminapi"
	"github.com/DejavuMoe/uPaste/internal/buildinfo"
	"github.com/DejavuMoe/uPaste/internal/challenge"
	"github.com/DejavuMoe/uPaste/internal/config"
	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/fileapi"
	"github.com/DejavuMoe/uPaste/internal/httpapi"
	"github.com/DejavuMoe/uPaste/internal/maintenance"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
	"github.com/DejavuMoe/uPaste/internal/share"
	"github.com/DejavuMoe/uPaste/internal/webapp"
)

// newHandler composes the application listener. Precedence is explicit:
// API and raw routes always reach the API handler, /healthz always reaches the
// liveness mux, and only then may the embedded frontend handle a request.
// A nil frontend preserves the development shape where Vite serves the UI.
func newHandler(api http.Handler, adminHTTP http.Handler, frontend http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			mux.ServeHTTP(w, r)
			return
		case adminHTTP != nil && (r.URL.Path == "/api/v1/admin" || strings.HasPrefix(r.URL.Path, "/api/v1/admin/")):
			adminHTTP.ServeHTTP(w, r)
			return
		case strings.HasPrefix(r.URL.Path, "/api/"), r.URL.Path == "/raw", strings.HasPrefix(r.URL.Path, "/raw/"):
			if api == nil {
				http.NotFound(w, r)
				return
			}
			api.ServeHTTP(w, r)
			return
		}
		if frontend != nil {
			frontend.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log, os.Args[1:], os.Stdout); err != nil {
		log.Error("application failed", "error", err)
		os.Exit(1)
	}
}

// maybePrintVersion handles the operator version command without touching
// configuration, SQLite, listeners, or maintenance. It accepts exactly one
// version flag so unknown or combined flags continue through normal parsing.
func maybePrintVersion(args []string, stdout io.Writer) (bool, error) {
	if len(args) != 1 || (args[0] != "--version" && args[0] != "-version") {
		return false, nil
	}
	_, err := io.WriteString(stdout, buildinfo.Current().String())
	return true, err
}

func run(log *slog.Logger, args []string, stdout io.Writer) (err error) {
	if handled, versionErr := maybePrintVersion(args, stdout); handled {
		return versionErr
	}
	cfg, err := config.Parse(args, os.LookupEnv)
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
	shareService.SetRetentionPolicy(share.RetentionPolicy{
		Public:     cfg.Public(),
		DefaultTTL: cfg.PublicDefaultTTL,
		MaxTTL:     cfg.PublicMaxTTL,
	})
	control := abuse.New(abuse.Config{Trusted: cfg.TrustedProxies})
	maintenanceService := maintenance.New(db, store)

	var challengeVerifier challenge.Verifier
	if cfg.ChallengeEnabled() {
		verifier, err := challenge.NewVerifier(cfg.Challenge, nil, cfg.ChallengeTimeout)
		if err != nil {
			return fmt.Errorf("initialize challenge verifier: %w", err)
		}
		challengeVerifier = verifier
	}

	var adminHTTP http.Handler
	if cfg.AdminEnabled {
		manager := admin.NewManager(cfg.AdminVerifier, admin.Options{SecureCookie: cfg.AdminCookieSecure, Now: time.Now})
		adminHTTP = adminapi.New(shareService, manager, maintenanceService, cfg.FileOrigin, cfg.TrustedProxies, log, time.Now)
		log.Info("superadmin enabled")
	} else {
		log.Info("superadmin disabled")
	}

	frontend, err := webapp.NewEmbedded(webapp.Options{
		AdminEnabled:      cfg.AdminEnabled,
		ChallengeProvider: string(cfg.Challenge.Provider),
		CapEndpoint:       cfg.Challenge.CapEndpoint,
	})
	if err != nil {
		return fmt.Errorf("initialize embedded frontend: %w", err)
	}
	if frontend == nil {
		log.Info("embedded frontend disabled; serve the development UI with the Vite dev server")
	} else {
		log.Info("embedded frontend enabled")
	}

	apiHandler := httpapi.NewWithOptions(shareService, cfg.FileOrigin, log, control, httpapi.Options{
		Public:           cfg.Public(),
		PublicDefaultTTL: cfg.PublicDefaultTTL,
		PublicMaxTTL:     cfg.PublicMaxTTL,
		Challenge:        challengeVerifier,
		ChallengeConfig:  cfg.Challenge.PublicConfig(),
		ChallengeTimeout: cfg.ChallengeTimeout,
		AdminEnabled:     cfg.AdminEnabled,
	})
	appServer := newServer(cfg.Addr, newHandler(apiHandler, adminHTTP, frontend))
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
		maintenance.Start(maintenanceCtx, maintenanceService, time.Now, log)
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

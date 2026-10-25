package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"streamops/internal/config"
	httpapi "streamops/internal/http"
	"streamops/internal/observability"
	"streamops/internal/repository"
	"streamops/internal/service"
	"streamops/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("application arrêtée", "reason", err.Error())
		os.Exit(1)
	}
}
func run(log *slog.Logger) error {
	// La sonde du conteneur réutilise le binaire statique, sans interpréteur de commandes ni curl.
	if len(os.Args) > 1 && os.Args[1] == "probe" {
		c := http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://127.0.0.1:8080/readyz")
		if err != nil {
			return fmt.Errorf("échec de la sonde")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("sonde non disponible")
		}
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pc, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("DATABASE_URL invalide")
	}
	pc.MaxConns = cfg.PoolMax
	pc.MinConns = 1
	pc.MaxConnLifetime = 30 * time.Minute
	pc.MaxConnIdleTime = 5 * time.Minute
	pc.ConnConfig.ConnectTimeout = cfg.DBTimeout
	pool, err := pgxpool.NewWithConfig(root, pc)
	if err != nil {
		return fmt.Errorf("échec de l’initialisation du pool de connexions")
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(root, 30*time.Second)
	defer cancel()
	if err = pool.Ping(ctx); err != nil {
		return fmt.Errorf("base de données indisponible au démarrage")
	}
	if len(os.Args) > 1 {
		if os.Args[1] != "migrate" {
			return fmt.Errorf("utilisation : streamops [migrate|probe]")
		}
		if err = migrations.Up(ctx, pool); err != nil {
			return fmt.Errorf("échec de la migration : %w", err)
		}
		log.Info("migrations appliquées")
		return nil
	}
	// Le processus vérifie le schéma avant d’accepter les requêtes.
	if err = migrations.Verify(ctx, pool); err != nil {
		return fmt.Errorf("échec de la vérification du schéma ; exécutez streamops migrate avec l’image de cette version")
	}
	shutdownTracing, err := observability.Tracing(ctx, cfg.OTLPEndpoint)
	if err != nil {
		return fmt.Errorf("échec de l’initialisation du traçage")
	}
	defer func() {
		ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if shutdownTracing(ctx) != nil {
			log.Warn("échec de l’arrêt de l’export des traces")
		}
	}()
	handler := httpapi.New(service.App{Store: &repository.Postgres{Pool: pool}}, cfg, log, observability.NewMetrics())
	server := &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	return serve(root, server, cfg.ShutdownTimeout, log)
}

func serve(root context.Context, server *http.Server, shutdownTimeout time.Duration, log *slog.Logger) error {
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	log.Info("serveur démarré", "address", server.Addr)
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("échec du serveur HTTP : %w", err)
		}
		return nil
	case <-root.Done():
	}
	ctxShutdown, c := context.WithTimeout(context.Background(), shutdownTimeout)
	defer c()
	if err := server.Shutdown(ctxShutdown); err != nil {
		_ = server.Close()
		return fmt.Errorf("délai d’arrêt progressif dépassé")
	}
	log.Info("serveur arrêté")
	return nil
}

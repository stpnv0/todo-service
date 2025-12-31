package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo-service/internal/config"
	"todo-service/internal/handler"
	"todo-service/internal/logger"
	"todo-service/internal/middleware"
	"todo-service/internal/repository"
)

func main() {
	cfg := config.MustLoad()

	log := logger.InitLogger(&cfg.Logging)

	repo := repository.NewRepository()
	h := handler.NewHandler(repo, log)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	handlerWithMiddleware := middleware.LoggingMiddleware(log)(mux)

	server := &http.Server{
		Addr:         cfg.Server.GetAddr(),
		Handler:      handlerWithMiddleware,
		ReadTimeout:  cfg.Server.Timeout,
		WriteTimeout: cfg.Server.Timeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Info("server starting",
			slog.String("address", server.Addr),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed to start",
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	sig := <-stop
	log.Info("shutdown signal received",
		slog.String("signal", sig.String()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.Shutdown(ctx, server); err != nil {
		log.Error("graceful shutdown failed",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	log.Info("server stopped gracefully")
}

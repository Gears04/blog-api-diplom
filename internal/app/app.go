package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blog-api/internal/config"
	"blog-api/internal/database"
	"blog-api/internal/handler"
	"blog-api/internal/logger"
	"blog-api/internal/middleware"
	postgresrepo "blog-api/internal/repository/postgres"
	"blog-api/internal/service"
)

// Run is shared by main.go and the original cmd/api/main.go entry point.
func Run() (runErr error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fileLogger, err := logger.New(cfg.LogFile)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	previousOutput := log.Writer()
	log.SetOutput(io.MultiWriter(os.Stderr, fileLogger))
	defer func() {
		if runErr != nil {
			log.Printf("server error: %v", runErr)
		}
		log.SetOutput(previousOutput)
		if err := fileLogger.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close log file: %w", err))
		}
	}()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	db, err := database.Open(startupCtx, cfg.DatabaseURL)
	cancelStartup()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := database.Migrate(db, "migrations"); err != nil {
		return err
	}

	userRepository := postgresrepo.NewUserRepository(db)
	postRepository := postgresrepo.NewPostRepository(db)
	commentRepository := postgresrepo.NewCommentRepository(db)
	userService := service.NewUserService(userRepository, cfg.JWTSecret, cfg.JWTTTL)
	postService := service.NewPostService(postRepository)
	commentService := service.NewCommentService(commentRepository, postRepository)
	authMiddleware := middleware.NewAuth(userService)
	apiHandler := handler.New(userService, postService, commentService, authMiddleware, db)

	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           middleware.Logging(middleware.Recovery(apiHandler.Routes())),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()
	log.Printf("starting blog API on http://localhost:%s", cfg.ServerPort)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		log.Print("shutting down server")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("shutdown server: %w", err)
	}
	log.Print("server stopped")
	return nil
}

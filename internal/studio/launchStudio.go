package studio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/speakeasy-api/speakeasy-core/auth"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/browser"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"go.uber.org/zap"
)

func LaunchStudio(ctx context.Context, workflow *run.Workflow) error {
	secret, err := getOrCreateSecret()
	if err != nil {
		return fmt.Errorf("error creating studio secret key: %w", err)
	}

	if workflow == nil {
		return errors.New("unable to launch studio without a workflow")
	}

	handlers, err := NewStudioHandlers(ctx, workflow)
	if err != nil {
		return fmt.Errorf("error creating studio handlers: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler(handlers.health))

	mux.HandleFunc("/source", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler(handlers.updateSource)(w, r)
		case http.MethodGet:
			handler(handlers.getSource)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler(handlers.updateRun)(w, r)
		case http.MethodGet:
			handler(handlers.getRun)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	port, err := searchForAvailablePort()
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsMiddleware(authMiddleware(secret, mux)),
	}

	serverURL := auth.GetWorkspaceBaseURL(ctx)

	url := fmt.Sprintf("%s/studio/%d#%s", serverURL, port, secret)

	if err := browser.OpenURL(url); err != nil {
		fmt.Println("Please open the following URL in your browser:", url)
	} else {
		fmt.Println("Opening URL in your browser:", url)
	}

	return startServer(ctx, server)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authMiddleware(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Secret-Key") != secret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handler(h func(context.Context, http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.From(r.Context()).Info("handling request", zap.String("method", r.Method), zap.String("path", r.URL.Path))
		ctx := r.Context()
		if err := h(ctx, w, r); err != nil {
			log.From(ctx).Error("error handling request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Error(err))
			respondJSONError(ctx, w, err)
		}
	}
}

func startServer(ctx context.Context, server *http.Server) error {

	// Channel to listen for interrupt or terminate signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		// Attempt to gracefully shutdown the server
		// TODO: Clean up temp dir
		if err := server.Shutdown(ctx); err != nil {
			log.From(ctx).Error("Server forced to shutdown", zap.Error(err))
		}
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

func searchForAvailablePort() (int, error) {
	for port := 3333; port < 7000; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = l.Close()
			return port, nil
		}
	}
	return 0, errors.New("no available port found")
}

func getOrCreateSecret() (string, error) {
	secret := config.GetStudioSecret()
	if secret == "" {
		secret = generateSecret()
		if err := config.SetStudioSecret(secret); err != nil {
			return "", fmt.Errorf("error saving studio secret: %w", err)
		}
	}
	return secret, nil
}

func generateSecret() string {
	n := 16
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	// Short for studio - the prefix allows us to easily identify the secret and version it if needed in the future
	return "stu-" + hex.EncodeToString(b)
}

func respondJSONError(ctx context.Context, w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError

	switch {
	case errors.Is(err, errors.ErrUnauthorized):
		code = http.StatusUnauthorized
	case errors.Is(err, errors.ErrValidation):
		code = http.StatusUnprocessableEntity
	case errors.Is(err, errors.ErrBadRequest):
		code = http.StatusBadRequest
	case errors.Is(err, errors.ErrNotFound):
		code = http.StatusNotFound
	}

	w.WriteHeader(code)
	data := map[string]interface{}{
		"error":      err.Error(),
		"statusCode": code,
	}
	if jsonError := json.NewEncoder(w).Encode(data); jsonError != nil {
		log.From(ctx).Error("failed to encode JSON error response", zap.Error(jsonError))
	}
}

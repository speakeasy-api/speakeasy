package studio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/samber/lo"
	"github.com/speakeasy-api/speakeasy-core/suggestions"
	"github.com/speakeasy-api/speakeasy/internal/env"
	"github.com/speakeasy-api/speakeasy/internal/utils"
	"golang.org/x/exp/maps"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/speakeasy-api/speakeasy-core/auth"

	"github.com/pkg/browser"
	"github.com/speakeasy-api/speakeasy-core/errors"
	"github.com/speakeasy-api/speakeasy/internal/config"
	"github.com/speakeasy-api/speakeasy/internal/log"
	"github.com/speakeasy-api/speakeasy/internal/run"
	"go.uber.org/zap"
)

// CanLaunch returns true if the studio can be launched, and the number of diagnostics
func CanLaunch(ctx context.Context, wf *run.Workflow) (bool, int) {
	if len(wf.SourceResults) != 1 {
		// Only one source at a time is supported in the studio at the moment
		return false, 0
	}

	sourceResult := maps.Values(wf.SourceResults)[0]

	if !utils.IsInteractive() || env.IsGithubAction() {
		return false, 0
	}

	if sourceResult.LintResult == nil {
		// No lint result indicates the spec wasn't even loaded successfully, the studio can't help with that
		return false, 0
	}

	// TODO: include more relevant diagnostics as we go!
	numDiagnostics := lo.SumBy(maps.Values(sourceResult.Diagnosis), func(x []suggestions.Diagnostic) int {
		return len(x)
	})

	return numDiagnostics > 0, numDiagnostics
}

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

	mux.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler(handlers.reRun)(w, r)
		case http.MethodGet:
			handler(handlers.getLastRunResult)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/suggest/method-names", handler(handlers.suggestMethodNames))

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

	// After ten seconds, if the health check hasn't been seen then kill the server
	go func() {
		time.Sleep(1 * time.Minute)
		if !handlers.healthCheckSeen {
			log.From(ctx).Warnf("Health check not seen, shutting down server")
			err := server.Shutdown(context.Background())
			if err != nil {
				fmt.Println("Error shutting down server:", err)
			}
		}
	}()

	return startServer(ctx, server, workflow)
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
		start := time.Now()
		id := generateRequestID()
		method := fmt.Sprintf("%-6s", r.Method)  // Fixed width 6 characters
		path := fmt.Sprintf("%-21s", r.URL.Path) // Fixed width 21 characters
		base := fmt.Sprintf("%s %s %s", id, method, path)
		log.From(r.Context()).Info(fmt.Sprintf("%s started", base))
		ctx := r.Context()
		if err := h(ctx, w, r); err != nil {
			log.From(ctx).Error(fmt.Sprintf("%s failed: %v", base, err))
			respondJSONError(ctx, w, err)
			return
		}
		duration := time.Since(start)
		log.From(ctx).Info(fmt.Sprintf("%s completed in %s", base, duration))
	}
}

func startServer(ctx context.Context, server *http.Server, workflow *run.Workflow) error {
	// Channel to listen for interrupt or terminate signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		// Attempt to gracefully shutdown the server
		workflow.Cleanup()
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
		"message":    err.Error(),
		"statusCode": code,
	}
	if jsonError := json.NewEncoder(w).Encode(data); jsonError != nil {
		log.From(ctx).Error("failed to encode JSON error response", zap.Error(jsonError))
	}
}

var counter int

func generateRequestID() string {
	counter++
	return fmt.Sprintf("%03d", counter)
}

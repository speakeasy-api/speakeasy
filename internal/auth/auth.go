package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pkg/browser"
	"github.com/speakeasy-api/speakeasy/internal/config"
)

const (
	// Comment these out and uncomment the localhost ones to test locally
	appURL = "https://app.speakeasyapi.dev"
	apiURL = "https://api.prod.speakeasyapi.dev"
	//appURL = "http://localhost:35291"
	//apiURL = "http://localhost:35290"
)

type authResult struct {
	config.SpeakeasyAuthInfo
	err error
}

func Authenticate(force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	existingApiKey, setByEnvVar := config.GetSpeakeasyAPIKey()
	if existingApiKey != "" && !force {
		if err := testAuth(existingApiKey); err != nil {
			if setByEnvVar {
				return err
			}
		} else {
			return nil
		}
	}

	if !force {
		fmt.Println("Authentication needed")
	}

	done := make(chan authResult)

	addr, srv := startServer(done)
	defer func() { _ = srv.Shutdown(ctx) }()

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s?cli_callback_url=%s&cli_host_name=%s\n", appURL, addr, hostname)

	if err := browser.OpenURL(url); err != nil {
		fmt.Println("Please open the following URL in your browser:", url)
	} else {
		fmt.Println("Opening URL in your browser:", url)
	}

	var res authResult
	select {
	case <-ctx.Done():
		return fmt.Errorf("authentication timed out")
	case res = <-done:
	}

	if res.err != nil {
		return res.err
	}

	if err := testAuth(res.APIKey); err != nil {
		return err
	}

	if err := config.SetSpeakeasyAuthInfo(res.SpeakeasyAuthInfo); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	fmt.Printf("Authenticated with workspace successfully - %s/workspaces/%s\n", appURL, res.WorkspaceID)

	return nil
}

func Logout() error {
	if err := config.ClearSpeakeasyAuthInfo(); err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	fmt.Println("Logout successful!")

	return nil
}

func testAuth(apiKey string) error {
	// TODO eventually replace with a call from the SDK
	u, err := url.JoinPath(apiURL, "v1/auth/validate")
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to successfully authenticate with the Speakeasy Platform. Contact Speakeasy Support for Help support@speakeasyapi.dev: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("api key invalid! Please try to reauthenticate: %d", res.StatusCode)
	}

	return nil
}

func startServer(done chan authResult) (string, *http.Server) {
	srv := &http.Server{}

	resultSent := false
	sendResult := func(result authResult) {
		if !resultSent {
			done <- result
			resultSent = true
		}
	}
	var res config.SpeakeasyAuthInfo

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			return
		}

		res.APIKey = r.URL.Query().Get("apiKey")
		res.CustomerID = r.URL.Query().Get("customerId")
		res.WorkspaceID = r.URL.Query().Get("workspaceId")

		http.Redirect(w, r, "/complete", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/complete", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		if len(res.APIKey) == 0 || len(res.CustomerID) == 0 || len(res.WorkspaceID) == 0 {
			sendResult(authResult{err: fmt.Errorf("empty values in AuthInfo %v", res)})
			return
		}
		w.Write([]byte("Authentication successful! You can now close this tab."))
		sendResult(authResult{SpeakeasyAuthInfo: res})
	})

	srv.Handler = mux
	l, close := createListener()

	go func() {
		defer close()

		err := srv.Serve(l)
		sendResult(authResult{err: err})
	}()

	return fmt.Sprintf("http://localhost:%d/callback", l.Addr().(*net.TCPAddr).Port), srv
}

func createListener() (l net.Listener, close func()) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	return l, func() {
		_ = l.Close()
	}
}

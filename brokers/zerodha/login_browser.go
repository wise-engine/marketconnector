package zerodha

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/sunnyme20/marketconnector/model"
)

const (
	// defaultCallbackPort is the local port the temporary login callback server
	// listens on.
	defaultCallbackPort = "8080"

	// defaultCallbackPath is the path the Kite Connect app's redirect URL must
	// point at: http://localhost:8080/api/user/callback/kite/.
	defaultCallbackPath = "/api/user/callback/kite/"

	// defaultLoginTimeout bounds how long the login flow waits for the browser
	// redirect to reach the local callback server.
	defaultLoginTimeout = 5 * time.Minute

	// shutdownTimeout bounds how long we wait for the callback server to stop.
	shutdownTimeout = 5 * time.Second
)

// LoginWithBrowser performs the Kite Connect oauth login flow by opening the
// login URL in the default browser and running a temporary local server that
// captures the request_token from the login redirect. On success the access and
// refresh tokens are stored on the broker and returned.
//
// apiKey    is the Kite Connect API key.
// apiSecret is the Kite Connect API secret.
//
// The redirect URL registered for the Kite Connect app must be
// http://localhost:8080/api/user/callback/kite/ so the login redirect reaches
// the local callback server.
func (z *Zerodha) LoginWithBrowser(apiKey, apiSecret string) (*model.Response[model.LoginResponse], error) {
	z.apiKey = apiKey
	z.apiSecret = apiSecret
	z.client = nil

	kc := z.kite()
	loginURL := kc.GetLoginURL()
	fmt.Println("Open the following url in your browser:\n", loginURL)
	_ = openBrowser(loginURL)

	// Temporary local server that captures the request_token from the callback.
	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(defaultCallbackPath, func(w http.ResponseWriter, r *http.Request) {
		tokens, ok := r.URL.Query()["request_token"]
		if !ok || len(tokens) == 0 {
			errCh <- errors.New("callback received without request_token")
			http.Error(w, "missing request_token", http.StatusBadRequest)
			return
		}
		tokenCh <- tokens[0]
		_, _ = w.Write([]byte("login successful!"))
	})

	srv := &http.Server{Addr: ":" + defaultCallbackPort, Handler: mux}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return nil, fmt.Errorf("zerodha: callback server on :%s: %w", defaultCallbackPort, err)
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var requestToken string
	select {
	case rt := <-tokenCh:
		requestToken = rt
	case err := <-errCh:
		_ = srv.Close()
		return nil, fmt.Errorf("zerodha: login callback: %w", err)
	case <-time.After(defaultLoginTimeout):
		_ = srv.Close()
		return nil, errors.New("zerodha: timed out waiting for login callback")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)

	session, err := kc.GenerateSession(requestToken, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("zerodha: generate session: %w", err)
	}

	z.SetAccessToken(session.AccessToken)
	z.SetRefreshToken(session.RefreshToken)

	return &model.Response[model.LoginResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data: model.LoginResponse{
			AccessToken:  session.AccessToken,
			RefreshToken: session.RefreshToken,
			// Zerodha has no feed token; the WebSocket uses the access token.
			FeedToken: "",
		},
	}, nil
}

// openBrowser attempts to open url in the default browser. It is best-effort:
// the returned error is informational and the caller should still print the
// URL so the flow works in headless environments too.
func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", "", url}
	default: // Linux and other Unix-like systems.
		cmd, args = "xdg-open", []string{url}
	}
	return exec.Command(cmd, args...).Start()
}

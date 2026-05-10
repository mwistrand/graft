package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/mwistrand/graft/internal/provider"
)

// defaultAPIPackage is the npm package spec used when no override is supplied.
// Callers should configure a pinned version (e.g. copilot-api@1.2.3) for any
// production use; @latest is retained for backward compatibility only.
const defaultAPIPackage = "copilot-api@latest"

// ProxyManager handles the lifecycle of the copilot-api proxy server.
type ProxyManager struct {
	baseURL    string
	apiPackage string // npm package spec passed to npx
	mu         sync.Mutex
	cmd        *exec.Cmd
	started    bool
	models     []provider.ModelInfo // cached models from /v1/models
}

// NewProxyManager creates a new proxy manager for the given base URL.
func NewProxyManager(baseURL string) *ProxyManager {
	return NewProxyManagerWithPackage(baseURL, defaultAPIPackage)
}

// NewProxyManagerWithPackage creates a proxy manager that auto-launches the
// given npm package spec. An empty apiPackage falls back to the default.
func NewProxyManagerWithPackage(baseURL, apiPackage string) *ProxyManager {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if apiPackage == "" {
		apiPackage = defaultAPIPackage
	}
	return &ProxyManager{baseURL: baseURL, apiPackage: apiPackage}
}

// EnsureRunning checks if the proxy is running and starts it if not.
// Returns true if the proxy was started by this call (and should be stopped later).
//
// Concurrent callers are safe: at most one will start the subprocess. All callers
// wait for readiness before returning, so once EnsureRunning returns nil the proxy
// is reachable. Only the caller that actually started the subprocess receives
// started=true; that caller alone is responsible for Stop().
func (m *ProxyManager) EnsureRunning(ctx context.Context, logFn func(string, ...any)) (bool, error) {
	// Fast path: already running, no need to start anything.
	if m.IsRunning(ctx) {
		return false, nil
	}

	started, err := m.tryStart(ctx, logFn)
	if err != nil {
		return false, err
	}

	logFn("Waiting for proxy to be ready (you may need to authenticate with GitHub)...")

	if err := m.WaitReady(ctx, 2*time.Minute); err != nil {
		if started {
			m.Stop()
		}
		return false, fmt.Errorf("proxy failed to start: %w", err)
	}

	if started {
		logFn("Copilot proxy ready")
	}
	return started, nil
}

// tryStart attempts to launch the subprocess under the lock.
// Returns started=true only if this call actually launched the process.
func (m *ProxyManager) tryStart(ctx context.Context, logFn func(string, ...any)) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil {
		// Another goroutine already started it; we'll wait alongside them.
		return false, nil
	}

	logFn("Starting copilot-api proxy...")
	if err := m.startLocked(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// IsRunning checks if the proxy is responding at the configured URL.
// If the proxy is running, it also caches the available models.
func (m *ProxyManager) IsRunning(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", m.baseURL+"/v1/models", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Parse and cache the models response
	var modelsResp struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by,omitempty"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err == nil {
		m.mu.Lock()
		m.models = make([]provider.ModelInfo, len(modelsResp.Data))
		for i, model := range modelsResp.Data {
			m.models[i] = provider.ModelInfo{
				ID:   model.ID,
				Name: model.ID,
			}
		}
		m.mu.Unlock()
	}

	return true
}

// Models returns a copy of the cached models from the last successful /v1/models request.
func (m *ProxyManager) Models() []provider.ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]provider.ModelInfo, len(m.models))
	copy(result, m.models)
	return result
}

// Start launches the copilot-api proxy as a subprocess.
func (m *ProxyManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startLocked(ctx)
}

// startLocked launches the proxy (caller must hold mu).
func (m *ProxyManager) startLocked(ctx context.Context) error {
	if m.cmd != nil {
		return nil
	}

	// Try npx first (most common way to run copilot-api). The package spec is
	// caller-controlled so users can pin a known-good version.
	m.cmd = exec.CommandContext(ctx, "npx", m.apiPackage, "start")
	m.cmd.Stdout = os.Stderr // Redirect to stderr so it doesn't interfere with graft output
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		m.cmd = nil
		return fmt.Errorf("failed to start copilot-api proxy: %w\nMake sure Node.js and npm are installed", err)
	}

	m.started = true
	return nil
}

// WaitReady waits for the proxy to become responsive.
func (m *ProxyManager) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if m.IsRunning(ctx) {
			return nil
		}

		if m.cmdExited() {
			return fmt.Errorf("proxy process exited unexpectedly")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
			// Continue polling
		}
	}

	return fmt.Errorf("timeout waiting for proxy to start (did you complete GitHub authentication?)")
}

// cmdExited reports whether a started subprocess has already terminated.
// Returns false when no process was ever started (m.cmd == nil) or when it is
// still running. Reads m.cmd under the lock to stay race-free with Stop().
func (m *ProxyManager) cmdExited() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.cmd.ProcessState != nil && m.cmd.ProcessState.Exited()
}

// Stop terminates the proxy if it was started by this manager.
//
// The cmd is detached from the manager (m.cmd = nil) before the shutdown
// sequence runs, so concurrent observers (e.g. WaitReady) never read a
// half-stopped subprocess. Wait/Kill operate on a local copy.
func (m *ProxyManager) Stop() {
	m.mu.Lock()
	cmd := m.cmd
	m.cmd = nil
	m.started = false
	m.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	cmd.Process.Signal(os.Interrupt)

	// Give it a moment to shut down gracefully
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		<-done // Reap the process to prevent zombies
	}
}

// WasStarted returns true if the proxy was started by this manager.
func (m *ProxyManager) WasStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

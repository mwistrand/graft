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

// ProxyManager handles the lifecycle of the copilot-api proxy server.
type ProxyManager struct {
	baseURL string
	mu      sync.Mutex
	cmd     *exec.Cmd
	started bool
	models  []provider.ModelInfo // cached models from /v1/models
}

// NewProxyManager creates a new proxy manager for the given base URL.
func NewProxyManager(baseURL string) *ProxyManager {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &ProxyManager{baseURL: baseURL}
}

// EnsureRunning checks if the proxy is running and starts it if not.
// Returns true if the proxy was started by this call (and should be stopped later).
func (m *ProxyManager) EnsureRunning(ctx context.Context, logFn func(string, ...any)) (bool, error) {
	if m.IsRunning(ctx) {
		return false, nil
	}

	m.mu.Lock()
	// Double-check after acquiring lock
	if m.cmd != nil {
		m.mu.Unlock()
		return false, nil
	}

	logFn("Starting copilot-api proxy...")

	if err := m.startLocked(ctx); err != nil {
		m.mu.Unlock()
		return false, err
	}
	m.mu.Unlock()

	logFn("Waiting for proxy to be ready (you may need to authenticate with GitHub)...")

	if err := m.WaitReady(ctx, 2*time.Minute); err != nil {
		m.Stop()
		return false, fmt.Errorf("proxy failed to start: %w", err)
	}

	logFn("Copilot proxy ready")
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

	// Try npx first (most common way to run copilot-api)
	m.cmd = exec.CommandContext(ctx, "npx", "copilot-api@latest", "start")
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

		// Check if the process died
		if m.cmd != nil && m.cmd.ProcessState != nil && m.cmd.ProcessState.Exited() {
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

// Stop terminates the proxy if it was started by this manager.
func (m *ProxyManager) Stop() {
	m.mu.Lock()
	cmd := m.cmd
	if cmd == nil || cmd.Process == nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

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
	}

	m.mu.Lock()
	m.cmd = nil
	m.started = false
	m.mu.Unlock()
}

// WasStarted returns true if the proxy was started by this manager.
func (m *ProxyManager) WasStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

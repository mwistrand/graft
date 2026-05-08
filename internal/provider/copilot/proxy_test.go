package copilot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestEnsureRunning_ConcurrentCallers verifies that many goroutines calling
// EnsureRunning against an already-up proxy all return (false, nil) and never
// trigger a subprocess start. This catches the TOCTOU regression where two
// callers could race past the fast-path check.
func TestEnsureRunning_ConcurrentCallers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer server.Close()

	pm := NewProxyManager(server.URL)
	logFn := func(string, ...any) {}

	const goroutines = 32
	var wg sync.WaitGroup
	var startedCount int32
	errs := make(chan error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			started, err := pm.EnsureRunning(context.Background(), logFn)
			if err != nil {
				errs <- err
				return
			}
			if started {
				atomic.AddInt32(&startedCount, 1)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("EnsureRunning failed: %v", err)
	}
	if got := atomic.LoadInt32(&startedCount); got != 0 {
		t.Errorf("started=true returned by %d callers, want 0 (proxy was already up)", got)
	}
	if pm.WasStarted() {
		t.Error("WasStarted=true after concurrent fast-path returns; expected false")
	}
}

// TestStop_ConcurrentWithWaitReady drives Stop() on one goroutine while
// WaitReady polls on another. Reaching the cmd-exited check while Stop is
// nil-ing out m.cmd was the original race; the test must pass under `go test -race`.
func TestStop_ConcurrentWithWaitReady(t *testing.T) {
	pm := NewProxyManager("http://127.0.0.1:1") // unreachable; IsRunning always false

	// Seed a live subprocess so cmdExited has something to inspect.
	cmd := exec.Command("sh", "-c", "sleep 5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting test subprocess: %v", err)
	}
	pm.mu.Lock()
	pm.cmd = cmd
	pm.started = true
	pm.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		// WaitReady will poll IsRunning (always false here) and call cmdExited
		// repeatedly. It should return cleanly when ctx is cancelled or the
		// process is killed by Stop().
		_ = pm.WaitReady(ctx, 5*time.Second)
	}()

	go func() {
		defer wg.Done()
		// Brief delay so WaitReady is mid-loop when Stop fires.
		time.Sleep(50 * time.Millisecond)
		pm.Stop()
	}()

	wg.Wait()

	// After Stop, cmd must be cleared; cmdExited returns false (no cmd).
	if pm.cmdExited() {
		t.Error("cmdExited true after Stop cleared m.cmd; want false")
	}
	if pm.WasStarted() {
		t.Error("WasStarted=true after Stop; want false")
	}
}

// TestStop_Idempotent verifies that Stop() leaves m.cmd nil on repeated calls
// and the second call is a no-op.
func TestStop_Idempotent(t *testing.T) {
	pm := NewProxyManager("http://127.0.0.1:1")

	cmd := exec.Command("sh", "-c", "sleep 5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting test subprocess: %v", err)
	}
	pm.mu.Lock()
	pm.cmd = cmd
	pm.started = true
	pm.mu.Unlock()

	pm.Stop()

	pm.mu.Lock()
	if pm.cmd != nil {
		pm.mu.Unlock()
		t.Fatal("m.cmd != nil after first Stop")
	}
	if pm.started {
		pm.mu.Unlock()
		t.Fatal("m.started true after first Stop")
	}
	pm.mu.Unlock()

	// Second call must be a no-op without panic.
	pm.Stop()
}

// TestEnsureRunning_StartFailure exercises the error return from tryStart when
// the subprocess cannot be launched. With PATH cleared, exec.Cmd.Start fails to
// resolve "npx" and the error must propagate without setting started.
func TestEnsureRunning_StartFailure(t *testing.T) {
	t.Setenv("PATH", "")

	pm := NewProxyManager("http://127.0.0.1:1") // unreachable; IsRunning=false
	logFn := func(string, ...any) {}

	started, err := pm.EnsureRunning(context.Background(), logFn)
	if err == nil {
		t.Fatal("expected error from EnsureRunning, got nil")
	}
	if started {
		t.Error("started=true on launch failure; want false")
	}
	if pm.WasStarted() {
		t.Error("WasStarted=true after failed start; want false")
	}
}

// TestWaitReady_ProcessExited verifies that WaitReady returns the
// "exited unexpectedly" error when the seeded subprocess has already terminated.
// ProcessState is populated by cmd.Wait, so we wait on the process before handing
// it to the manager.
func TestWaitReady_ProcessExited(t *testing.T) {
	pm := NewProxyManager("http://127.0.0.1:1") // unreachable; IsRunning=false

	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting test subprocess: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting for test subprocess: %v", err)
	}
	pm.mu.Lock()
	pm.cmd = cmd
	pm.started = true
	pm.mu.Unlock()

	err := pm.WaitReady(context.Background(), 5*time.Second)
	if err == nil {
		t.Fatal("expected error from WaitReady, got nil")
	}
	if !strings.Contains(err.Error(), "exited unexpectedly") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "exited unexpectedly")
	}
}

// hangServer returns an httptest.Server whose handler blocks until the test
// ends. The stop channel is closed in t.Cleanup before server.Close() runs,
// so handlers unblock first and Close() doesn't deadlock waiting on them.
func hangServer(t *testing.T) *httptest.Server {
	t.Helper()
	stop := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(stop)
		server.Close()
	})
	return server
}

// TestChat_HTTPClientTimeout verifies that a wedged proxy does not hang the
// CLI. We use a Provider whose http.Client has a short timeout and point it at
// a server that never responds.
func TestChat_HTTPClientTimeout(t *testing.T) {
	server := hangServer(t)

	p := &Provider{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	start := time.Now()
	_, err := p.chat(context.Background(), "test-model", "prompt", "", 100)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// Generous bound: the timeout is 100ms; allow up to 2s for slow CI.
	if elapsed > 2*time.Second {
		t.Errorf("chat() took %v with a 100ms client timeout; expected <2s", elapsed)
	}
}

// TestChat_ContextCancellation verifies that a caller-supplied context deadline
// is honored even when the client timeout is generous.
func TestChat_ContextCancellation(t *testing.T) {
	server := hangServer(t)

	p := &Provider{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := p.chat(ctx, "test-model", "prompt", "", 100)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("chat() took %v with a 100ms ctx; expected <2s", elapsed)
	}
}

package matter

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/stephens/tcc-bridge/internal/log"
)

// Process manages the Node.js Matter bridge subprocess
type Process struct {
	dir     string
	cmd     *exec.Cmd
	running bool
	mu      sync.RWMutex
}

// NewProcess creates a new process manager
func NewProcess(dir string) *Process {
	return &Process{
		dir: dir,
	}
}

// Start starts the Node.js process
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("process already running")
	}

	// Check if the bridge directory exists
	if _, err := os.Stat(p.dir); os.IsNotExist(err) {
		return fmt.Errorf("bridge directory not found: %s", p.dir)
	}

	// Check for node_modules
	nodeModules := p.dir + "/node_modules"
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		log.Warn("node_modules not found, Matter bridge may not be installed")
	}

	// Start the Node.js process
	p.cmd = exec.CommandContext(ctx, "node", "dist/index.js")
	p.cmd.Dir = p.dir

	// Set up environment
	p.cmd.Env = append(os.Environ(),
		"NODE_ENV=production",
	)

	// Capture stdout/stderr
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout: %w", err)
	}
	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr: %w", err)
	}

	// Start the process
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.running = true

	// Log output in goroutines
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Debug("[matter-bridge] %s", scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Warn("[matter-bridge] %s", scanner.Text())
		}
	}()

	// Monitor process exit
	go func() {
		err := p.cmd.Wait()
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
		if err != nil {
			log.Error("Matter bridge exited with error: %v", err)
		} else {
			log.Info("Matter bridge exited")
		}
	}()

	log.Info("Started Matter bridge process")
	return nil
}

// Stop stops the Node.js process
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		// Force kill if SIGTERM fails
		p.cmd.Process.Kill()
	}

	p.running = false
	log.Info("Stopped Matter bridge process")
	return nil
}

// IsRunning returns true if the process is running
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Restart restarts the process
func (p *Process) Restart(ctx context.Context) error {
	if err := p.Stop(); err != nil {
		return err
	}
	return p.Start(ctx)
}

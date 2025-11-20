package mdns

import (
	"context"
	"log/slog"
	"os/exec"
	"sync"
)

const (
	DnsSDBinary = "dns-sd"
)

type MacHandler struct {
	ctx       context.Context
	logger    *slog.Logger
	ipAddress string

	mu sync.Mutex

	processes map[string]context.CancelFunc
}

func NewMacHandler(ctx context.Context, logger *slog.Logger, ipAddress string) *MacHandler {
	return &MacHandler{
		ctx:       ctx,
		logger:    logger,
		ipAddress: ipAddress,
		processes: make(map[string]context.CancelFunc),
	}
}

func (mh *MacHandler) OnHostsAdded(hosts []string) {
	mh.logger.Info("Registering hosts", "hosts", hosts)

	for _, host := range hosts {
		mh.startHost(host)
	}
}

func (mh *MacHandler) OnHostsRemoved(hosts []string) {
	mh.logger.Info("Unregistering hosts", "hosts", hosts)

	for _, host := range hosts {
		mh.stopHost(host)
	}
}

func (mh *MacHandler) startHost(host string) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	if _, exists := mh.processes[host]; exists {
		mh.logger.Info("Host already registered, skipping", "host", host)
		return
	}

	hostCtx, cancel := context.WithCancel(mh.ctx)

	mh.processes[host] = cancel

	// dns-sd -P <host> _http._tcp local 443 <host> <ip>
	// Note: _tcp is generic, but requested. Usually this is _http._tcp or similar.
	args := []string{
		"-P",         // Proxy mode
		host,         // Instance Name
		"_http._tcp", // Service Type
		"local",      // Domain
		"443",        // Port
		host,         // Host Target
		mh.ipAddress, // IP Address
	}

	cmd := exec.CommandContext(hostCtx, DnsSDBinary, args...)

	go func(h string, c *exec.Cmd) {
		mh.logger.Info("Starting dns-sd process", "host", h, "args", args)

		if err := c.Run(); err != nil {
			if hostCtx.Err() == context.Canceled {
				mh.logger.Info("dns-sd process stopped (context canceled)", "host", h)
			} else {
				mh.logger.Error("dns-sd process exited with error", "host", h, "error", err)
			}
		}

		mh.mu.Lock()

		delete(mh.processes, h)
		mh.mu.Unlock()
	}(host, cmd)
}

func (mh *MacHandler) stopHost(host string) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	cancel, exists := mh.processes[host]
	if !exists {
		mh.logger.Info("Host not found in registry, processing skip", "host", host)
		return
	}

	cancel()

	delete(mh.processes, host)
}

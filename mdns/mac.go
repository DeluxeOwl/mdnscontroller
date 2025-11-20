package mdns

import "log/slog"

// MacHandler acts as the "Real" implementation for your mDNS logic
// for demonstration purposes.
type MacHandler struct{}

func NewMacHandler() *MacHandler {
	return &MacHandler{}
}

func (lp *MacHandler) OnHostsAdded(hosts []string) {
	// TODO: Insert mDNS Register Logic Here
	slog.Info("ACTION: Registering hosts", "hosts", hosts)
}

func (lp *MacHandler) OnHostsRemoved(hosts []string) {
	// TODO: Insert mDNS Unregister Logic Here
	slog.Info("ACTION: Unregistering hosts", "hosts", hosts)
}

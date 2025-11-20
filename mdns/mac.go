package mdns

import (
	"context"
	"log/slog"
)

type MacHandler struct {
	ctx context.Context
}

func NewMacHandler(ctx context.Context) *MacHandler {
	return &MacHandler{
		ctx: ctx,
	}
}

func (lp *MacHandler) OnHostsAdded(hosts []string) {
	slog.Info("ACTION: Registering hosts", "hosts", hosts)
}

func (lp *MacHandler) OnHostsRemoved(hosts []string) {
	slog.Info("ACTION: Unregistering hosts", "hosts", hosts)
}

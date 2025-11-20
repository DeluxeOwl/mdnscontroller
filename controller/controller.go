package controller

import (
	"context"
	"fmt"
	"log/slog"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const annotationKey = "mdnscontroller/enabled"

type HostHandler interface {
	OnHostsAdded(hosts []string)
	OnHostsRemoved(hosts []string)
}

type MDNSController struct {
	informerFactory informers.SharedInformerFactory
	ingressInformer cache.SharedIndexInformer
	handler         HostHandler
	logger          *slog.Logger
}

func NewMDNS(
	factory informers.SharedInformerFactory,
	handler HostHandler,
	logger *slog.Logger,
) *MDNSController {
	ingressInformer := factory.Networking().V1().Ingresses().Informer()

	c := &MDNSController{
		informerFactory: factory,
		ingressInformer: ingressInformer,
		handler:         handler,
		logger:          logger,
	}

	_, err := ingressInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAdd,
		UpdateFunc: c.onUpdate,
		DeleteFunc: c.onDelete,
	})
	if err != nil {
		logger.Error("add event handler", "err", err)
	}

	return c
}

func (c *MDNSController) Run(ctx context.Context) error {
	c.logger.Info("Starting mDNS Controller")

	c.informerFactory.Start(ctx.Done())

	c.logger.Info("Waiting for informer caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.ingressInformer.HasSynced) {
		return fmt.Errorf("sync informer cache")
	}

	c.logger.Info("Controller synced and ready")

	<-ctx.Done()
	c.logger.Info("Shutting down controller")
	return nil
}

func (c *MDNSController) onAdd(obj any) {
	ing, ok := obj.(*netv1.Ingress)
	if !ok {
		return
	}

	if !isEnabled(ing) {
		return
	}

	hosts := extractHosts(ing)
	if len(hosts) > 0 {
		c.logger.Info("Ingress added with enabled annotation", "name", ing.Name, "hosts", hosts)
		c.handler.OnHostsAdded(hosts)
	}
}

func (c *MDNSController) onUpdate(oldObj, newObj interface{}) {
	oldIng, ok1 := oldObj.(*netv1.Ingress)
	newIng, ok2 := newObj.(*netv1.Ingress)
	if !ok1 || !ok2 {
		return
	}

	wasEnabled := isEnabled(oldIng)
	isEnabled := isEnabled(newIng)

	oldHosts := extractHosts(oldIng)
	newHosts := extractHosts(newIng)

	if !wasEnabled && isEnabled {
		// Case 1: Annotation enabled (Disabled -> Enabled)
		c.logger.Info("Annotation enabled on existing ingress", "name", newIng.Name, "hosts", newHosts)
		c.handler.OnHostsAdded(newHosts)
	} else if wasEnabled && !isEnabled {
		// Case 2: Annotation disabled (Enabled -> Disabled)
		c.logger.Info("Annotation disabled on existing ingress", "name", newIng.Name)
		c.handler.OnHostsRemoved(oldHosts)
	} else if isEnabled {
		// Case 3: Still enabled, check for specific host changes
		added, removed := calculateHostDiff(oldHosts, newHosts)

		if len(added) > 0 || len(removed) > 0 {
			c.logger.Info("Hosts updated", "name", newIng.Name, "added", added, "removed", removed)

			// Unregister only the removed hosts
			if len(removed) > 0 {
				c.handler.OnHostsRemoved(removed)
			}

			// Register only the new hosts
			if len(added) > 0 {
				c.handler.OnHostsAdded(added)
			}
		}
	}
}

// calculateHostDiff returns the hosts that are in newList but not oldList (added),
// and hosts in oldList but not newList (removed).
func calculateHostDiff(oldList, newList []string) (added, removed []string) {
	oldSet := make(map[string]struct{}, len(oldList))
	for _, h := range oldList {
		oldSet[h] = struct{}{}
	}

	newSet := make(map[string]struct{}, len(newList))
	for _, h := range newList {
		newSet[h] = struct{}{}
	}

	for _, h := range newList {
		if _, exists := oldSet[h]; !exists {
			added = append(added, h)
		}
	}

	// Find removed: present in old, missing from new
	for _, h := range oldList {
		if _, exists := newSet[h]; !exists {
			removed = append(removed, h)
		}
	}

	return added, removed
}

func (c *MDNSController) onDelete(obj any) {
	ing, ok := obj.(*netv1.Ingress)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		ing, ok = tombstone.Obj.(*netv1.Ingress)
		if !ok {
			return
		}
	}

	if isEnabled(ing) {
		hosts := extractHosts(ing)
		c.logger.Info("Ingress deleted", "name", ing.Name, "hosts", hosts)
		c.handler.OnHostsRemoved(hosts)
	}
}

// isEnabled checks the annotation
func isEnabled(ing *netv1.Ingress) bool {
	if ing.Annotations == nil {
		return false
	}
	return ing.Annotations[annotationKey] == "true"
}

// extractHosts pulls hostnames from Ingress rules
func extractHosts(ing *netv1.Ingress) []string {
	var hosts []string
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	return hosts
}

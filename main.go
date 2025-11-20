package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DeluxeOwl/mdnscontroller/controller"
	"github.com/DeluxeOwl/mdnscontroller/mdns"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

func GetLocalAddressOr(logger *slog.Logger, fallback string) string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logger.Error("List interface addresses", "err", err)
		return fallback
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipv4 := ipnet.IP.To4(); ipv4 != nil {
				logger.Info("Using ip for mDNS", "ip", ipv4.String())
				return ipv4.String()
			}
		}
	}

	logger.Warn("No IPv4 addresses found")
	return fallback
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	configFlags := genericclioptions.NewConfigFlags(true)

	var ipAddress string
	cmd := &cobra.Command{
		Use:   "mdnscontroller",
		Short: "Watches ingresses for hosts and registers them.",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := configFlags.ToRESTConfig()
			if err != nil {
				logger.Error("load kubeconfig", "err", err)
				os.Exit(1)
			}

			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				logger.Error("create clientset", "err", err)
				os.Exit(1)
			}

			// Determine namespace (empty string == all namespaces)
			namespace, _, _ := configFlags.ToRawKubeConfigLoader().Namespace()

			logger.Info("Starting controller", "namespace", namespace)

			if !cmd.Flags().Changed("ip-address") {
				ipAddress = GetLocalAddressOr(logger, ipAddress)
			}

			// Create Informer Factory
			// Re-sync every 10 mins ensures the cache doesn't drift
			factory := informers.NewSharedInformerFactoryWithOptions(
				clientset,
				10*time.Minute,
				informers.WithNamespace(namespace),
			)

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			mdnsHandler := mdns.NewMacHandler(ctx)
			controller := controller.NewMDNS(factory, mdnsHandler, logger)

			// Handle crash inside the informer routines
			defer runtime.HandleCrash()

			if err := controller.Run(ctx); err != nil {
				logger.Error("Error running controller", "err", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&ipAddress, "ip-address", "127.0.0.1", "IP address to advertise (auto-detected if not specified)")
	configFlags.AddFlags(cmd.Flags())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

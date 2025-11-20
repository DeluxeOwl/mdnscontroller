
# mDNS Controller

This is a small tool that saves you from constantly editing your `/etc/hosts` file when developing with Kubernetes locally.

It connects to your cluster, watches for Ingress resources, and if it sees a specific annotation, it broadcasts that hostname on your local network using mDNS (Bonjour). Effectively, it points the Ingress hostname to your local machine's IP address automatically.

**Note:** The current implementation wraps the `dns-sd` command, so this is designed primarily for macOS.

### Running

Run the binary. By default, it looks for your kubeconfig in the standard location (`~/.kube/config`).

```bash
go run github.com/DeluxeOwl/mdnscontroller@v1.0.0
# or install it
go install github.com/DeluxeOwl/mdnscontroller@v1.0.0
```

The controller attempts to auto-detect your main IPv4 address to advertise. If it picks the wrong interface (like a Docker bridge instead of your actual LAN IP), you can force it:

```bash
mdnscontroller --ip-address 192.168.1.50
```

You can also limit it to a specific namespace if you don't want it watching the whole cluster:

```bash
mdnscontroller -n my-namespace
```

### Usage

To make the controller pick up an Ingress, just add the `mdnscontroller/enabled: "true"` annotation to it.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    mdnscontroller/enabled: "true"
spec:
  rules:
    - host: my-local-app.test
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80
```

As soon as you apply this, the logs will show the host being registered. You should immediately be able to curl or browse to `my-local-app.test` (assuming your cluster is exposing ports on your machine, generally via a LoadBalancer or NodePort mapped to localhost).

### How it works

1.  The `controller` package sets up a Kubernetes Informer to watch Ingresses.
2.  When an Ingress is added, updated, or deleted, it checks for the annotation.
3.  The `mdns` package (specifically `mac.go`) wraps the native macOS `dns-sd` binary in proxy mode (`-P`).
4.  It keeps a background process running for every active host. If you delete the Ingress, it kills the process and stops advertising.

### Not on macOS?

The code is modular. The Kubernetes controller doesn't know about `dns-sd`; it just talks to a generic `HostHandler` interface defined in `controller.go`.

```go
type HostHandler interface {
    OnHostsAdded(hosts []string)
    OnHostsRemoved(hosts []string)
}
```

If you need Linux support, you don't need to touch the controller logic. You just need to write a struct that implements those two methods (perhaps by wrapping `avahi-publish` or using a DBus library) and swap it into `main.go`.

I'm open to PRs.
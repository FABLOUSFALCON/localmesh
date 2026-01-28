# LocalMesh

> A secure, offline-first framework for building location-aware services on campus mesh networks.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Security](https://img.shields.io/badge/Security-First-green.svg)]()

## What is LocalMesh?

LocalMesh enables universities and enterprises to run secure, local-only services where **your WiFi connection determines what you can access**. No internet required. No GPS needed. Just fast, secure, location-aware services.

### Key Features

- 🔐 **Location-Based Auth** - WiFi network = identity. Connect to CS-DEPT-WIFI? Access CS resources.
- 🌐 **Zero Internet Dependency** - Everything runs on local mesh network
- 🔌 **Plugin Architecture** - Build custom services on top of the framework
- 🛡️ **Security First** - CVE-free by design, audit-ready code
- ☁️ **Cloud Sync** - Periodic backups to survive hardware failures
- ⚡ **Blazing Fast** - Local network latency, not internet latency

## Quick Start

```bash
# Install LocalMesh
go install github.com/FABLOUSFALCON/localmesh/cmd/localmesh@latest

# Initialize a new node
localmesh init

# Start the framework
localmesh start

# Check status
localmesh status
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Plugin Development](docs/plugin-development.md)
- [Security Model](docs/security.md)
- [Architecture](PLAN.md)

## Demo Plugins

| Plugin | Description | Access Level |
|--------|-------------|--------------|
| Attendance | Mark & track attendance | Department WiFi only |
| Live Lecture | Real-time lecture broadcast | General campus |
| Notices | Announcement board | General campus |

## Development

```bash
# Clone the repo
git clone https://github.com/FABLOUSFALCON/localmesh.git
cd localmesh

# Install dependencies
go mod download

# Run linter
golangci-lint run

# Run tests
go test ./...

# Build
go build -o localmesh ./cmd/localmesh
```

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](LICENSE) for details.

---

Built with 💻 by [FABLOUSFALCON](https://github.com/FABLOUSFALCON)

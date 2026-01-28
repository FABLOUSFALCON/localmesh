# Getting Started with LocalMesh

This guide will walk you through setting up LocalMesh for your campus or organization.

## Prerequisites

- Go 1.22 or later
- A local network with multicast enabled (most campus networks support this)
- One or more machines to run LocalMesh nodes

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/FABLOUSFALCON/localmesh.git
cd localmesh

# Build
make build

# Install to GOPATH/bin
make install
```

### From Release

Download the latest release from the [releases page](https://github.com/FABLOUSFALCON/localmesh/releases).

## Quick Start

### 1. Initialize a Node

```bash
# Create a new LocalMesh node
localmesh init

# This creates:
# - localmesh.yaml (configuration)
# - data/ directory (storage)
# - data/keys/ directory (encryption keys)
```

### 2. Configure Your Node

Edit `localmesh.yaml`:

```yaml
node:
  name: "main-gateway"
  role: gateway
  zone: general

gateway:
  port: 8080
  domain: "college.local"

zones:
  - id: general
    name: "Campus WiFi"
    ssids:
      - "CAMPUS-WIFI"
```

### 3. Start the Framework

```bash
localmesh start
```

### 4. Access the Dashboard

Open your browser and go to:
- http://localhost:8080 (direct IP)
- http://college.local (mDNS - if configured)

## Network Zones

LocalMesh uses **network zones** for location-based access control. Define zones based on:

1. **WiFi SSIDs** - What network the user is connected to
2. **IP Ranges** - What subnet the user's IP is in

Example zone configuration:

```yaml
zones:
  # Parent zone - general campus access
  - id: general
    name: "General Campus"
    ssids:
      - "CAMPUS-WIFI"
    ip_ranges:
      - "10.0.0.0/8"

  # Child zone - CS department (inherits from general)
  - id: cs-department
    name: "CS Department"
    ssids:
      - "CS-DEPT-WIFI"
    ip_ranges:
      - "10.10.0.0/16"
    parent: general
```

## Installing Plugins

```bash
# List available plugins
localmesh plugin list

# Install a plugin
localmesh plugin install ./plugins/attendance

# Enable a plugin
localmesh plugin enable attendance
```

## Multi-Node Setup

For larger deployments, you can run multiple LocalMesh nodes:

1. **Gateway Node** - Central router and service registry
2. **Worker Nodes** - Run plugins and services

### Gateway Node

```yaml
node:
  name: "main-gateway"
  role: gateway
```

### Worker Node

```yaml
node:
  name: "cs-node-01"
  role: worker
  zone: cs-department
```

Worker nodes automatically discover the gateway via mDNS and register their services.

## Troubleshooting

### Node Not Discovering Others

1. Ensure multicast is enabled on your network
2. Check firewall rules for mDNS (UDP port 5353)
3. Verify all nodes are on the same network segment

### Services Not Loading

```bash
# Check service health
localmesh status

# View logs
localmesh start --debug
```

### Token Issues

```bash
# Regenerate keys
localmesh keys generate

# Restart the framework
localmesh stop && localmesh start
```

## Next Steps

- [Plugin Development Guide](plugin-development.md)
- [Security Guide](security.md)
- [API Reference](api-reference.md)

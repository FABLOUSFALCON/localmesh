# LocalMesh Dashboard - Development Plan

> **Goal:** Enterprise-grade admin dashboard for LocalMesh management
> **Tech Stack:** React + TypeScript + Tailwind CSS + shadcn/ui
> **Inspiration:** Cisco Network Tools, Kubernetes Dashboard, Portainer

---

## 🎯 Why Dashboard + CLI + TUI (All Three Matter)

| Component | When to Use | Target User |
|-----------|-------------|-------------|
| **CLI (`localmesh`)** | Server startup, scripting, automation | DevOps, SysAdmins |
| **Agent (`localmesh-agent`)** | Service registration, system-level ops | Developers, Services |
| **TUI** | Quick local monitoring | Admins at terminal |
| **Dashboard** | Full management, visualization | Everyone |

**You're NOT duplicating work - you're building a complete platform!**

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                           LocalMesh Dashboard                             │
│                         (React SPA on :8080)                             │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  Services   │  │   Users &   │  │   Network   │  │  Federation │    │
│  │  Manager    │  │   Roles     │  │   Topology  │  │   Manager   │    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘    │
│         │                │                │                │            │
│         └────────────────┴────────────────┴────────────────┘            │
│                                   │                                      │
│                          HTTP REST API                                   │
│                                   │                                      │
├───────────────────────────────────┼──────────────────────────────────────┤
│                                   ▼                                      │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    LocalMesh Gateway (:8080)                        │ │
│  │                                                                      │ │
│  │  Existing:                          New (for Dashboard):            │ │
│  │  • GET  /health                     • GET  /api/v1/users            │ │
│  │  • GET  /api/v1/services            • POST /api/v1/users            │ │
│  │  • POST /api/v1/services            • GET  /api/v1/roles            │ │
│  │  • GET  /api/v1/nodes               • POST /api/v1/roles            │ │
│  │  • GET  /api/v1/stats               • GET  /api/v1/federation       │ │
│  │  • POST /auth/login                 • GET  /api/v1/alerts           │ │
│  │  • POST /auth/register              • GET  /api/v1/logs             │ │
│  │                                     • GET  /api/v1/topology         │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 📱 Dashboard Pages

### 1. **Overview / Home**
```
┌─────────────────────────────────────────────────────────────────────────┐
│  LocalMesh Dashboard                     🔔 Alerts(3) │ 👤 admin@campus │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐  │
│  │   Services   │ │    Users     │ │    Realms    │ │   Uptime     │  │
│  │     12 ✅    │ │     45       │ │      3       │ │  99.9%       │  │
│  │   2 ⚠️ 1 ❌  │ │   5 online   │ │  2 federated │ │  30d 4h 12m  │  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘  │
│                                                                         │
│  ┌─────────────────────────────────┐ ┌─────────────────────────────┐   │
│  │     Service Health Timeline     │ │      Recent Activity        │   │
│  │  ▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄   │ │  • User john joined         │   │
│  │  09:00        12:00      15:00  │ │  • api.campus.local updated │   │
│  └─────────────────────────────────┘ │  • Federation sync OK       │   │
│                                      │  • Alert: printer.local ⚠️  │   │
│  ┌─────────────────────────────────┐ └─────────────────────────────┘   │
│  │        Network Topology         │                                   │
│  │                                 │ ┌─────────────────────────────┐   │
│  │    [Main Campus]               │ │      Quick Actions          │   │
│  │         │                       │ │  [+ Register Service]       │   │
│  │    ┌────┼────┐                  │ │  [+ Add User]               │   │
│  │    ▼    ▼    ▼                  │ │  [+ Create Realm]           │   │
│  │  [Lib] [Lab] [Dorm]            │ │  [🔄 Sync Federation]       │   │
│  │                                 │ └─────────────────────────────┘   │
│  └─────────────────────────────────┘                                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2. **Services Page**
- List all registered services with health status
- Register/unregister services
- View service details (IP, port, health checks)
- Filter by realm, status, type
- Service dependency graph

### 3. **Users & Roles Page**
- CRUD users
- Assign roles (admin, user, service-account)
- Set permissions per realm/service
- Password reset
- View audit logs per user

### 4. **Realms & Federation**
- View all LocalMesh realms
- Establish/revoke trust relationships
- View cross-realm services
- Sync status and history
- Geographic distribution map

### 5. **Network Topology**
- Visual graph of services and connections
- Real-time health indicators
- Click to drill down into service
- Export topology as image/JSON

### 6. **Alerts & Logs**
- Real-time log streaming
- Alert rules configuration
- Alert history
- Filter by severity/service/realm

### 7. **Settings**
- Gateway configuration
- mDNS/DNS settings
- TLS certificates
- Backup/restore
- API keys management

---

## 🛠️ Tech Stack

| Layer | Technology | Why |
|-------|------------|-----|
| **Framework** | React 18 + Vite | Fast, modern, great ecosystem |
| **Language** | TypeScript | Type safety, better DX |
| **Styling** | Tailwind CSS | Utility-first, fast styling |
| **Components** | shadcn/ui | Beautiful, accessible, customizable |
| **State** | TanStack Query | Server state management |
| **Routing** | React Router v6 | Standard SPA routing |
| **Charts** | Recharts | React-native charting |
| **Topology** | React Flow | Network topology visualization |
| **Icons** | Lucide React | Consistent icon set |
| **Forms** | React Hook Form + Zod | Form handling + validation |

---

## 📁 Project Structure

```
web/dashboard/
├── src/
│   ├── components/
│   │   ├── ui/              # shadcn/ui components
│   │   ├── layout/          # Header, Sidebar, Footer
│   │   ├── services/        # Service-related components
│   │   ├── users/           # User management components
│   │   ├── topology/        # Network graph components
│   │   └── common/          # Shared components
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   ├── Services.tsx
│   │   ├── Users.tsx
│   │   ├── Realms.tsx
│   │   ├── Topology.tsx
│   │   ├── Alerts.tsx
│   │   └── Settings.tsx
│   ├── hooks/               # Custom React hooks
│   ├── api/                 # API client functions
│   ├── types/               # TypeScript interfaces
│   ├── lib/                 # Utility functions
│   ├── App.tsx
│   └── main.tsx
├── public/
├── index.html
├── package.json
├── tsconfig.json
├── tailwind.config.js
├── vite.config.ts
└── README.md
```

---

## 🚀 Development Phases

### Phase 1: Foundation (Week 1)
- [ ] Initialize Vite + React + TypeScript project
- [ ] Set up Tailwind CSS + shadcn/ui
- [ ] Create layout components (Sidebar, Header)
- [ ] Implement authentication flow
- [ ] Basic Dashboard page with stats cards

### Phase 2: Core Features (Week 2)
- [ ] Services page (list, register, unregister)
- [ ] Users page (CRUD, role assignment)
- [ ] API client for all endpoints
- [ ] Real-time health status updates

### Phase 3: Advanced Features (Week 3)
- [ ] Network topology visualization
- [ ] Federation management
- [ ] Alerts & logs streaming
- [ ] Settings page

### Phase 4: Polish (Week 4)
- [ ] Dark/light mode
- [ ] Responsive design (mobile support)
- [ ] Error handling & loading states
- [ ] Performance optimization
- [ ] Documentation

---

## 🔌 Backend API Requirements

New endpoints needed in `internal/gateway/router.go`:

```go
// User Management
GET    /api/v1/users           // List users
POST   /api/v1/users           // Create user
GET    /api/v1/users/{id}      // Get user
PUT    /api/v1/users/{id}      // Update user
DELETE /api/v1/users/{id}      // Delete user

// Role Management  
GET    /api/v1/roles           // List roles
POST   /api/v1/roles           // Create role
GET    /api/v1/roles/{id}      // Get role
PUT    /api/v1/roles/{id}      // Update role
DELETE /api/v1/roles/{id}      // Delete role

// Federation
GET    /api/v1/federation/realms      // List federated realms
POST   /api/v1/federation/trust       // Establish trust
DELETE /api/v1/federation/trust/{id}  // Revoke trust
GET    /api/v1/federation/sync        // Sync status

// Monitoring
GET    /api/v1/alerts          // List alerts
GET    /api/v1/logs            // Stream logs (SSE)
GET    /api/v1/topology        // Network graph data
```

---

## 📝 Notes

- Dashboard will be served from the same gateway (:8080)
- Static files embedded in Go binary using `embed`
- Authentication uses existing PASETO token system
- WebSocket/SSE for real-time updates
- All API calls through existing auth middleware

---

## 🎨 Design Inspiration

- [Kubernetes Dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/)
- [Portainer](https://www.portainer.io/)
- [Grafana](https://grafana.com/)
- [Cisco DNA Center](https://www.cisco.com/c/en/us/products/cloud-systems-management/dna-center/)
- [Cloudflare Dashboard](https://dash.cloudflare.com/)

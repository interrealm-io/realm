# realm

> The reference runtime for the InterRealm protocol.

`realm` is a standalone Go binary that runs a realm as a live process — loading its cryptographic identity, exposing capability tool endpoints over HTTP, and optionally registering its address on the [realmnet](https://realmnet.io) distributed ledger.

---

## What is a Realm Runtime?

The [realmnet ledger](https://github.com/interrealm-io/realmnet) gives a realm an **address**. The realm runtime gives it a **presence** — a running process that can be reached, identified, and interacted with by other realms.

```
realmnet ledger   →  where your realm IS  (address, public key)
realm runtime     →  what your realm DOES (capabilities, tools)
```

---

## Quick Start

### 1. Install

```bash
go install github.com/interrealm-io/realm/cmd/realm@latest
```

### 2. Generate a Keypair

```bash
realm keygen
```

### 3. Configure

Edit `realm.yaml`:

```yaml
realm:
  id: newrock.realmnet
  name: Newrock
  mode: private
  keyfile: ./keys/newrock.realmnet.pem

network:
  port: 8080

capabilities:
  enabled: true
  basePath: /capabilities
  tools:
    - name: ping
      description: Health check
      path: /ping
      method: GET
      public: true
```

### 4. Start

```bash
realm start
```

Your realm is now running at `http://localhost:8080`.

---

## Built-in Endpoints

Every realm exposes these out of the box:

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Liveness check |
| `GET /capabilities/ping` | Realm identity — ID, name, public key |
| `GET /capabilities/manifest` | Full list of exposed tool endpoints |

### Example: Ping a Realm

```bash
curl http://localhost:8080/capabilities/ping
```

```json
{
  "realmId": "newrock.realmnet",
  "name": "Newrock",
  "mode": "private",
  "publicKey": "-----BEGIN PUBLIC KEY-----...",
  "timestamp": 1741478400
}
```

### Example: Discover Capabilities

```bash
curl http://localhost:8080/capabilities/manifest
```

```json
{
  "realmId": "newrock.realmnet",
  "capabilities": [
    {
      "name": "ping",
      "description": "Health check",
      "path": "/capabilities/ping",
      "method": "GET",
      "public": true
    }
  ]
}
```

---

## CLI Reference

```bash
realm start               # start the runtime (reads realm.yaml)
realm start -c ./my.yaml  # use a custom config file
realm keygen              # generate a keypair for this realm
realm status              # show realm config and status
realm capabilities        # list configured tool endpoints
```

---

## Network Participation

By default realms run in **private mode** — no ledger registration, no public exposure.

To join the realmnet public ledger, set `mode: public` in `realm.yaml` and provide a public endpoint:

```yaml
realm:
  id: newrock.realmnet
  mode: public          # opt-in — your endpoint becomes publicly discoverable

network:
  port: 8080
  endpoint: https://realm.newrock.com
```

On startup the runtime will sign and broadcast a `REALM_REGISTERED` block to the realmnet ledger. From that point your realm is discoverable by any other realm on the network.

> See [realmnet](https://github.com/interrealm-io/realmnet) for details on what gets exposed and what stays private.

---

## Repository Structure

```
realm/
├── cmd/
│   └── realm/              # Binary entrypoint
├── internal/
│   ├── config/             # realm.yaml loading and validation
│   ├── identity/           # Keypair generation and loading
│   ├── server/             # HTTP runtime, capability routing
│   ├── registry/           # realmnet registration (coming soon)
│   └── capabilities/       # Tool handler registration (coming soon)
├── pkg/
│   └── realm/              # Public SDK
└── realm.yaml              # Example configuration
```

---

## Roadmap

- [x] realm.yaml config loading
- [x] ECDSA keypair generation
- [x] HTTP server with built-in ping and manifest endpoints
- [x] CLI — start, keygen, status, capabilities
- [ ] realmnet registration on startup (public mode)
- [ ] Inter-realm request authentication (signature verification)
- [ ] Tool handler registration API
- [ ] TLS support
- [ ] Docker image (`ghcr.io/interrealm-io/realm`)

---

## Relationship to realmnet

```
interrealm-io/realmnet   →  the ledger protocol (addresses)
interrealm-io/realm      →  the runtime (this repo)
realmtrix.com            →  enterprise platform built on both
```

---

## Contributing

```bash
git clone https://github.com/interrealm-io/realm
cd realm
go mod tidy
go run ./cmd/realm start
```

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a pull request.

---

## License

Apache 2.0 — see [LICENSE](./LICENSE)

---

> Built by [Realmtrix](https://realmtrix.com) · Protocol spec at [interrealm.io](https://interrealm.io) · Ledger at [realmnet.io](https://realmnet.io)

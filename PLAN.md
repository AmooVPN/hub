# PLAN

## Goal

Turn this project into a control hub that does not run Xray locally.
The hub should:

- store inbounds, clients, nodes, and assignments
- connect to VPS hosts over SSH only for bootstrap and recovery
- install a lightweight node agent on each VPS
- use the agent to install, configure, start, stop, and update Xray on that VPS
- keep the panel UI as the central place where admins create inbounds and clients

## Recommended Architecture

- Hub panel: central web UI + API + database only.
- Node agent: small binary/service installed on each VPS.
- Bootstrap path: admin adds SSH credentials, hub connects once, installs agent, registers the node.
- Runtime path: hub talks to agents over authenticated API, not SSH.
- Node responsibility: own the local Xray install, config, process lifecycle, logs, and traffic stats.

## Data Model

- Node: host, SSH bootstrap data, agent URL, auth material, status, version.
- Inbound: logical inbound owned by the hub.
- Client: logical client owned by the hub.
- Assignment: links an inbound/client to one or more nodes.
- Per-node runtime state: generated Xray config, service status, version, stats.

## Build Phases

1. Split the current server into hub-only and node-agent responsibilities.
2. Add node CRUD in the panel for SSH bootstrap credentials and agent status.
3. Implement the node agent binary and install/update flow.
4. Make inbound/client creation sync to selected nodes through the agent API.
5. Make node status, logs, and traffic flow back to the hub.
6. Remove any assumption that the hub runs a local Xray process.
7. Update docs, scripts, Docker, and release packaging for the new architecture.

### Current progress

- Phase 1 is started: the binary now has an explicit `agent` mode that serves API-only routes and requires a bootstrap token.

## Acceptance Criteria

- The hub can manage multiple VPS nodes from one panel.
- The hub does not need a local Xray daemon.
- Creating an inbound or client in the hub can deploy it to selected nodes.
- Each node can install and manage its own Xray instance through the agent.

## Notes

- Keep the hub as the source of truth.
- Prefer idempotent node operations.
- Use SSH only for bootstrap, repair, or agent reinstallation.

# Event Service

An HTTP service for asynchronous event processing with idempotency guarantees, packaged as a container and deployable to Azure with Terraform.

## What This Service Does

The service provides an HTTP API to accept events, process them asynchronously in the background, and track their status. It implements basic idempotency to prevent duplicate processing of events with the same ID.

**Key Features:**
- Accept events via HTTP POST
- Queue events for background processing
- Track event status (accepted/processed)
- Health and readiness checks
- Graceful shutdown handling
- **Containerized** — a multi-stage [`Dockerfile`](Dockerfile) produces a small, non-root Alpine image with a built-in health check
- **Infrastructure as code** — [`terraform/`](terraform/) provisions the service on Azure Container Apps (Consumption plan), sized to stay inside the Azure free grant

## Repository Layout at a Glance

| Path | Purpose |
|------|---------|
| `main.go`, `internal/` | The Go application (HTTP server + background worker) |
| [`Dockerfile`](Dockerfile) | Builds the runnable container image for the service |
| [`.dockerignore`](.dockerignore) | Keeps build context small and prevents host binaries/secrets leaking into the image |
| [`terraform/`](terraform/) | Terraform config that deploys the container image to Azure Container Apps |
| `insomnia-collection.json` | Importable API request collection for manual testing |

## Running Locally

### Prerequisites
- Go 1.22 or later

### Start the service

```bash
go run ./...
```

The service will start on port 8080 by default.

### Configuration

The service can be configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `ENV` | `dev` | Environment (dev/staging/prod) |
| `PROCESSING_DELAY_MS` | `1000` | Simulated processing delay in milliseconds |

Example with custom configuration:

```bash
PORT=3000 ENV=staging PROCESSING_DELAY_MS=500 go run ./...
```

## Frontend Dashboard

A minimal web dashboard is available at the root URL when you start the service:

```
http://127.0.0.1:8080/
```

The dashboard provides:
- Real-time service health and worker status monitoring
- Interactive form to submit new events
- Live event list showing all events and their status (accepted/processed)
- Auto-refresh every 2 seconds to display updates

Simply open the URL in your browser to visualize and interact with the event service.

## API Endpoints

### GET /

Serves the frontend dashboard HTML page.

### GET /events

Lists all events currently stored in the service.

**Response:**
```json
[
  {
    "event_id": "evt_123",
    "payload": {
      "any": "data"
    },
    "status": "processed"
  }
]
```

Always returns `200 OK` with an array of events.

### POST /events

Accepts an event for processing.

**Request:**
```json
{
  "event_id": "evt_123",
  "payload": {
    "any": "data",
    "goes": "here"
  }
}
```

**Responses:**
- `202 Accepted` - Event accepted and queued for processing
- `409 Conflict` - Event with this ID already exists
- `400 Bad Request` - Invalid request body or missing event_id

### GET /health

Returns service health status.

**Response:**
```json
{
  "status": "ok",
  "uptime": "5m32s"
}
```

Always returns `200 OK`.

### GET /ready

Returns readiness status (whether the background worker is running).

**Response (Ready):**
```json
{
  "status": "ready",
  "ready": true
}
```
Returns `200 OK` when ready.

**Response (Not Ready):**
```json
{
  "status": "not ready",
  "ready": false
}
```
Returns `503 Service Unavailable` when not ready.

## Testing the Service

### Using Insomnia (Recommended)

An Insomnia collection is included for easy API testing:

1. Install [Insomnia](https://insomnia.rest/download) if you haven't already
2. Import the collection: `File > Import` and select `insomnia-collection.json`
3. The collection includes pre-configured requests for all endpoints:
   - `GET /health` - Check service health
   - `GET /ready` - Check worker readiness
   - `POST /events - New Event` - Create a new event (returns 202)
   - `POST /events - Duplicate Event` - Test idempotency (returns 409)
   - `POST /events - Event 2, 3` - Additional test events
   - `POST /events - Missing event_id` - Test validation (returns 400)
   - `POST /events - Invalid JSON` - Test error handling (returns 400)

The base URL is configured as `http://127.0.0.1:8080`.

### Using curl

**Accept an event:**
```bash
curl -X POST http://127.0.0.1:8080/events \
  -H "Content-Type: application/json" \
  -d '{"event_id": "evt_001", "payload": {"test": "data"}}'
```

**Try to send the same event again (should return 409):**
```bash
curl -X POST http://127.0.0.1:8080/events \
  -H "Content-Type: application/json" \
  -d '{"event_id": "evt_001", "payload": {"test": "data"}}'
```

**Check health:**
```bash
curl http://127.0.0.1:8080/health
```

**Check readiness:**
```bash
curl http://127.0.0.1:8080/ready
```

**Stop the service:**
Press `Ctrl+C` in the terminal where the service is running.

## Running with Docker

### What the [`Dockerfile`](Dockerfile) does

The [`Dockerfile`](Dockerfile) is a **multi-stage build**. It turns the Go source in this repo into a self-contained, production-shaped container image without shipping the Go toolchain or any build tooling in the final artifact.

**Stage 1 — `build` (`golang:1.22-alpine`)**

| Step | Why |
|------|-----|
| `COPY go.mod go.sum* ./` then `go mod download` | Dependencies are resolved in their own layer, so Docker only re-downloads them when `go.mod`/`go.sum` change — not on every source edit. `go.sum*` tolerates the file being absent (this service currently has no external deps). |
| `COPY . .` then `go build` | Compiles the service. |
| `CGO_ENABLED=0 GOOS=linux` | Produces a **statically linked** Linux binary with no libc dependency, so it can run on a minimal base image. |
| `-trimpath -ldflags="-s -w"` | Strips local filesystem paths and debug/symbol tables — smaller binary, no leaking of build-host paths. |

**Stage 2 — `runtime` (`alpine:3.20`)**

| Step | Why |
|------|-----|
| `apk add ca-certificates` | Root CA bundle so the service can make outbound HTTPS calls if needed. |
| `addgroup`/`adduser` + `USER app` | The container runs as an **unprivileged user**, not root — standard container hardening. |
| `COPY --from=build /out/event-service` | Only the compiled binary crosses over from stage 1; the ~300 MB Go toolchain is left behind. Final image is a few tens of MB. |
| `ENV PORT / ENV / PROCESSING_DELAY_MS` | Bakes in the same defaults the app reads (see `internal/app`), overridable at `docker run` time with `-e`. |
| `EXPOSE 8080` | Documents the listening port. It is static — if you override `PORT`, publish that port explicitly. |
| `HEALTHCHECK … wget … /health` | Container-level liveness probe. Docker/orchestrators mark the container unhealthy if `/health` stops returning `200`. Uses BusyBox `wget` (already in Alpine) so no `curl` needs bundling. |
| `ENTRYPOINT ["/app/event-service"]` | The container *is* the service; no shell wrapper. |

### What [`.dockerignore`](.dockerignore) does

Excludes files from the build context that Docker sends to the daemon. It keeps builds fast and, more importantly, **prevents host artifacts leaking into the image** — notably the prebuilt macOS binary named `event-service`, `.git/`, editor folders, and the docs. The image is built purely from source inside the `build` stage.

### Build and run

```bash
# Build the image (run from the repo root, where the Dockerfile lives)
docker build -t event-service:local .

# Run it, publishing the port
docker run --rm -p 8080:8080 event-service:local

# Override configuration at runtime
docker run --rm -p 9000:9000 \
  -e PORT=9000 -e ENV=staging -e PROCESSING_DELAY_MS=500 \
  event-service:local

# Check container health status
docker inspect --format '{{.State.Health.Status}}' <container-id>
```

The dashboard and all API endpoints work exactly as they do when running with `go run` — see the sections above.

## Deploying to Azure with Terraform

The [`terraform/`](terraform/) directory provisions the service on **Azure Container Apps (Consumption plan)**. Azure Container Apps is effectively "Kubernetes-as-a-service" — you supply a container image and declarative config for scaling, ingress, configuration, secrets, and health probes, and the platform runs it. Every resource is chosen to sit inside Azure's **free monthly grant**.

> Terraform does **not** build the image. You build it from the [`Dockerfile`](Dockerfile) above, push it to a registry the Container App can pull from (a **public** Docker Hub / GHCR repo to stay free — Azure Container Registry has no free tier), and pass the image reference to Terraform via `var.image`.

### What each Terraform file is for

| File | What it declares / does |
|------|-------------------------|
| [`terraform/providers.tf`](terraform/providers.tf) | Pins Terraform (`>= 1.5`) and the `hashicorp/azurerm` provider (`~> 4.0`). Configures the provider to authenticate from your `az login` session (no secrets in code), targets the free-tier subscription, and auto-registers the two Azure resource providers this config needs (`Microsoft.App`, `Microsoft.OperationalInsights`). Contains a commented-out `backend "azurerm"` block to switch state from local to remote when the config is shared. |
| [`terraform/variables.tf`](terraform/variables.tf) | Every input, with types, defaults, and validation rules. Key ones: `env` (`dev`/`staging`/`prod`, surfaced to the app as `ENV`), `replicas` (min running replicas; `0` = scale-to-zero to stay free), `max_replicas` (autoscale ceiling), `image` (**required, no default** — your pushed image reference), `container_port`/`processing_delay_ms` (plain app config), `dummy_secret_data` (placeholder secret values), `cpu`/`memory` (smallest Consumption combo), and Log Analytics retention/quota caps. Validation blocks reject bad values before any API call. |
| [`terraform/main.tf`](terraform/main.tf) | The actual resources: a **resource group**, a **Log Analytics workspace** (log sink, hard daily-quota-capped so it cannot bill), a **Container Apps environment** wired to that workspace, and the **Container App** itself. The Container App block maps directly onto Kubernetes concepts: `template` + `min/max_replicas` = Deployment, `ingress` = Service (public HTTPS FQDN), plain `env {}` entries = ConfigMap, `secret {}` + secret-backed `env {}` = Secret, and `liveness_probe` (`/health`) / `readiness_probe` (`/ready`) / `startup_probe` (`/health`, cold-start grace) = the pod probes. An HTTP scale rule scales on concurrent requests; a `precondition` enforces `max_replicas >= replicas`. Optional `dynamic` blocks add private-registry auth only when `registry_server` is set. |
| [`terraform/outputs.tf`](terraform/outputs.tf) | What you get back after `apply`: the resource group name, region, Container Apps environment and app names, the app's public **FQDN** and base **URL**, ready-made `health_url` / `ready_url`, and a copy-paste `smoke_test` that curls `/health`, `/ready`, and `POST /events`. |
| [`terraform/terraform.tfvars.example`](terraform/terraform.tfvars.example) | A template you copy to `terraform.tfvars` (which is gitignored) and edit. Shows every commonly-set variable with sensible free-tier values, plus the `docker build` / `docker push` commands to produce the `image` value. |
| [`terraform/.gitignore`](terraform/.gitignore) | Keeps Terraform noise and sensitive material out of git: `.terraform/`, the provider lock file, **all `*.tfstate`** (state can contain secrets), plan files, crash logs, and **`terraform.tfvars`** / `*.auto.tfvars` (your real values). |
| [`terraform/README.md`](terraform/README.md) | Deep-dive on the deployment: the Kubernetes↔Container Apps mapping, why each resource is free, the full variable reference, and step-by-step deploy/destroy commands. |

### Deploy

```bash
# 1. Authenticate
az login
az account set --subscription <subscription-id>

# 2. Build from the repo Dockerfile and push to a PUBLIC registry
docker build -t docker.io/<you>/event-service:1.0.0 .
docker push  docker.io/<you>/event-service:1.0.0

# 3. Configure and apply
cd terraform
cp terraform.tfvars.example terraform.tfvars   # set image, env, replicas
terraform init
terraform plan  -var 'image=docker.io/<you>/event-service:1.0.0'
terraform apply -var 'image=docker.io/<you>/event-service:1.0.0'

# 4. Smoke test the live service
terraform output -raw app_url
terraform output smoke_test

# 5. Tear everything down
terraform destroy
```

See [`terraform/README.md`](terraform/README.md) for the free-tier rationale, the full variable table, and notes on scale-to-zero behaviour, remote state, and real secret management.

## Project Structure

```
event-service/
├── go.mod                          # Go module definition
├── main.go                         # Application entry point, signal handling
├── insomnia-collection.json        # Insomnia API client collection
├── internal/
│   ├── app/
│   │   └── app.go                  # HTTP server, handlers, config
│   ├── model/
│   │   └── model.go                # Request/response types, event model
│   ├── store/
│   │   └── store.go                # In-memory idempotency store
│   └── worker/
│       └── worker.go               # Background event processor
├── Dockerfile                      # Multi-stage build -> small non-root Alpine image w/ HEALTHCHECK
├── .dockerignore                   # Excludes host binaries, .git, docs from the build context
├── .gitignore                      # Excludes compiled binaries and editor/OS cruft
├── terraform/                      # Infrastructure as code: deploy to Azure Container Apps
│   ├── providers.tf                # Terraform + azurerm provider, subscription, auth, backend
│   ├── variables.tf                # All inputs (env, replicas, image, sizing, ...) with validation
│   ├── main.tf                     # Resource group, Log Analytics, Container Apps env, Container App
│   ├── outputs.tf                  # App FQDN / URLs + copy-paste smoke test
│   ├── terraform.tfvars.example    # Copy to terraform.tfvars and edit
│   ├── .gitignore                  # Excludes .terraform/, *.tfstate, terraform.tfvars
│   └── README.md                   # Deployment deep-dive (free-tier rationale, deploy steps)
└── README.md
```

## Important Note: Not Production-Ready

**This service is intentionally not production-ready.** It started as starter code for a technical assessment. The packaging and deployment gaps have since been addressed:

### Addressed
- **Containerization** — multi-stage [`Dockerfile`](Dockerfile): static Linux binary, non-root user, `HEALTHCHECK`, runtime-overridable config
- **Infrastructure as code** — [`terraform/`](terraform/): Azure Container Apps deployment with ingress, autoscaling, config/secrets, and liveness/readiness/startup probes
- **Log sink** — a Log Analytics workspace is provisioned for the Container Apps environment (application logging inside the service is still basic — see below)

### Still open
- **No persistence**: All event state is stored in memory and will be lost on restart
- **No structured logging**: The app uses basic `log.Printf` statements (logs are collected by the platform but are unstructured)
- **No application metrics or tracing**: No instrumentation beyond health endpoints and platform logs
- **Limited error handling**: Basic error responses without detailed error types
- **No rate limiting**: No protection against traffic spikes
- **No authentication/authorization**: Endpoints are completely open
- **In-process worker only**: The background worker runs in the same process as the HTTP server; no distributed queue, so with `replicas = 0` (scale-to-zero) the worker only runs while HTTP traffic keeps a replica alive
- **No dead letter queue**: Failed events are not captured or retried
- **Secrets are placeholders**: `dummy_secret_data` in Terraform is not wired to Azure Key Vault
- **Local Terraform state**: switch to a remote `azurerm` backend before sharing

Candidates are expected to identify and address these gaps as part of the technical assessment.

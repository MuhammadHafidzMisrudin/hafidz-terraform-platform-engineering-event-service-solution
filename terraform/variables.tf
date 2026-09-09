# ---------------------------------------------------------------------------
# Azure subscription / location
# ---------------------------------------------------------------------------
variable "subscription_id" {
  description = "Azure subscription ID to deploy into (free-tier subscription)."
  type        = string
  default     = "ddb0c900-8251-41b7-aa03-38b51de2f598"
}

variable "location" {
  description = "Azure region. Keep one region to stay inside free-grant limits."
  type        = string
  default     = "eastus"
}

# ---------------------------------------------------------------------------
# Naming
# ---------------------------------------------------------------------------
variable "name" {
  description = "Base name used to derive every resource name."
  type        = string
  default     = "event-service"
}

# ---------------------------------------------------------------------------
# Required assessment variables: env + replicas
# ---------------------------------------------------------------------------
variable "env" {
  description = "Deployment environment. Surfaced to the app as the ENV variable."
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.env)
    error_message = "env must be one of: dev, staging, prod."
  }
}

variable "replicas" {
  description = <<-EOT
    Minimum number of running container replicas.
      0 = scale-to-zero -> stays within the Container Apps monthly free grant,
          but the background worker only runs while HTTP traffic keeps a
          replica alive.
      1 = always-on -> worker runs continuously; may exceed the free vCPU-second
          grant if left running all month.
  EOT
  type        = number
  default     = 0

  validation {
    condition     = var.replicas >= 0 && var.replicas <= 20
    error_message = "replicas must be between 0 and 20."
  }
}

variable "max_replicas" {
  description = "Upper bound for autoscaling (HTTP-driven)."
  type        = number
  default     = 3

  validation {
    condition     = var.max_replicas >= 1 && var.max_replicas <= 20
    error_message = "max_replicas must be between 1 and 20."
  }
}

# ---------------------------------------------------------------------------
# Container image
# ---------------------------------------------------------------------------
variable "image" {
  description = <<-EOT
    REQUIRED. Fully-qualified image reference built from the repo Dockerfile and
    pushed to a registry the Container App can reach. Use a PUBLIC registry
    (Docker Hub / GHCR) to stay free - Azure Container Registry has no free tier.
    Example: docker.io/<your-user>/event-service:1.0.0
    Pass it via terraform.tfvars, -var, or TF_VAR_image.
  EOT
  type        = string
  # No default: force the caller to supply a real image they have pushed.

  validation {
    condition     = can(regex("^[a-z0-9.-]+(:[0-9]+)?/[^:@]+(:[^:@/]+|@sha256:[0-9a-f]{64})$", var.image))
    error_message = "image must be a fully-qualified reference like 'registry/repo:tag' (e.g. docker.io/you/event-service:1.0.0)."
  }
}

variable "registry_server" {
  description = "Private registry host (e.g. ghcr.io). Leave null for a public image."
  type        = string
  default     = null
}

variable "registry_username" {
  description = "Username for the private registry (when registry_server is set)."
  type        = string
  default     = null
}

variable "registry_password" {
  description = "Password/token for the private registry (when registry_server is set)."
  type        = string
  default     = null
  sensitive   = true
}

# ---------------------------------------------------------------------------
# App configuration (non-secret -> Container App env vars == k8s ConfigMap)
# ---------------------------------------------------------------------------
variable "container_port" {
  description = "Port the service listens on (app reads PORT)."
  type        = number
  default     = 8080
}

variable "processing_delay_ms" {
  description = "Simulated worker processing delay -> PROCESSING_DELAY_MS."
  type        = number
  default     = 1000
}

# ---------------------------------------------------------------------------
# Secret (dummy placeholder -> Container App secrets == k8s Secret)
# ---------------------------------------------------------------------------
variable "dummy_secret_data" {
  description = <<-EOT
    Placeholder secret values, mounted as env vars in the container.
    Dummy only - in real use source these from Azure Key Vault references and
    never commit them.
  EOT
  type        = map(string)
  default = {
    API_KEY       = "dummy-api-key-do-not-use-in-prod"
    WEBHOOK_TOKEN = "dummy-webhook-token"
  }
}

# ---------------------------------------------------------------------------
# Container sizing (Consumption plan allowed combinations)
# ---------------------------------------------------------------------------
variable "cpu" {
  description = "vCPU per replica. Smallest (0.25) stretches the free grant furthest."
  type        = number
  default     = 0.25

  validation {
    condition     = contains([0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2], var.cpu)
    error_message = "cpu must be one of 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2."
  }
}

variable "memory" {
  description = "Memory per replica. Must pair with cpu (~2Gi per vCPU), e.g. 0.25 -> 0.5Gi."
  type        = string
  default     = "0.5Gi"

  validation {
    condition     = can(regex("^[0-9]+(\\.[0-9]+)?Gi$", var.memory))
    error_message = "memory must look like '0.5Gi', '1Gi', '2Gi'."
  }
}

# ---------------------------------------------------------------------------
# Observability (Log Analytics free tier: 5 GB/month ingest, 31-day retention)
# ---------------------------------------------------------------------------
variable "log_analytics_retention_days" {
  description = "Log Analytics retention. 30 keeps it inside the free retention window."
  type        = number
  default     = 30

  validation {
    condition     = var.log_analytics_retention_days >= 30 && var.log_analytics_retention_days <= 730
    error_message = "log_analytics_retention_days must be between 30 and 730."
  }
}

variable "log_analytics_daily_quota_gb" {
  description = "Hard daily ingestion cap (GB) so logs cannot generate a bill."
  type        = number
  default     = 0.5
}

# ---------------------------------------------------------------------------
# Ingress / misc
# ---------------------------------------------------------------------------
variable "ingress_external" {
  description = "true = public HTTPS FQDN; false = internal to the environment only."
  type        = bool
  default     = true
}

variable "tags" {
  description = "Extra tags merged onto every resource."
  type        = map(string)
  default     = {}
}

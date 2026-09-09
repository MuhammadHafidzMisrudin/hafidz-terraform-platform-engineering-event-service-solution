locals {
  suffix = "${var.name}-${var.env}"

  common_tags = merge({
    application = var.name
    environment = var.env
    managed-by  = "terraform"
    free-tier   = "true"
  }, var.tags)

  # Non-secret configuration -> Container App env vars.
  # This is the Azure equivalent of the Kubernetes ConfigMap and matches the
  # three variables the Go service reads (see app.LoadConfig).
  config_env = {
    PORT                = tostring(var.container_port)
    ENV                 = var.env
    PROCESSING_DELAY_MS = tostring(var.processing_delay_ms)
  }

  # Container App secret names must be lowercase / dash-separated.
  secret_name = { for k, v in var.dummy_secret_data : k => lower(replace(k, "_", "-")) }

  use_private_registry = var.registry_server != null
}

# ---------------------------------------------------------------------------
# Resource group - free (no charge for the group itself)
# ---------------------------------------------------------------------------
resource "azurerm_resource_group" "this" {
  name     = "rg-${local.suffix}"
  location = var.location
  tags     = local.common_tags
}

# ---------------------------------------------------------------------------
# Log Analytics workspace - Container Apps environment log sink.
# Free tier: 5 GB/month ingestion + 31-day retention. daily_quota_gb caps
# anything beyond that so it can never bill.
# ---------------------------------------------------------------------------
resource "azurerm_log_analytics_workspace" "this" {
  name                = "log-${local.suffix}"
  resource_group_name = azurerm_resource_group.this.name
  location            = azurerm_resource_group.this.location
  sku                 = "PerGB2018"
  retention_in_days   = var.log_analytics_retention_days
  daily_quota_gb      = var.log_analytics_daily_quota_gb
  tags                = local.common_tags
}

# ---------------------------------------------------------------------------
# Container Apps environment - Consumption only.
# No fixed hourly charge; usage is billed per vCPU-second / GiB-second with a
# monthly free grant (180k vCPU-s, 360k GiB-s, 2M requests) per subscription.
# ---------------------------------------------------------------------------
resource "azurerm_container_app_environment" "this" {
  name                       = "cae-${local.suffix}"
  resource_group_name        = azurerm_resource_group.this.name
  location                   = azurerm_resource_group.this.location
  log_analytics_workspace_id = azurerm_log_analytics_workspace.this.id
  tags                       = local.common_tags
}

# ---------------------------------------------------------------------------
# Container App - the workload itself.
#   Deployment  -> template + replica bounds
#   Service     -> ingress (public HTTPS FQDN)
#   ConfigMap   -> plain env vars from local.config_env
#   Secret      -> secret blocks + secret-backed env vars
#   Probes      -> liveness (/health), readiness (/ready), startup (/health)
# HTTP server and background worker run in one process, so one app covers both.
# ---------------------------------------------------------------------------
resource "azurerm_container_app" "this" {
  name                         = "ca-${local.suffix}"
  container_app_environment_id = azurerm_container_app_environment.this.id
  resource_group_name          = azurerm_resource_group.this.name
  revision_mode                = "Single"
  tags                         = local.common_tags

  # --- Secrets (dummy) --------------------------------------------------------
  dynamic "secret" {
    for_each = var.dummy_secret_data
    content {
      name  = local.secret_name[secret.key]
      value = secret.value
    }
  }

  # Private-registry password, only when a private registry is configured.
  dynamic "secret" {
    for_each = local.use_private_registry && var.registry_password != null ? { "registry-password" = var.registry_password } : {}
    content {
      name  = secret.key
      value = secret.value
    }
  }

  dynamic "registry" {
    for_each = local.use_private_registry ? [1] : []
    content {
      server               = var.registry_server
      username             = var.registry_username
      password_secret_name = "registry-password"
    }
  }

  # --- Ingress (k8s Service equivalent) ------------------------------------
  ingress {
    external_enabled           = var.ingress_external
    target_port                = var.container_port
    transport                  = "auto"
    allow_insecure_connections = false

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }

  # --- Workload / scaling (k8s Deployment equivalent) --------------------------
  template {
    min_replicas = var.replicas
    max_replicas = var.max_replicas

    http_scale_rule {
      name                = "http-requests"
      concurrent_requests = 50
    }

    container {
      name   = var.name
      image  = var.image
      cpu    = var.cpu
      memory = var.memory

      # Plain configuration (ConfigMap equivalent).
      dynamic "env" {
        for_each = local.config_env
        content {
          name  = env.key
          value = env.value
        }
      }

      # Secret-backed configuration (Secret equivalent).
      dynamic "env" {
        for_each = var.dummy_secret_data
        content {
          name        = env.key
          secret_name = local.secret_name[env.key]
        }
      }

      # Liveness: /health returns 200 while the process is alive.
      # Failure -> the platform restarts the replica.
      liveness_probe {
        transport               = "HTTP"
        path                    = "/health"
        port                    = var.container_port
        initial_delay           = 3
        interval_seconds        = 10
        timeout                 = 2
        failure_count_threshold = 3
      }

      # Readiness: /ready returns 503 until the background worker is running.
      # Failure -> replica is pulled out of the ingress rotation.
      readiness_probe {
        transport               = "HTTP"
        path                    = "/ready"
        port                    = var.container_port
        interval_seconds        = 5
        timeout                 = 2
        failure_count_threshold = 3
        success_count_threshold = 1
      }

      # Startup: give a cold container time to come up before liveness bites.
      startup_probe {
        transport               = "HTTP"
        path                    = "/health"
        port                    = var.container_port
        interval_seconds        = 5
        timeout                 = 3
        failure_count_threshold = 30
      }
    }
  }

  lifecycle {
    precondition {
      condition     = var.max_replicas >= max(var.replicas, 1)
      error_message = "max_replicas must be >= replicas (and at least 1)."
    }
  }
}

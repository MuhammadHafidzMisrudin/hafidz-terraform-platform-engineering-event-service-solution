# Terraform - deploy to Azure (free tier)

Provisions the Event Service on **Azure Container Apps (Consumption plan)** in the
free-tier subscription `ddb0c900-8251-41b7-aa03-38b51de2f598`.

Container Apps is "Kubernetes-as-a-service", so the concepts from the earlier
Kubernetes skeleton map straight across:

| Kubernetes | Azure Container Apps |
|---|---|
| Deployment | `azurerm_container_app.template` + `min_replicas` / `max_replicas` |
| Service | `ingress` (public HTTPS FQDN) |
| ConfigMap | plain `env {}` entries from `local.config_env` |
| Secret | `secret {}` blocks + secret-backed `env {}` entries |
| Liveness probe `/health` | `liveness_probe` |
| Readiness probe `/ready` | `readiness_probe` |
| — | `startup_probe` (cold-start grace) |

## Why these resources are free

| Resource | Free basis |
|---|---|
| Resource group | No charge. |
| Log Analytics workspace (`PerGB2018`) | 5 GB/month ingestion + 31-day retention free; `daily_quota_gb = 0.5` hard-caps the rest. |
| Container Apps environment (Consumption) | No fixed/hourly charge. |
| Container App | Monthly free grant per subscription: **180k vCPU-seconds, 360k GiB-seconds, 2M requests**. `replicas = 0` (scale-to-zero) keeps usage inside it. |

> Azure Container Registry has **no** free tier - push the image to a **public**
> Docker Hub / GHCR repo instead (`var.image`).

## Files

| File | Contents |
|---|---|
| `providers.tf` | Terraform + `azurerm ~> 4.0`, subscription, provider registration. |
| `variables.tf` | All inputs, incl. required `env` and `replicas`. |
| `main.tf` | Resource group, Log Analytics, Container Apps environment, Container App. |
| `outputs.tf` | App FQDN / URLs + a copy-paste smoke test. |
| `terraform.tfvars.example` | Copy to `terraform.tfvars`. |

## Key variables

| Name | Default | Notes |
|---|---|---|
| `env` | `dev` | `dev` / `staging` / `prod`. Sets the `ENV` env var + tags. |
| `replicas` | `0` | Min running replicas. `0` = scale-to-zero (free); `1` = worker always on. |
| `max_replicas` | `3` | Autoscale ceiling (HTTP-driven). |
| `image` | _(required, no default)_ | Your image, pushed to a public registry. Set via `terraform.tfvars`, `-var`, or `TF_VAR_image`. |
| `subscription_id` | the assessment sub | |
| `container_port` | `8080` | App `PORT`. |
| `processing_delay_ms` | `1000` | App `PROCESSING_DELAY_MS`. |
| `cpu` / `memory` | `0.25` / `0.5Gi` | Smallest Consumption combo. |
| `dummy_secret_data` | 2 dummy keys | Mounted as env vars via Container App secrets. |
| `registry_server/username/password` | `null` | Set only for a private image. |

## Deploy

```bash
# 1. Auth
az login
az account set --subscription ddb0c900-8251-41b7-aa03-38b51de2f598

# 2. Build the image from the repo Dockerfile and push to a PUBLIC registry
#    (run from the repo root - the Dockerfile is one level up from ./terraform)
docker build -t docker.io/<you>/event-service:1.0.0 ..
docker push  docker.io/<you>/event-service:1.0.0

# 3. Configure + apply
cd terraform
cp terraform.tfvars.example terraform.tfvars   # set image, env, replicas

terraform init
terraform plan  -var 'image=docker.io/<you>/event-service:1.0.0'
terraform apply -var 'image=docker.io/<you>/event-service:1.0.0'

# 4. Smoke test
terraform output -raw app_url
terraform output smoke_test

# 5. Tear down (removes the whole resource group)
terraform destroy
```

## Notes

- **Scale-to-zero vs the worker:** with `replicas = 0` the container only runs
  while HTTP traffic keeps a replica alive; the in-process background worker
  pauses when it scales to zero and resumes on the next request. Set
  `replicas = 1` for a continuously-running worker (watch the free vCPU-second
  grant if you leave it up all month).
- **State is local.** Uncomment the `backend "azurerm"` block in `providers.tf`
  before sharing.
- **Secrets are dummies.** For real values use Azure Key Vault references instead
  of `dummy_secret_data`, and never commit `terraform.tfvars`.
- **Provider registration:** first `apply` registers `Microsoft.App` and
  `Microsoft.OperationalInsights` automatically (may take a few minutes).

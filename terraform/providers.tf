terraform {
  required_version = ">= 1.5"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }

  # Local state for the skeleton. For shared use, switch to an azurerm backend:
  #
  # backend "azurerm" {
  #   resource_group_name  = "rg-tfstate"
  #   storage_account_name = "sttfstateeventsvc"
  #   container_name        = "tfstate"
  #   key                  = "event-service.tfstate"
  # }
}

provider "azurerm" {
  # Free-tier subscription this deploys into.
  subscription_id = var.subscription_id

  # Auth comes from `az login` (Azure CLI). No secrets in code.
  features {
    resource_group {
      # Let `terraform destroy` remove the RG even if it still holds resources.
      prevent_deletion_if_contains_resources = false
    }
  }

  # Register only the resource providers this config needs, on first use.
  resource_provider_registrations = "core"
  resource_providers_to_register = [
    "Microsoft.App",                 # Azure Container Apps
    "Microsoft.OperationalInsights", # Log Analytics
  ]
}

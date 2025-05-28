variable "resource_group_location" {
  type        = string
  default     = "eastus"
  description = "Location of the resource group."
}

variable "resource_group_name_prefix" {
  type        = string
  default     = "rg"
  description = "Prefix of the resource group name that's combined with a random ID so name is unique in your Azure subscription."
}

variable "username_production" {
  type        = string
  description = "The username for the local account that will be created on the new VM."
  default     = "azureadmin"
}

variable "password_production" {
  type        = string
  description = "The password for the local account that will be created on the new VM."
  default     = "SuperSecurePassword"
}

variable "username_development" {
  type        = string
  description = "The username for the local account that will be created on the new VM."
  default     = "azureadmin"
}

variable "password_development" {
  type        = string
  description = "The password for the local account that will be created on the new VM."
  default     = "SuperSecurePassword"
}
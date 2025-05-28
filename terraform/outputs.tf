output "resource_group_name" {
  value = azurerm_resource_group.pso_rg.name
}

output "public_ip_address_prod" {
  value = azurerm_linux_virtual_machine.pso_vm_prod.public_ip_address
}

output "public_ip_address_dev" {
  value = azurerm_linux_virtual_machine.pso_vm_dev.public_ip_address
}
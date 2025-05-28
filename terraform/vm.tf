resource "azurerm_linux_virtual_machine" "pso_vm_prod" {
  name                  = "PSOVM-Prod"
  location              = azurerm_resource_group.pso_rg.location
  resource_group_name   = azurerm_resource_group.pso_rg.name
  network_interface_ids = [azurerm_network_interface.pso_nic_prod.id]
  size                  = "Standard_B1s"

  os_disk {
    name                 = "pso_disk_prod"
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts-gen2"
    version   = "latest"
  }

  computer_name  = "hostname"
  admin_username = var.username_production
  admin_password = var.password_production
  disable_password_authentication = false

  boot_diagnostics {
    storage_account_uri = azurerm_storage_account.pso_sg.primary_blob_endpoint
  }
}

resource "azurerm_linux_virtual_machine" "pso_vm_dev" {
  name                  = "PSOVM-Dev"
  location              = azurerm_resource_group.pso_rg.location
  resource_group_name   = azurerm_resource_group.pso_rg.name
  network_interface_ids = [azurerm_network_interface.pso_nic_dev.id]
  size                  = "Standard_B1s"

  os_disk {
    name                 = "pso_disk_dev"
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts-gen2"
    version   = "latest"
  }

  computer_name  = "hostname"
  admin_username = var.username_development
  admin_password = var.password_development
  disable_password_authentication = false

  boot_diagnostics {
    storage_account_uri = azurerm_storage_account.pso_sg.primary_blob_endpoint
  }
}
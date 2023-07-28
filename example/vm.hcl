job "vm" {
  datacenters = ["dc1"]
  type        = "service"

  group "vm" {
    task "vm" {
      driver = "libvirt"

      config {
      }

      resources {
        cpu = 1024
        memory = 1024
      }
    }
  }
}

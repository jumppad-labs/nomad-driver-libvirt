variable "image_path" {
  type = string
}

job "ubuntu" {
  datacenters = ["dc1"]
  type        = "service"

  group "ubuntu" {
    task "ubuntu" {
      driver = "libvirt"

      config {
        disk {
          driver {
            name = "qemu"
            type = "qcow2"
          }

          source = "${var.image_path}/ubuntu.qcow2"
          target = "hda"
          device = "disk"
        }

        interface {
          type = "bridge"
          source = "default"
        }

        vnc {
          port = -1
          websocket = 8998
        }
      }

      resources {
        cpu = 1024
        memory = 1024
      }
    }
  }
}

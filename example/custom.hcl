variable "image_path" {
  type = string
}

job "custom" {
  datacenters = ["dc1"]
  type        = "service"

  group "custom" {
    network {
      mode = "bridge"
    }

    task "custom" {
      driver = "libvirt"

      config {
        disk {
          driver {
            name = "qemu"
            type = "qcow2"
          }

          source = "${var.image_path}/custom.qcow2"
          target = "hda"
          device = "disk"
        }

        interface {
          type = "bridge"
          source = "default"
        }

        vnc {
          port = -1
          websocket = 8999
        }
      }

      resources {
        cpu = 1024
        memory = 1024
      }
    }
  }
}

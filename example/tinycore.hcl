variable "image_path" {
  type = string
}

job "tinycore" {
  datacenters = ["dc1"]
  type        = "service"

  group "tinycore" {
    task "tinycore" {
      driver = "libvirt"

      config {
        disk {
          driver {
            name = "qemu"
            type = "qcow2"
          }

          source = "${var.image_path}/tinycore.qcow2"
          target = "hda"
          device = "disk"
        }
        
        interface {
          type = "bridge"
          source = "default"
        }

        vnc {
          port = -1
          websocket = 8997
        }
      }

      resources {
        cpu = 1024
        memory = 1024
      }
    }
  }
}

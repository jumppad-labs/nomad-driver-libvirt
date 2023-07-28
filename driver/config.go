package driver

import (
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

// Config contains configuration information for the plugin
type Config struct {
	Emulator string `codec:"emulator"`
}

// TaskConfig contains configuration information for a task that runs with
// this plugin
type TaskConfig struct {
	Disk      []Disk      `codec:"disk"`
	Interface []Interface `codec:"interface"`
	Vnc       Vnc         `codec:"vnc"`
}

type Disk struct {
	Source string `codec:"source"`
	Driver Driver `codec:"driver"`
	Target string `codec:"target"`
	Device string `codec:"device"`
}

type Driver struct {
	Name string `codec:"name"`
	Type string `codec:"type"`
}

// <interface type='bridge'>
// 	<model type='virtio'/>
// 	<source bridge='virbr0'/>
// 	<address type='pci' domain='0x0000' bus='0x00' slot='0x03' function='0x0'/>
// </interface>

type Interface struct {
	Type   string `codec:"type"`
	Source string `codec:"source"`
}

type Vnc struct {
	Port      int `codec:"port"`
	Websocket int `codec:"websocket"`
}

var configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
	"emulator": hclspec.NewDefault(
		hclspec.NewAttr("emulator", "string", false),
		hclspec.NewLiteral(`"/usr/bin/qemu-system-x86_64"`),
	),
})

var taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
	"disk": hclspec.NewBlockList("disk", hclspec.NewObject(map[string]*hclspec.Spec{
		"source": hclspec.NewAttr("source", "string", true),
		"driver": hclspec.NewBlock("driver", true, hclspec.NewObject(map[string]*hclspec.Spec{
			"name": hclspec.NewAttr("name", "string", true),
			"type": hclspec.NewAttr("type", "string", true),
		})),
		"target": hclspec.NewAttr("target", "string", true),
		"device": hclspec.NewAttr("device", "string", true),
	})),
	"interface": hclspec.NewBlockList("interface", hclspec.NewObject(map[string]*hclspec.Spec{
		"type":   hclspec.NewAttr("type", "string", true),
		"source": hclspec.NewAttr("source", "string", true),
	})),
	"vnc": hclspec.NewBlock("vnc", true, hclspec.NewObject(map[string]*hclspec.Spec{
		"port":      hclspec.NewAttr("port", "number", true),
		"websocket": hclspec.NewAttr("websocket", "number", true),
	})),
})

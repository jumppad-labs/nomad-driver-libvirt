package main

import (
	"github.com/jumppad-labs/nomad-driver-libvirt/driver"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins"
)

func main() {
	plugins.Serve(factory)
}

func factory(log hclog.Logger) interface{} {
	return driver.NewPlugin(log)
}

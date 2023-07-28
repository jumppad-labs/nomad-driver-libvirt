package driver

import (
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

// Config contains configuration information for the plugin
type Config struct {
}

// TaskConfig contains configuration information for a task that runs with
// this plugin
type TaskConfig struct {
}

var configSpec = hclspec.NewObject(map[string]*hclspec.Spec{})

var taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{})

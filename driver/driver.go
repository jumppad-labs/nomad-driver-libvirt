package driver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"

	libvirt "github.com/digitalocean/go-libvirt"
	socket "github.com/digitalocean/go-libvirt/socket/dialers"
	"libvirt.org/go/libvirtxml"
)

const (
	pluginName        = "libvirt"
	pluginVersion     = "v0.1.0"
	fingerprintPeriod = 30 * time.Second
	taskHandleVersion = 1
)

var (
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{drivers.ApiVersion010},
		PluginVersion:     pluginVersion,
		Name:              pluginName,
	}

	capabilities = &drivers.Capabilities{
		SendSignals:         true,
		Exec:                false,
		MustInitiateNetwork: false,
		NetIsolationModes: []drivers.NetIsolationMode{
			drivers.NetIsolationModeGroup,
		},
	}
)

type TaskState struct {
	ReattachConfig *structs.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	StartedAt      time.Time
}

type LibVirtDriverPlugin struct {
	eventer        *eventer.Eventer
	config         *Config
	nomadConfig    *base.ClientDriverConfig
	tasks          *taskStore
	ctx            context.Context
	signalShutdown context.CancelFunc
	logger         hclog.Logger
}

// NewPlugin returns a new driver plugin
func NewPlugin(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)

	return &LibVirtDriverPlugin{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &Config{},
		tasks:          newTaskStore(),
		ctx:            ctx,
		signalShutdown: cancel,
		logger:         logger,
	}
}

// PluginInfo returns information describing the plugin.
func (d *LibVirtDriverPlugin) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the plugin configuration schema.
func (d *LibVirtDriverPlugin) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig is called by the client to pass the configuration for the plugin.
func (d *LibVirtDriverPlugin) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}

	d.config = &config

	// TODO: parse and validated any configuration value if necessary.

	// Save the Nomad agent configuration
	if cfg.AgentConfig != nil {
		d.nomadConfig = cfg.AgentConfig.Driver
	}

	// Here you can use the config values to initialize any resources that are
	// shared by all tasks that use this driver, such as a daemon process.

	return nil
}

// TaskConfigSchema returns the HCL schema for the configuration of a task.
func (d *LibVirtDriverPlugin) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

// Capabilities returns the features supported by the driver.
func (d *LibVirtDriverPlugin) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

// Fingerprint returns a channel that will be used to send health information
// and other driver specific node attributes.
func (d *LibVirtDriverPlugin) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil
}

// handleFingerprint manages the channel and the flow of fingerprint data.
func (d *LibVirtDriverPlugin) handleFingerprint(ctx context.Context, ch chan<- *drivers.Fingerprint) {
	defer close(ch)

	// Nomad expects the initial fingerprint to be sent immediately
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			// after the initial fingerprint we can set the proper fingerprint
			// period
			ticker.Reset(fingerprintPeriod)
			ch <- d.buildFingerprint()
		}
	}
}

// buildFingerprint returns the driver's fingerprint data
func (d *LibVirtDriverPlugin) buildFingerprint() *drivers.Fingerprint {
	fp := &drivers.Fingerprint{
		Attributes:        map[string]*structs.Attribute{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: drivers.DriverHealthy,
	}

	return fp
}

// StartTask returns a task handle and a driver network if necessary.
func (d *LibVirtDriverPlugin) StartTask(taskConfig *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if _, ok := d.tasks.Get(taskConfig.ID); ok {
		return nil, nil, fmt.Errorf("task with ID %q already started", taskConfig.ID)
	}

	var driverConfig TaskConfig
	if err := taskConfig.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	d.logger.Info("starting task", "driver_cfg", hclog.Fmt("%+v", driverConfig))
	handle := drivers.NewTaskHandle(taskHandleVersion)
	handle.Config = taskConfig

	// LibVirt logic start
	dialer := socket.NewLocal()

	client := libvirt.NewWithDialer(dialer)
	if err := client.Connect(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect: %v", err)
	}

	domainConfig := &libvirtxml.Domain{
		Type: "qemu",
		Name: "vm",
		UUID: "c7a5fdbd-cdaf-9455-926a-d65c16db1809",
		Memory: &libvirtxml.DomainMemory{
			Value: 4096,
			Unit:  "MB",
		},
		CurrentMemory: &libvirtxml.DomainCurrentMemory{
			Value: 4096,
			Unit:  "MB",
		},
		VCPU: &libvirtxml.DomainVCPU{
			Value: 2,
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Arch:    "i686",
				Machine: "pc-i440fx-6.2",
				Type:    "hvm",
			},
			BootDevices: []libvirtxml.DomainBootDevice{
				{
					Dev: "hd",
				},
			},
		},
		Devices: &libvirtxml.DomainDeviceList{
			Emulator: "/usr/bin/qemu-system-x86_64",
			Disks: []libvirtxml.DomainDisk{
				{
					Driver: &libvirtxml.DomainDiskDriver{
						Name: "qemu",
						Type: "qcow2",
					},
					Source: &libvirtxml.DomainDiskSource{
						File: &libvirtxml.DomainDiskSourceFile{
							File: "/home/erik/code/jumppad/vm/docker/custom.qcow2",
						},
					},
					Target: &libvirtxml.DomainDiskTarget{
						Dev: "hda",
					},
					Device: "disk",
				},
			},
			Graphics: []libvirtxml.DomainGraphic{
				{
					VNC: &libvirtxml.DomainGraphicVNC{
						Port:      -1,
						WebSocket: 8999,
					},
				},
			},
		},
	}

	xml, err := domainConfig.Marshal()
	if err != nil {
		return nil, nil, fmt.Errorf("could not marshall vm config: %v", err)
	}

	domain, err := client.DomainCreateXML(xml, libvirt.DomainNone)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create domains: %v", err)
	}

	// LibVirt logic end

	h := &taskHandle{
		ctx:           d.ctx,
		domain:        domain,
		client:        client,
		taskConfig:    taskConfig,
		taskState:     drivers.TaskStateRunning,
		startedAt:     time.Now().Round(time.Millisecond),
		logger:        d.logger,
		cpuStatsSys:   stats.NewCpuStats(),
		cpuStatsUser:  stats.NewCpuStats(),
		cpuStatsTotal: stats.NewCpuStats(),
	}

	driverState := TaskState{
		TaskConfig: taskConfig,
		StartedAt:  h.startedAt,
	}

	if err := handle.SetDriverState(&driverState); err != nil {
		return nil, nil, fmt.Errorf("failed to set driver state: %v", err)
	}

	d.tasks.Set(taskConfig.ID, h)
	go h.run()
	return handle, nil, nil
}

func (d *LibVirtDriverPlugin) Shutdown(ctx context.Context) error {
	d.signalShutdown()
	return nil
}

// RecoverTask recreates the in-memory state of a task from a TaskHandle.
func (d *LibVirtDriverPlugin) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return errors.New("error: handle cannot be nil")
	}

	if _, ok := d.tasks.Get(handle.Config.ID); ok {
		return nil
	}

	var taskState TaskState
	if err := handle.GetDriverState(&taskState); err != nil {
		return fmt.Errorf("failed to decode task state from handle: %v", err)
	}

	var driverConfig TaskConfig
	if err := taskState.TaskConfig.DecodeDriverConfig(&driverConfig); err != nil {
		return fmt.Errorf("failed to decode driver config: %v", err)
	}

	h := &taskHandle{
		taskConfig: taskState.TaskConfig,
		taskState:  drivers.TaskStateRunning,
		startedAt:  taskState.StartedAt,
		exitResult: &drivers.ExitResult{},
	}

	d.tasks.Set(taskState.TaskConfig.ID, h)

	go h.run()
	return nil
}

// WaitTask returns a channel used to notify Nomad when a task exits.
func (d *LibVirtDriverPlugin) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	ch := make(chan *drivers.ExitResult)
	go d.handleWait(ctx, handle, ch)
	return ch, nil
}

func (d *LibVirtDriverPlugin) handleWait(ctx context.Context, handle *taskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)

	// Going with simplest approach of polling for handler to mark exit.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			s := handle.status()
			if s.State == drivers.TaskStateExited {
				ch <- handle.exitResult
			}
		}
	}
}

// StopTask stops a running task with the given signal and within the timeout window.
func (d *LibVirtDriverPlugin) StopTask(taskID string, timeout time.Duration, signal string) error {
	d.logger.Info("!!!!!!!!!! stopping task", "taskID", taskID)

	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if err := handle.shutdown(timeout); err != nil {
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	return nil
}

// DestroyTask cleans up and removes a task that has terminated.
func (d *LibVirtDriverPlugin) DestroyTask(taskID string, force bool) error {
	d.logger.Info("!!!!!!!!!! destroy task", "taskID", taskID)

	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if handle.IsRunning() && !force {
		return errors.New("cannot destroy running task")
	}

	// Destroying a task includes removing any resources used by task and any
	// local references in the plugin. If force is set to true the task should
	// be destroyed even if it's currently running.
	//
	// In the example below we use the executor to force shutdown the task
	// (timeout equals 0).
	if handle.IsRunning() {
		// grace period is chosen arbitrary here
		if err := handle.shutdown(1 * time.Minute); err != nil {
			handle.logger.Error("failed to destroy executor", "err", err)
		}

		// Wait for stats handler to report that the task has exited
		for i := 0; i < 10; i++ {
			if !handle.IsRunning() {
				break
			}
			time.Sleep(time.Millisecond * 250)
		}
	}

	d.logger.Info("!!!!!!!!!! destroyed task", "taskID", taskID)

	d.tasks.Delete(taskID)
	return nil
}

// InspectTask returns detailed status information for the referenced taskID.
func (d *LibVirtDriverPlugin) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return handle.status(), nil
}

// TaskStats returns a channel which the driver should send stats to at the given interval.
func (d *LibVirtDriverPlugin) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	statsChannel := make(chan *drivers.TaskResourceUsage)
	go handle.stats(ctx, statsChannel, interval)

	return statsChannel, nil
}

// TaskEvents returns a channel that the plugin can use to emit task related events.
func (d *LibVirtDriverPlugin) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

// SignalTask forwards a signal to a task.
// This is an optional capability.
func (d *LibVirtDriverPlugin) SignalTask(taskID string, signal string) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig := os.Interrupt
	if s, ok := signals.SignalLookup[signal]; ok {
		sig = s
	} else {
		d.logger.Warn("unknown signal to send to task, using SIGINT instead", "signal", signal, "task_id", handle.taskConfig.ID)

	}
	return handle.signal(sig)
}

func (d *LibVirtDriverPlugin) CalculateVcpuCount(shares int64) int64 {
	cores := math.Floor(float64(shares) / 1000)
	if cores < 1 {
		return 1
	} else {
		return int64(cores)
	}
}

// ExecTask returns the result of executing the given command inside a task.
// This is an optional capability.
func (d *LibVirtDriverPlugin) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	return nil, errors.New("this driver does not support exec")
}

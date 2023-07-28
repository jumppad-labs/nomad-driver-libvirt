package driver

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// taskHandle should store all relevant runtime information
// such as process ID if this is a local task or other meta
// data if this driver deals with external APIs
type taskHandle struct {
	// stateLock syncs access to all fields below
	stateLock sync.RWMutex

	logger      hclog.Logger
	taskConfig  *drivers.TaskConfig
	taskState   drivers.TaskState
	startedAt   time.Time
	completedAt time.Time
	exitResult  *drivers.ExitResult

	client *libvirt.Libvirt
	domain libvirt.Domain
	ctx    context.Context

	// cpuStatsSys   *stats.CpuStats
	// cpuStatsUser  *stats.CpuStats
	// cpuStatsTotal *stats.CpuStats
}

func (h *taskHandle) status() *drivers.TaskStatus {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()

	return &drivers.TaskStatus{
		ID:               h.taskConfig.ID,
		Name:             h.taskConfig.Name,
		State:            h.taskState,
		StartedAt:        h.startedAt,
		CompletedAt:      h.completedAt,
		ExitResult:       h.exitResult,
		DriverAttributes: map[string]string{},
	}
}

func (h *taskHandle) IsRunning() bool {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()

	return h.taskState == drivers.TaskStateRunning
}

func (h *taskHandle) run() {
	h.stateLock.Lock()
	if h.exitResult == nil {
		h.exitResult = &drivers.ExitResult{}
	}
	h.stateLock.Unlock()

	for {
		active, err := h.client.DomainIsActive(h.domain)
		if active != 1 || err != nil {
			break
		}
	}

	// wait for vm
	// events, err := h.client.SubscribeEvents(h.ctx, libvirt.DomainEventIDLifecycle, libvirt.OptDomain{h.domain})
	// if err != nil {
	// 	h.logger.Error("Could not subscribe to events", err)
	// 	h.taskState = drivers.TaskStateExited
	// 	h.exitResult.ExitCode = 0
	// 	h.exitResult.Signal = 0
	// 	h.completedAt = time.Now()
	// 	return
	// }

	// h.logger.Info("Listening for events from libvirt")

	// for event := range events {
	// 	h.logger.Info("Received a message from libvirt", event)
	// }

	h.stateLock.Lock()
	defer h.stateLock.Unlock()

	h.logger.Info("We are done, shutting down")

	// stop vm
	err := h.client.DomainShutdown(h.domain)
	if err != nil {
		h.logger.Error("Could not shutdown vm", err)
	}

	h.logger.Info("Disconnecting from libvirt")

	err = h.client.Disconnect()
	if err != nil {
		h.logger.Error("Could not disconnect from libvirt", err)
	}

	h.taskState = drivers.TaskStateExited
	h.exitResult.ExitCode = 0
	h.exitResult.Signal = 0
	h.completedAt = time.Now()
}

func (h *taskHandle) shutdown(timeout time.Duration) error {
	err := h.client.DomainDestroy(h.domain)

	// err := h.client.DomainShutdownFlags(h.domain, libvirt.DomainShutdownDefault)
	if err != nil {
		return fmt.Errorf("Shutdown failed: %v", err)
	}

	time.Sleep(timeout)

	h.logger.Info("!!!!!!!!!! stopped task")

	return nil
}

func (h *taskHandle) stats(ctx context.Context, statsChannel chan *drivers.TaskResourceUsage, interval time.Duration) {
	defer close(statsChannel)
	timer := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			timer.Reset(interval)
		}

		// h.stateLock.Lock()
		// t := time.Now()

		// h.stateLock.Unlock()

		// usage := drivers.TaskResourceUsage{
		// 	ResourceUsage: &drivers.ResourceUsage{},
		// 	Timestamp:     t.UTC().UnixNano(),
		// }
		// // send stats to nomad
		// statsChannel <- &usage
	}
}

func (h *taskHandle) signal(sig os.Signal) error {

	return nil
}

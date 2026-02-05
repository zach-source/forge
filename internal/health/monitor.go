package health

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// GetResourceMetrics reads current CPU and memory usage.
func GetResourceMetrics() (cpuPercent, memPercent float64, err error) {
	cpuPcts, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, 0, err
	}
	if len(cpuPcts) > 0 {
		cpuPercent = cpuPcts[0]
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, err
	}
	memPercent = memInfo.UsedPercent

	return cpuPercent, memPercent, nil
}

package health

// HealthStatus represents the overall system health level.
type HealthStatus string

const (
	StatusGood     HealthStatus = "good"
	StatusDegraded HealthStatus = "degraded"
	StatusCritical HealthStatus = "critical"
)

// Metrics holds current system health measurements.
type Metrics struct {
	CPUPercent    float64      `json:"cpu_percent"`
	MemoryPercent float64      `json:"memory_percent"`
	FailureRate   float64      `json:"failure_rate"`
	APIErrors     int          `json:"api_errors"`
	ActiveWorkers int          `json:"active_workers"`
	Status        HealthStatus `json:"status"`
	Reason        string       `json:"reason,omitempty"`
}

// ThresholdConfig defines when to degrade or pause worker assignment.
type ThresholdConfig struct {
	CPUCritical      float64
	CPUDegraded      float64
	MemoryCritical   float64
	MemoryDegraded   float64
	FailureThreshold float64
	APIErrorLimit    int
}

// DefaultThresholds returns sensible defaults for health thresholds.
func DefaultThresholds() ThresholdConfig {
	return ThresholdConfig{
		CPUCritical:      90.0,
		CPUDegraded:      80.0,
		MemoryCritical:   90.0,
		MemoryDegraded:   80.0,
		FailureThreshold: 0.5,
		APIErrorLimit:    5,
	}
}

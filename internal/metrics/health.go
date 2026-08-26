package metrics

import (
	"fmt"
	"strings"

	"etl-lineage/internal/graph"
)

type HealthStatus string

const (
	HealthGood    HealthStatus = "GOOD"
	HealthWarning HealthStatus = "WARNING"
	HealthCrit    HealthStatus = "CRITICAL"
)

type HealthCheck struct {
	Name    string       `json:"name"`
	Status  HealthStatus `json:"status"`
	Value   float64      `json:"value"`
	Limit   float64      `json:"limit"`
	Message string       `json:"message"`
}

type HealthReport struct {
	Overall HealthStatus  `json:"overall"`
	Checks  []HealthCheck `json:"checks"`
}

type HealthConfig struct {
	MaxDepth      int
	MaxFanIn      int
	MaxFanOut     int
	MaxDensity    float64
	MaxCoupling   float64
	MaxComponents int
}

func DefaultHealthConfig() *HealthConfig {
	return &HealthConfig{
		MaxDepth:      8,
		MaxFanIn:      10,
		MaxFanOut:     15,
		MaxDensity:    0.3,
		MaxCoupling:   0.5,
		MaxComponents: 2,
	}
}

func CheckHealth(g *graph.Graph, cfg *HealthConfig) *HealthReport {
	if cfg == nil {
		cfg = DefaultHealthConfig()
	}
	m := Compute(g)
	report := &HealthReport{Overall: HealthGood}

	report.addCheck("depth", float64(m.Depth), float64(cfg.MaxDepth),
		fmt.Sprintf("pipeline depth %d", m.Depth))

	report.addCheck("max_fan_in", float64(m.MaxFanIn), float64(cfg.MaxFanIn),
		fmt.Sprintf("max fan-in %d at %s", m.MaxFanIn, m.MaxFanInNode))

	report.addCheck("max_fan_out", float64(m.MaxFanOut), float64(cfg.MaxFanOut),
		fmt.Sprintf("max fan-out %d at %s", m.MaxFanOut, m.MaxFanOutNode))

	report.addCheck("density", m.Density, cfg.MaxDensity,
		fmt.Sprintf("graph density %.4f", m.Density))

	report.addCheck("coupling", m.CouplingScore, cfg.MaxCoupling,
		fmt.Sprintf("cross-owner coupling %.2f%%", m.CouplingScore*100))

	report.addCheck("components", float64(m.Components), float64(cfg.MaxComponents),
		fmt.Sprintf("%d disconnected components", m.Components))

	for _, c := range report.Checks {
		if c.Status == HealthCrit {
			report.Overall = HealthCrit
			break
		}
		if c.Status == HealthWarning && report.Overall == HealthGood {
			report.Overall = HealthWarning
		}
	}

	return report
}

func (r *HealthReport) addCheck(name string, value, limit float64, message string) {
	status := HealthGood
	if value > limit {
		status = HealthCrit
	} else if value > limit*0.8 {
		status = HealthWarning
	}
	r.Checks = append(r.Checks, HealthCheck{
		Name:    name,
		Status:  status,
		Value:   value,
		Limit:   limit,
		Message: message,
	})
}

func (r *HealthReport) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Health: %s\n", r.Overall)
	for _, c := range r.Checks {
		icon := "✓"
		if c.Status == HealthWarning {
			icon = "!"
		} else if c.Status == HealthCrit {
			icon = "X"
		}
		fmt.Fprintf(&b, "  [%s] %s: %s (%.1f / %.1f)\n", icon, c.Name, c.Message, c.Value, c.Limit)
	}
	return b.String()
}

func (r *HealthReport) PassingChecks() int {
	count := 0
	for _, c := range r.Checks {
		if c.Status == HealthGood {
			count++
		}
	}
	return count
}

func (r *HealthReport) FailingChecks() []string {
	var out []string
	for _, c := range r.Checks {
		if c.Status == HealthCrit {
			out = append(out, c.Name)
		}
	}
	return out
}

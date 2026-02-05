package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/leader"
)

func newObserveCmd() *cobra.Command {
	var (
		workDir       string
		maxIterations int
		skipPerms     bool
	)

	cmd := &cobra.Command{
		Use:   "observe",
		Short: "Run the infrastructure monitor leader",
		Long: `Run the monitor leader to observe infrastructure health.

The monitor watches:
- K0s cluster health and node status
- Prometheus metrics and alerts
- Loki logs for errors and warnings
- Distributed tracing (Jaeger/Tempo)
- Flux GitOps reconciliation status
- Service health and readiness

The monitor uses a prompt file from .forge/prompts/monitor.md if present,
otherwise falls back to a default monitoring prompt.

Examples:
  foundry observe                    # Start monitor in current directory
  foundry observe -d /path/to/repo   # Monitor specific repository
  foundry observe --max-iterations 50  # Limit iterations`,
		Aliases: []string{"mon-leader", "watch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if workDir == "" {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
				workDir = wd
			}

			// Try to load custom prompt from .forge/prompts/monitor.md
			loader := leader.NewPromptLoader(workDir)
			prompt, err := loader.Load("monitor")
			if err != nil {
				return fmt.Errorf("loading monitor prompt: %w", err)
			}

			// If no custom prompt, use default
			if prompt == "" {
				prompt = buildDefaultMonitorPrompt(workDir)
			}

			cfg := leader.Config{
				Role:          leader.RoleMonitor,
				WorkDir:       workDir,
				MaxIterations: maxIterations,
				SkipPerms:     skipPerms,
			}

			return leader.Run(context.Background(), cfg, prompt, leader.PromiseMonitor)
		},
	}

	cmd.Flags().StringVarP(&workDir, "dir", "d", "", "Working directory (default: current)")
	cmd.Flags().IntVar(&maxIterations, "max-iterations", 100, "Maximum iterations before stopping")
	cmd.Flags().BoolVar(&skipPerms, "skip-perms", true, "Skip permission prompts")

	return cmd
}

// buildDefaultMonitorPrompt creates the default monitoring prompt.
func buildDefaultMonitorPrompt(workDir string) string {
	return fmt.Sprintf(`You are an INFRASTRUCTURE MONITOR for the Forge supervisor.
Your job is to observe and report on infrastructure health.

## What to Monitor

### 1. K0s Cluster Health
- Node status: %s
- Pod health across namespaces
- Resource utilization (CPU, memory, disk)
- Cluster events and warnings

### 2. Prometheus Metrics
- Active alerts and their severity
- Key metrics trending (error rates, latency)
- Scrape target health
- Alert rule evaluation

### 3. Loki Logs
- Error patterns across services
- Warning frequency and trends
- Log volume anomalies
- Critical service logs

### 4. Distributed Tracing
- Trace error rates
- Latency percentiles (p50, p95, p99)
- Service dependency health
- Slow endpoints

### 5. Flux GitOps Status
- Kustomization reconciliation status
- HelmRelease health
- GitRepository sync status
- Image update automation status

### 6. Service Health
- Deployment readiness
- Service endpoint availability
- Ingress/route status
- Certificate expiration

## Loading Context from Memory

At the start of your session, load relevant context from Graphiti:

%s
# Search for infrastructure context
search_nodes({ query: "infrastructure k0s prometheus" })

# Search for recent alerts or issues
search_memory_facts({ query: "alert error infrastructure" })

# Search for monitoring configuration
search_memory_facts({ query: "monitor config threshold" })
%s

## Monitoring Commands

Use kubectl and CLI tools to gather data:

%s
# K0s cluster status
kubectl get nodes -o wide
kubectl get pods -A --field-selector=status.phase!=Running

# Prometheus alerts
kubectl exec -n monitoring prometheus-0 -- promtool query instant http://localhost:9090 'ALERTS{alertstate="firing"}'

# Flux status
flux get all -A

# Service health
kubectl get deployments -A
kubectl get ingress -A
%s

## Reporting Format

When you find issues, report them clearly:

%s
### Health Report

**Cluster Status**: [healthy/degraded/critical]
**Active Alerts**: [count]
**Services Down**: [list]

#### Issues Found
1. [Issue description]
   - Severity: [critical/warning/info]
   - Component: [affected component]
   - Recommendation: [suggested action]

#### Metrics Summary
- Error rate: [value]
- P95 latency: [value]
- Pod restarts (24h): [count]
%s

## Saving to Memory

Before completing, save important findings to Graphiti:

%s
add_memory({
  group_id: "forge-monitor",
  content: %s
    TIMESTAMP: [current time]
    CLUSTER_STATUS: [overall health]
    ALERTS: [active alert summary]
    ISSUES: [issues found]
    RECOMMENDATIONS: [suggested actions]
  %s
})
%s

## Continuous Operation

You run continuously until:
1. All systems are healthy (no active alerts)
2. All issues have been documented
3. Recommendations have been recorded

Working directory: %s

%s

When monitoring is complete and all systems are healthy, output EXACTLY this text (including the XML tags):
%s
<promise>%s</promise>
%s
`, workDir,
		"```", "```", // Context section
		"```bash", "```", // Commands section
		"```", "```", // Reporting section
		"```", "`", "`", "```", // Memory section
		workDir,
		leader.OutputFormat,
		"```", leader.PromiseMonitor, "```")
}

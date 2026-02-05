package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/worker"
)

func newSummaryCmd() *cobra.Command {
	var rawOutput bool

	cmd := &cobra.Command{
		Use:     "summary",
		Aliases: []string{"status", "s"},
		Short:   "Show orchestration summary",
		Long: `Display a formatted summary of the current orchestration state.

Uses Claude Haiku to format the output into a readable table format showing:
- Active workers and their current tasks
- Idle leaders and why they're waiting
- Board state (task counts by status)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSummary(rawOutput)
		},
	}

	cmd.Flags().BoolVar(&rawOutput, "raw", false, "Output raw data without Haiku formatting")

	return cmd
}

type summaryData struct {
	ActiveWorkers []workerInfo `json:"active_workers"`
	IdleLeaders   []leaderInfo `json:"idle_leaders"`
	BoardState    boardState   `json:"board_state"`
	ForgeSessions int          `json:"forge_sessions"`
}

type workerInfo struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Task   string `json:"task"`
	Status string `json:"status"`
}

type leaderInfo struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

type boardState struct {
	Backlog    int `json:"backlog"`
	Todo       int `json:"todo"`
	InProgress int `json:"in_progress"`
	Review     int `json:"review"`
	Merge      int `json:"merge"`
	Done       int `json:"done"`
}

func runSummary(rawOutput bool) error {
	// Gather data
	data, err := gatherSummaryData()
	if err != nil {
		return fmt.Errorf("gathering data: %w", err)
	}

	if rawOutput {
		// Output raw JSON
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Format with Haiku
	return formatWithHaiku(data)
}

func gatherSummaryData() (*summaryData, error) {
	data := &summaryData{}

	// Load worker registry
	reg, err := worker.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("loading registry: %w", err)
	}

	// Categorize workers
	workers := reg.List()
	for _, w := range workers {
		if w.Status == worker.StatusActive {
			data.ActiveWorkers = append(data.ActiveWorkers, workerInfo{
				Name:   w.DisplayName(),
				Role:   string(w.Role),
				Task:   w.CurrentTask,
				Status: "active",
			})
		} else if w.Status == worker.StatusIdle && isLeaderRole(w.Role) {
			reason := getIdleReason(w.Role, data)
			data.IdleLeaders = append(data.IdleLeaders, leaderInfo{
				Name:   w.DisplayName(),
				Role:   string(w.Role),
				Reason: reason,
			})
		}
	}

	// Load board state
	store, err := getKanbanStore()
	if err == nil {
		defer store.Close()
		board, err := store.GetBoard()
		if err == nil {
			for _, col := range board.Columns {
				count := len(col.Issues)
				switch col.Status {
				case kanban.StatusBacklog:
					data.BoardState.Backlog = count
				case kanban.StatusTodo:
					data.BoardState.Todo = count
				case kanban.StatusInProgress:
					data.BoardState.InProgress = count
				case kanban.StatusReview:
					data.BoardState.Review = count
				case kanban.StatusMerge:
					data.BoardState.Merge = count
				case kanban.StatusDone:
					data.BoardState.Done = count
				}
			}
		}
	}

	// Count forge sessions
	out, err := exec.Command("forge", "list", "--json").Output()
	if err == nil {
		var sessions []interface{}
		if json.Unmarshal(out, &sessions) == nil {
			data.ForgeSessions = len(sessions)
		}
	}

	return data, nil
}

func getIdleReason(role worker.Role, data *summaryData) string {
	switch role {
	case worker.RolePlanner:
		return "No backlog to prioritize"
	case worker.RoleReviewer:
		if data.BoardState.Review == 0 {
			return "No tasks in review queue"
		}
		return "Waiting to start"
	case worker.RoleMerge:
		if data.BoardState.Merge == 0 {
			return "No tasks ready to merge"
		}
		return "Waiting for workers to complete"
	case worker.RoleDeploy:
		return "Waiting for merge to complete"
	case worker.RoleGroomer:
		if data.BoardState.Backlog == 0 {
			return "No backlog items to groom"
		}
		return "Waiting to start"
	case worker.RoleMonitor:
		return "Waiting to start"
	case worker.RoleTester:
		return "Waiting to start"
	case worker.RolePM:
		return "Waiting to start"
	}
	return "Idle"
}

func formatWithHaiku(data *summaryData) error {
	// Build context for Haiku
	var sb strings.Builder

	sb.WriteString("Format this orchestration data into a clean markdown summary.\n\n")
	sb.WriteString("## Data\n\n")

	// Active workers
	sb.WriteString("### Active Workers\n")
	if len(data.ActiveWorkers) == 0 {
		sb.WriteString("None\n")
	} else {
		for _, w := range data.ActiveWorkers {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", w.Name, w.Role, w.Task))
		}
	}

	// Idle leaders
	sb.WriteString("\n### Idle Leaders\n")
	if len(data.IdleLeaders) == 0 {
		sb.WriteString("None\n")
	} else {
		for _, l := range data.IdleLeaders {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", l.Name, l.Role, l.Reason))
		}
	}

	// Board state
	sb.WriteString("\n### Board State\n")
	sb.WriteString(fmt.Sprintf("- Backlog: %d\n", data.BoardState.Backlog))
	sb.WriteString(fmt.Sprintf("- Todo: %d\n", data.BoardState.Todo))
	sb.WriteString(fmt.Sprintf("- In Progress: %d\n", data.BoardState.InProgress))
	sb.WriteString(fmt.Sprintf("- Review: %d\n", data.BoardState.Review))
	sb.WriteString(fmt.Sprintf("- Merge: %d\n", data.BoardState.Merge))
	sb.WriteString(fmt.Sprintf("- Done: %d\n", data.BoardState.Done))

	sb.WriteString("\n## Instructions\n\n")
	sb.WriteString("Format this into two tables:\n")
	sb.WriteString("1. **Active Workers** - columns: Worker, Role, Status (use emoji), Task\n")
	sb.WriteString("2. **Idle Leaders** - columns: Worker, Role, Reason\n")
	sb.WriteString("3. **Board State** - show as a flow: backlog → todo → in-progress → review → merge → done\n\n")
	sb.WriteString("Use these emoji for roles: worker=👷, planner=📋, reviewer=🔍, merge=🔀, deploy=🚀, groomer=🧹, monitor=📡, tester=🧪, pm=📊\n")
	sb.WriteString("Use 🔄 for active status.\n")
	sb.WriteString("Keep it concise. Output ONLY the formatted tables, no explanations.\n")

	// Call Haiku
	cmd := exec.Command("claude", "-p", sb.String(), "--model", "haiku")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Fallback to simple output if Haiku fails
		fmt.Println("## Orchestration Summary")
		fmt.Println()
		fmt.Println("### Active Workers")
		if len(data.ActiveWorkers) == 0 {
			fmt.Println("  (none)")
		}
		for _, w := range data.ActiveWorkers {
			fmt.Printf("  - %s (%s): %s\n", w.Name, w.Role, w.Task)
		}
		fmt.Println("\n### Idle Leaders")
		if len(data.IdleLeaders) == 0 {
			fmt.Println("  (none)")
		}
		for _, l := range data.IdleLeaders {
			fmt.Printf("  - %s (%s): %s\n", l.Name, l.Role, l.Reason)
		}
		fmt.Printf("\n### Board State\n")
		fmt.Printf("  %d backlog → %d todo → %d in-progress → %d review → %d merge → %d done\n",
			data.BoardState.Backlog, data.BoardState.Todo, data.BoardState.InProgress,
			data.BoardState.Review, data.BoardState.Merge, data.BoardState.Done)
		return nil
	}

	fmt.Print(stdout.String())
	return nil
}

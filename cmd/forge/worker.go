package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/agent"
	"github.com/zach-source/forge/internal/tmux"
	"github.com/zach-source/forge/internal/worker"
)

func newWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Manage parallel forge workers",
		Long: `Manage parallel forge workers with persistent identities.

Workers are named using the NATO alphabet (alpha, bravo, charlie...) and maintain
identity across sessions through Graphiti memory. Each worker can be assigned to
a task and worktree, enabling parallel development.

Examples:
  forge worker create                      # Create worker (auto-named)
  forge worker create --alias "api-dev"    # Create with alias
  forge worker list                        # List all workers
  forge worker start alpha --task auth     # Start worker on task
  forge worker stop alpha                  # Stop worker`,
		Aliases: []string{"w"},
	}

	cmd.AddCommand(
		newWorkerCreateCmd(),
		newWorkerListCmd(),
		newWorkerStartCmd(),
		newWorkerStopCmd(),
		newWorkerPauseCmd(),
		newWorkerResumeCmd(),
		newWorkerStatusCmd(),
		newWorkerAttachCmd(),
		newWorkerLogCmd(),
		newWorkerResetCmd(),
		newWorkerDeleteCmd(),
		newWorkerReassignCmd(),
	)

	return cmd
}

func newWorkerCreateCmd() *cobra.Command {
	var (
		alias string
		role  string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new worker",
		Long: `Create a new worker with an auto-generated NATO alphabet name.

Workers are created in idle status and can be started with 'forge worker start'.

Examples:
  forge worker create                      # Creates "alpha" (or next available)
  forge worker create --alias "api-dev"    # With alias
  forge worker create --role planner       # Create as planner role`,
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			r := worker.RoleWorker
			if role != "" {
				r, err = worker.ParseRole(role)
				if err != nil {
					return err
				}
			}

			w, err := reg.Create(r, alias)
			if err != nil {
				return fmt.Errorf("creating worker: %w", err)
			}

			fmt.Printf("%s  Created worker: %s", w.RoleIcon(), w.Name)
			if w.Alias != "" {
				fmt.Printf(" (alias: %s)", w.Alias)
			}
			fmt.Printf("\n")
			fmt.Printf("   ID: %s\n", w.ID)
			fmt.Printf("   Role: %s\n", w.Role)
			fmt.Printf("   Status: %s\n", w.Status)

			return nil
		},
	}

	cmd.Flags().StringVar(&alias, "alias", "", "Optional human-friendly alias")
	cmd.Flags().StringVar(&role, "role", "worker", "Worker role (worker, planner, reviewer, merge, deploy)")

	return cmd
}

func newWorkerListCmd() *cobra.Command {
	var (
		active bool
		idle   bool
		role   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workers",
		Long: `List all workers in the registry.

Examples:
  forge worker list           # All workers
  forge worker list --active  # Only active workers
  forge worker list --idle    # Only idle workers`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			// Build status filter
			var statuses []worker.Status
			if active {
				statuses = append(statuses, worker.StatusActive)
			}
			if idle {
				statuses = append(statuses, worker.StatusIdle)
			}

			workers := reg.List(statuses...)

			// Filter by role if specified
			if role != "" {
				r, err := worker.ParseRole(role)
				if err != nil {
					return err
				}
				filtered := make([]*worker.Worker, 0)
				for _, w := range workers {
					if w.Role == r {
						filtered = append(filtered, w)
					}
				}
				workers = filtered
			}

			if len(workers) == 0 {
				fmt.Println("No workers found. Create one with 'forge worker create'.")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tSTATUS\tROLE\tTASK\tWORKTREE")

			for _, w := range workers {
				task := "-"
				if w.CurrentTask != "" {
					task = w.CurrentTask
					if len(task) > 20 {
						task = task[:17] + "..."
					}
				}

				worktree := "-"
				if w.Worktree != "" {
					worktree = w.Worktree
					if len(worktree) > 30 {
						worktree = "..." + worktree[len(worktree)-27:]
					}
				}

				name := w.Name
				if w.Alias != "" {
					name = fmt.Sprintf("%s (%s)", w.Name, w.Alias)
				}

				fmt.Fprintf(tw, "%s\t%s %s\t%s\t%s\t%s\n",
					name,
					w.StatusIcon(),
					w.Status,
					w.Role,
					task,
					worktree,
				)
			}

			tw.Flush()

			// Show summary
			counts := reg.CountByStatus()
			fmt.Printf("\nTotal: %d workers (%d active, %d idle, %d paused, %d stopped)\n",
				reg.Count(),
				counts[worker.StatusActive],
				counts[worker.StatusIdle],
				counts[worker.StatusPaused],
				counts[worker.StatusStopped],
			)

			return nil
		},
	}

	cmd.Flags().BoolVar(&active, "active", false, "Show only active workers")
	cmd.Flags().BoolVar(&idle, "idle", false, "Show only idle workers")
	cmd.Flags().StringVar(&role, "role", "", "Filter by role")

	return cmd
}

func newWorkerStartCmd() *cobra.Command {
	var (
		task      string
		worktree  string
		prompt    string
		promise   string
		mcpConfig string
	)

	cmd := &cobra.Command{
		Use:   "start <worker>",
		Short: "Start a worker on a task",
		Long: `Start a worker with an optional task assignment.

The worker will run in a tmux session and maintain its identity across iterations.

Examples:
  forge worker start alpha --task auth --worktree .forge/worktrees/api/auth
  forge worker start bravo --prompt "Implement user authentication" --promise "DONE"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			// Default prompt if not specified
			if prompt == "" {
				if task != "" {
					prompt = fmt.Sprintf("Work on task: %s", task)
				} else {
					prompt = "Await instructions and assist with development tasks."
				}
			}

			opts := worker.StartOptions{
				TaskID:    task,
				Worktree:  worktree,
				Prompt:    prompt,
				Promise:   promise,
				MCPConfig: mcpConfig,
			}

			err = worker.Start(context.Background(), reg, w.ID, opts)
			if err != nil {
				if errors.Is(err, agent.ErrMaxIterationsReached) {
					fmt.Println("\nMax iterations reached. Use 'forge worker attach' to view the session.")
					return nil
				}
				if errors.Is(err, agent.ErrCancelled) {
					return nil
				}
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&task, "task", "", "Task ID to assign")
	cmd.Flags().StringVar(&worktree, "worktree", "", "Worktree path")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Custom prompt")
	cmd.Flags().StringVar(&promise, "promise", "", "Completion promise (default: WORKER_NAME_COMPLETE)")
	cmd.Flags().StringVar(&mcpConfig, "mcp-config", "", "Custom MCP config path")

	return cmd
}

func newWorkerStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <worker>",
		Short: "Stop a running worker",
		Long: `Stop a worker's tmux session and mark it as stopped.

The worker's task and worktree assignments are preserved for potential reassignment.

Examples:
  forge worker stop alpha`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			if err := worker.Stop(reg, w.ID); err != nil {
				return err
			}

			fmt.Printf("Stopped worker: %s\n", w.DisplayName())
			return nil
		},
	}

	return cmd
}

func newWorkerPauseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pause <worker>",
		Short: "Pause a running worker",
		Long: `Pause a worker by suspending its tmux session.

The worker can be resumed later with 'forge worker resume'.

Examples:
  forge worker pause alpha`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			if err := worker.Pause(reg, w.ID); err != nil {
				return err
			}

			fmt.Printf("Paused worker: %s\n", w.DisplayName())
			return nil
		},
	}

	return cmd
}

func newWorkerResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume <worker>",
		Short: "Resume a paused worker",
		Long: `Resume a paused worker's tmux session.

Examples:
  forge worker resume alpha`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			if err := worker.Resume(reg, w.ID); err != nil {
				return err
			}

			fmt.Printf("Resumed worker: %s\n", w.DisplayName())
			return nil
		},
	}

	return cmd
}

func newWorkerStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <worker>",
		Short: "Show detailed worker status",
		Long: `Show detailed status information for a worker.

Examples:
  forge worker status alpha`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			fmt.Printf("%s  %s", w.RoleIcon(), w.Name)
			if w.Alias != "" {
				fmt.Printf(" (%s)", w.Alias)
			}
			fmt.Println()
			fmt.Println()

			fmt.Printf("   ID:          %s\n", w.ID)
			fmt.Printf("   Status:      %s %s\n", w.StatusIcon(), w.Status)
			fmt.Printf("   Role:        %s\n", w.Role)

			if w.CurrentTask != "" {
				fmt.Printf("   Task:        %s\n", w.CurrentTask)
			}
			if w.Worktree != "" {
				fmt.Printf("   Worktree:    %s\n", w.Worktree)
			}
			if w.SessionID != "" {
				fmt.Printf("   Session:     %s\n", w.SessionID)

				// Get tmux session info
				if info, err := tmux.GetSessionInfo(w.SessionID); err == nil {
					status := "running"
					if info.Attached {
						status = "attached"
					}
					fmt.Printf("   Tmux:        %s (%dx%d)\n", status, info.Width, info.Height)
				}
			}

			fmt.Printf("   Created:     %s\n", w.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("   Last Active: %s\n", w.LastActive.Format("2006-01-02 15:04:05"))

			// Show memory group
			fmt.Printf("   Memory:      %s\n", w.GraphitiGroupID())

			return nil
		},
	}

	return cmd
}

func newWorkerAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach <worker>",
		Short: "Attach to a worker's tmux session",
		Long: `Attach to the tmux session of a running worker.

Use Ctrl+B, D to detach without stopping the worker.

Examples:
  forge worker attach alpha`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			if w.SessionID == "" {
				return fmt.Errorf("worker %s has no active session", w.DisplayName())
			}

			// Check session exists
			session := tmux.NewSession(w.SessionID, "", "")
			if !session.Exists() {
				return fmt.Errorf("tmux session %s not found", w.SessionID)
			}

			// Attach interactively
			attachCmd := exec.Command("tmux", "attach", "-t", w.SessionID)
			attachCmd.Stdin = os.Stdin
			attachCmd.Stdout = os.Stdout
			attachCmd.Stderr = os.Stderr

			return attachCmd.Run()
		},
	}

	return cmd
}

func newWorkerLogCmd() *cobra.Command {
	var (
		lines  int
		follow bool
	)

	cmd := &cobra.Command{
		Use:   "log <worker>",
		Short: "View worker output",
		Long: `View recent output from a worker's tmux session.

Examples:
  forge worker log alpha
  forge worker log alpha -n 50
  forge worker log alpha -f`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			output, err := worker.CaptureOutput(w, lines)
			if err != nil {
				return err
			}

			for _, line := range output {
				fmt.Println(line)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 20, "Number of lines to show")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow output (not yet implemented)")

	return cmd
}

func newWorkerResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset <worker>",
		Short: "Reset a stopped worker to idle",
		Long: `Reset a stopped worker to idle status, clearing task and worktree assignments.

Examples:
  forge worker reset alpha`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			if err := worker.Reset(reg, w.ID); err != nil {
				return err
			}

			fmt.Printf("Reset worker: %s\n", w.DisplayName())
			return nil
		},
	}

	return cmd
}

func newWorkerDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <worker>",
		Short: "Delete a worker",
		Long: `Delete a worker from the registry.

Active workers cannot be deleted unless --force is used.

Examples:
  forge worker delete alpha
  forge worker delete alpha --force`,
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workerID := args[0]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			w := reg.Get(workerID)
			if w == nil {
				return fmt.Errorf("worker not found: %s", workerID)
			}

			if w.Status == worker.StatusActive && !force {
				return fmt.Errorf("worker %s is active; use --force to delete", w.DisplayName())
			}

			// Stop if active
			if w.Status == worker.StatusActive || w.Status == worker.StatusPaused {
				if err := worker.Stop(reg, w.ID); err != nil {
					return fmt.Errorf("stopping worker: %w", err)
				}
			}

			if err := reg.Delete(w.ID); err != nil {
				return err
			}

			fmt.Printf("Deleted worker: %s\n", w.DisplayName())
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force delete active workers")

	return cmd
}

func newWorkerReassignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reassign <from-worker> <to-worker>",
		Short: "Reassign a task from one worker to another",
		Long: `Reassign a task and worktree from one worker to another.

The source worker will be stopped and its task/worktree transferred to the target.

Examples:
  forge worker reassign alpha bravo`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromID := args[0]
			toID := args[1]

			reg, err := worker.LoadRegistry()
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}

			from := reg.Get(fromID)
			if from == nil {
				return fmt.Errorf("source worker not found: %s", fromID)
			}

			to := reg.Get(toID)
			if to == nil {
				return fmt.Errorf("target worker not found: %s", toID)
			}

			if err := worker.Reassign(reg, from.ID, to.ID); err != nil {
				return err
			}

			fmt.Printf("Reassigned task from %s to %s\n", from.DisplayName(), to.DisplayName())
			if from.CurrentTask != "" {
				fmt.Printf("   Task: %s\n", from.CurrentTask)
			}
			if from.Worktree != "" {
				fmt.Printf("   Worktree: %s\n", from.Worktree)
			}

			return nil
		},
	}

	return cmd
}

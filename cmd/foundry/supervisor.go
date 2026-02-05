package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/detector"
	"github.com/zach-source/forge/internal/health"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/leader"
	"github.com/zach-source/forge/internal/supervisor/dashboard"
	"github.com/zach-source/forge/internal/tmux"
	"github.com/zach-source/forge/internal/webhooks"
	"github.com/zach-source/forge/internal/worker"
)

func newSupervisorCmd() *cobra.Command {
	var (
		interval             time.Duration
		analyzeInterval      time.Duration
		stuckThreshold       time.Duration
		maxPokes             int
		maxConcurrentWorkers int
		workDir              string
		autoAssign           bool
		withLeaders          bool
		leadersFlag          string
		noLeadersFlag        string
		autoRequeue          bool
		cleanupOrphans       bool
		dryRun               bool
		smartMode            bool
		githubSync           bool
		stateLog             string
		dashboardMode        bool
		cpuThreshold         float64
		memThreshold         float64
		failureThreshold     float64
		noThrottle           bool
	)

	cmd := &cobra.Command{
		Use:   "supervisor",
		Short: "Orchestrate workers and leaders to complete kanban tasks",
		Long: `Run a supervisor that orchestrates the full development workflow.

The supervisor manages:
1. Workers - Execute tasks from the kanban board
2. Reviewer - Reviews completed work and creates issues
3. Planner - Plans new tasks and verifies progress
4. Merge - Coordinates merging completed work
5. Deploy - Handles deployment when ready
6. Analyzer - Checks completed work for follow-up tasks

Task Analysis:
- Automatically requeues stuck tasks (in_progress with no worker)
- Analyzes completed work to discover new tasks
- Checks for incomplete work or TODO comments

Workflow: todo -> in_progress (worker) -> review (reviewer) -> done

Examples:
  foundry supervisor                    # Default 2 minute interval
  foundry supervisor --interval 1m      # Check every minute
  foundry supervisor --leaders          # Enable all leader agents
  foundry supervisor --no-auto-assign   # Only monitor, don't start new tasks
  foundry supervisor --stuck 5m         # Requeue tasks stuck for 5 minutes`,
		Aliases: []string{"sup", "orchestrate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if workDir == "" {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
				workDir = wd
			}
			// Parse leader flags
			enabledLeaders := parseLeaderFlags(leadersFlag, noLeadersFlag, withLeaders)

			// Build health thresholds from flags
			thresholds := health.DefaultThresholds()
			thresholds.CPUDegraded = cpuThreshold
			thresholds.CPUCritical = cpuThreshold + 10 // critical is 10% above degraded
			thresholds.MemoryDegraded = memThreshold
			thresholds.MemoryCritical = memThreshold + 10
			thresholds.FailureThreshold = failureThreshold

			return runSupervisor(cmd.Context(), supervisorConfig{
				interval:             interval,
				analyzeInterval:      analyzeInterval,
				stuckThreshold:       stuckThreshold,
				maxPokes:             maxPokes,
				maxConcurrentWorkers: maxConcurrentWorkers,
				workDir:              workDir,
				autoAssign:           autoAssign,
				withLeaders:          withLeaders || leadersFlag != "",
				enabledLeaders:       enabledLeaders,
				autoRequeue:          autoRequeue,
				cleanupOrphans:       cleanupOrphans,
				dryRun:               dryRun,
				smartMode:            smartMode,
				githubSync:           githubSync,
				stateLog:             stateLog,
				dashboardMode:        dashboardMode,
				healthThresholds:     thresholds,
				noThrottle:           noThrottle,
			})
		},
	}

	cmd.Flags().DurationVarP(&interval, "interval", "i", 2*time.Minute, "Time between checks")
	cmd.Flags().DurationVar(&analyzeInterval, "analyze-interval", 10*time.Minute, "Time between task analysis runs")
	cmd.Flags().DurationVar(&stuckThreshold, "stuck", 10*time.Minute, "Requeue tasks stuck longer than this")
	cmd.Flags().IntVar(&maxPokes, "max-pokes", 10, "Max pokes per worker before escalating (0 = unlimited)")
	cmd.Flags().IntVar(&maxConcurrentWorkers, "max-workers", 4, "Maximum concurrent development workers")
	cmd.Flags().StringVarP(&workDir, "dir", "d", "", "Working directory (default: current)")
	cmd.Flags().BoolVar(&autoAssign, "auto-assign", true, "Automatically assign tasks to idle workers")
	cmd.Flags().BoolVar(&withLeaders, "leaders", false, "Enable all leader agents (planner, reviewer, merge, deploy)")
	cmd.Flags().StringVar(&leadersFlag, "enable-leaders", "", "Enable specific leaders (comma-separated: groomer,monitor,reviewer,planner,merge,deploy,tester,pm)")
	cmd.Flags().StringVar(&noLeadersFlag, "disable-leaders", "", "Disable specific leaders (use with --leaders or --enable-leaders)")
	cmd.Flags().BoolVar(&dashboardMode, "dashboard", false, "Run with live TUI dashboard")
	cmd.Flags().BoolVar(&autoRequeue, "auto-requeue", true, "Automatically requeue stuck tasks")
	cmd.Flags().BoolVar(&cleanupOrphans, "cleanup-orphans", true, "Clean up orphaned tmux sessions on startup (disable with --cleanup-orphans=false)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be cleaned without taking action")
	cmd.Flags().BoolVar(&smartMode, "smart", false, "Use AI (Haiku) to make orchestration decisions")
	cmd.Flags().BoolVar(&githubSync, "github-sync", false, "Sync beads to GitHub Projects each cycle")
	cmd.Flags().StringVar(&stateLog, "state-log", "", "Path to state log file (JSON lines)")
	cmd.Flags().Float64Var(&cpuThreshold, "cpu-threshold", 80.0, "Degrade assignment above this CPU%")
	cmd.Flags().Float64Var(&memThreshold, "mem-threshold", 80.0, "Degrade assignment above this memory%")
	cmd.Flags().Float64Var(&failureThreshold, "failure-threshold", 0.5, "Pause assignment when failure rate exceeds this (0.0-1.0)")
	cmd.Flags().BoolVar(&noThrottle, "no-throttle", false, "Disable health-based throttling")

	return cmd
}

type supervisorConfig struct {
	interval             time.Duration
	analyzeInterval      time.Duration
	stuckThreshold       time.Duration
	maxPokes             int
	maxConcurrentWorkers int
	workDir              string
	autoAssign           bool
	withLeaders          bool
	enabledLeaders       map[string]bool
	autoRequeue          bool
	cleanupOrphans       bool
	dryRun               bool
	smartMode            bool
	githubSync           bool
	stateLog             string
	dashboardMode        bool
	webhookDispatcher    *webhooks.Dispatcher
	healthThresholds     health.ThresholdConfig
	noThrottle           bool
}

type leaderState struct {
	running   bool
	sessionID string
	startedAt time.Time
}

// allLeaderRoles lists all available leader roles.
var allLeaderRoles = []string{"planner", "reviewer", "merge", "deploy", "groomer", "monitor", "tester", "pm", "cicd"}

// parseLeaderFlags parses --enable-leaders and --disable-leaders flags.
// Returns a map of enabled leader roles.
func parseLeaderFlags(leadersFlag, noLeadersFlag string, withLeaders bool) map[string]bool {
	enabled := make(map[string]bool)

	// If --leaders is set, enable all
	if withLeaders {
		for _, role := range allLeaderRoles {
			enabled[role] = true
		}
	}

	// If --enable-leaders is specified, parse it
	if leadersFlag != "" {
		if leadersFlag == "all" {
			for _, role := range allLeaderRoles {
				enabled[role] = true
			}
		} else {
			// Parse comma-separated list
			for _, role := range strings.Split(leadersFlag, ",") {
				role = strings.TrimSpace(strings.ToLower(role))
				if role != "" {
					enabled[role] = true
				}
			}
		}
	}

	// Remove disabled leaders
	if noLeadersFlag != "" {
		for _, role := range strings.Split(noLeadersFlag, ",") {
			role = strings.TrimSpace(strings.ToLower(role))
			delete(enabled, role)
		}
	}

	return enabled
}

// isLeaderEnabled checks if a specific leader role is enabled.
func isLeaderEnabled(cfg supervisorConfig, role string) bool {
	// If no leaders are enabled at all, check withLeaders flag for backwards compat
	if len(cfg.enabledLeaders) == 0 {
		return cfg.withLeaders
	}
	return cfg.enabledLeaders[strings.ToLower(role)]
}

type supervisorState struct {
	pokeCounts     map[string]int    // worker ID -> poke count
	taskWorkers    map[string]string // task ID -> worker ID
	workerTasks    map[string]string // worker ID -> task ID
	completedTasks map[string]bool   // completed task IDs

	// Task timing
	taskStarted   map[string]time.Time // task ID -> when it entered in_progress
	taskCompleted map[string]time.Time // task ID -> when it was completed (grace period for requeue)

	// Smart poke tracking
	lastPaneOutput       map[string]string // worker ID -> last captured output
	unchangedOutputCount map[string]int    // worker ID -> count of cycles with same output

	// Leader states
	planner  leaderState
	reviewer leaderState
	merge    leaderState
	deploy   leaderState
	analyzer leaderState // for task analysis
	groomer  leaderState // for backlog grooming
	monitor  leaderState // for infrastructure monitoring
	tester   leaderState // for UI/API testing
	pm       leaderState // for project management
	cicd     leaderState // for CI/CD monitoring and fixes

	// Leader scheduling
	leaderLastRun       map[string]time.Time // role -> last run start time
	leadersJustFinished map[string]bool      // leaders that finished this cycle (for cooldown reset)

	// Workflow tracking
	allTasksDone    bool
	mergeCompleted  bool
	deployCompleted bool
	lastMergeCount  int // track Merge queue count to detect new tasks ready for merge

	// Analysis tracking
	lastAnalysis time.Time // when we last ran the analyzer

	// Health tracking
	failureTracker  *health.FailureTracker
	apiErrorTracker *health.APIErrorTracker
	lastHealth      *health.Metrics
}

// LeaderTrigger defines when a leader should be activated.
type LeaderTrigger struct {
	Role      string
	Condition func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool
	Cooldown  time.Duration
}

// leaderTriggers defines activation conditions and cooldowns for each leader.
var leaderTriggers = []LeaderTrigger{
	{
		Role: "groomer",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			return counts[kanban.StatusBacklog] > 0 && !state.groomer.running
		},
		Cooldown: 15 * time.Minute,
	},
	{
		Role: "reviewer",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			return counts[kanban.StatusReview] > 0 && !state.reviewer.running
		},
		Cooldown: 5 * time.Minute,
	},
	{
		Role: "planner",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			// Run when no work in progress and we need to plan
			needsPlanning := counts[kanban.StatusTodo] == 0 && counts[kanban.StatusInProgress] == 0 &&
				counts[kanban.StatusReview] == 0 && counts[kanban.StatusBacklog] > 0
			return needsPlanning && !state.planner.running && !workerActive
		},
		Cooldown: 20 * time.Minute,
	},
	{
		Role: "deploy",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			hasMergeTasks := counts[kanban.StatusMerge] > 0
			return hasMergeTasks && !state.deploy.running && !state.deployCompleted
		},
		Cooldown: 10 * time.Minute,
	},
	{
		Role: "monitor",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			return !state.monitor.running // Always run if not running
		},
		Cooldown: 5 * time.Minute,
	},
	{
		Role: "tester",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			// Run when there's active work to test
			return counts[kanban.StatusInProgress] > 0 && !state.tester.running
		},
		Cooldown: 10 * time.Minute,
	},
	{
		Role: "pm",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			// Run when there are done tasks to analyze
			return counts[kanban.StatusDone] >= 3 && !state.pm.running
		},
		Cooldown: 30 * time.Minute,
	},
	{
		Role: "cicd",
		Condition: func(counts map[kanban.Status]int, state *supervisorState, workerActive bool) bool {
			// Run periodically to check CI health, or when deploy completes
			return !state.cicd.running
		},
		Cooldown: 15 * time.Minute,
	},
}

// shouldStartLeader checks if a leader should be started based on triggers and cooldowns.
func shouldStartLeader(role string, counts map[kanban.Status]int, state *supervisorState, cfg supervisorConfig, workerActive bool) bool {
	// Check if leader is enabled
	if !isLeaderEnabled(cfg, role) {
		return false
	}

	// Find trigger for this role
	var trigger *LeaderTrigger
	for i := range leaderTriggers {
		if leaderTriggers[i].Role == role {
			trigger = &leaderTriggers[i]
			break
		}
	}

	if trigger == nil {
		return false
	}

	// Check if the condition is met first
	conditionMet := trigger.Condition(counts, state, workerActive)
	if !conditionMet {
		return false
	}

	// Check cooldown - but allow faster retry if leader just finished and work still pending
	if lastRun, ok := state.leaderLastRun[role]; ok {
		timeSinceRun := time.Since(lastRun)
		if timeSinceRun < trigger.Cooldown {
			// If this leader just finished but condition still met, use shorter retry cooldown
			if state.leadersJustFinished != nil && state.leadersJustFinished[role] {
				// Leader finished but work still pending - use 30s retry instead of full cooldown
				retryCooldown := 30 * time.Second
				if timeSinceRun < retryCooldown {
					return false
				}
				fmt.Printf("   🔄 %s finished but work still pending, restarting...\n", role)
				return true
			}
			return false
		}
	}

	return true
}

// recordLeaderStart records when a leader was started for cooldown tracking.
func recordLeaderStart(state *supervisorState, role string) {
	if state.leaderLastRun == nil {
		state.leaderLastRun = make(map[string]time.Time)
	}
	state.leaderLastRun[role] = time.Now()
}

// PersistentState is saved to disk to survive supervisor restarts.
type PersistentState struct {
	StartedAt      time.Time               `json:"started_at"`
	LastCycle      time.Time               `json:"last_cycle"`
	CycleCount     int                     `json:"cycle_count"`
	Leaders        map[string]LeaderPState `json:"leaders"`
	TaskWorkers    map[string]string       `json:"task_workers"`    // taskID -> workerID
	CompletedTasks map[string]time.Time    `json:"completed_tasks"` // taskID -> completion time
	LeaderLastRun  map[string]time.Time    `json:"leader_last_run"` // role -> last run time
}

// LeaderPState is the persisted state for a single leader.
type LeaderPState struct {
	Running    bool      `json:"running"`
	SessionID  string    `json:"session_id"`
	StartedAt  time.Time `json:"started_at"`
	LastActive time.Time `json:"last_active"`
	CycleCount int       `json:"cycle_count"` // cycles since leader started
}

// supervisorStatePath returns the path to the supervisor state file.
func supervisorStatePath(workDir string) string {
	return filepath.Join(workDir, ".forge", "supervisor", "state.json")
}

// loadSupervisorState loads persisted state from disk.
func loadSupervisorState(workDir string) (*PersistentState, error) {
	path := supervisorStatePath(workDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No state file yet
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var ps PersistentState
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	return &ps, nil
}

// saveSupervisorState saves persisted state to disk.
func saveSupervisorState(workDir string, ps *PersistentState) error {
	path := supervisorStatePath(workDir)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	return nil
}

// syncStateFromPersisted restores supervisor state from persisted state on startup.
func syncStateFromPersisted(state *supervisorState, ps *PersistentState, reg *worker.Registry) {
	if ps == nil {
		return
	}

	// Restore task workers mapping
	for taskID, workerID := range ps.TaskWorkers {
		state.taskWorkers[taskID] = workerID
		state.workerTasks[workerID] = taskID
	}

	// Restore completed tasks (convert time map to bool map)
	for taskID, completedAt := range ps.CompletedTasks {
		state.completedTasks[taskID] = true
		state.taskCompleted[taskID] = completedAt
	}

	// Restore leader last run times for cooldowns
	for role, lastRun := range ps.LeaderLastRun {
		state.leaderLastRun[role] = lastRun
	}

	// Validate and restore leader states (check if sessions still exist)
	for role, lps := range ps.Leaders {
		if !lps.Running || lps.SessionID == "" {
			continue
		}

		// Check if session still exists
		session := tmux.NewSession(lps.SessionID, "", "")
		if !session.Exists() {
			fmt.Printf("⚠️  Persisted %s session no longer exists\n", role)
			continue
		}

		// Restore leader state
		ls := getLeaderStateByRole(state, role)
		if ls != nil {
			ls.running = true
			ls.sessionID = lps.SessionID
			ls.startedAt = lps.StartedAt
			fmt.Printf("✅ Restored %s leader state (session: %s)\n", role, lps.SessionID)
		}
	}
}

// getLeaderStateByRole returns a pointer to the leaderState for a given role.
func getLeaderStateByRole(state *supervisorState, role string) *leaderState {
	switch role {
	case "planner":
		return &state.planner
	case "reviewer":
		return &state.reviewer
	case "merge":
		return &state.merge
	case "deploy":
		return &state.deploy
	case "groomer":
		return &state.groomer
	case "monitor":
		return &state.monitor
	case "tester":
		return &state.tester
	case "pm":
		return &state.pm
	case "analyzer":
		return &state.analyzer
	default:
		return nil
	}
}

// buildPersistentState creates a PersistentState from current supervisor state.
func buildPersistentState(state *supervisorState, startedAt time.Time, cycleCount int) *PersistentState {
	ps := &PersistentState{
		StartedAt:      startedAt,
		LastCycle:      time.Now(),
		CycleCount:     cycleCount,
		Leaders:        make(map[string]LeaderPState),
		TaskWorkers:    make(map[string]string),
		CompletedTasks: make(map[string]time.Time),
		LeaderLastRun:  make(map[string]time.Time),
	}

	// Copy task workers
	for k, v := range state.taskWorkers {
		ps.TaskWorkers[k] = v
	}

	// Copy completed tasks with timestamps
	for taskID := range state.completedTasks {
		if completedAt, ok := state.taskCompleted[taskID]; ok {
			ps.CompletedTasks[taskID] = completedAt
		} else {
			ps.CompletedTasks[taskID] = time.Now()
		}
	}

	// Copy leader last run times
	for k, v := range state.leaderLastRun {
		ps.LeaderLastRun[k] = v
	}

	// Build leader states
	leaders := []struct {
		role  string
		state *leaderState
	}{
		{"planner", &state.planner},
		{"reviewer", &state.reviewer},
		{"merge", &state.merge},
		{"deploy", &state.deploy},
		{"groomer", &state.groomer},
		{"monitor", &state.monitor},
		{"tester", &state.tester},
		{"pm", &state.pm},
		{"analyzer", &state.analyzer},
	}

	for _, l := range leaders {
		ps.Leaders[l.role] = LeaderPState{
			Running:    l.state.running,
			SessionID:  l.state.sessionID,
			StartedAt:  l.state.startedAt,
			LastActive: time.Now(),
		}
	}

	return ps
}

func newSupervisorState() *supervisorState {
	return &supervisorState{
		pokeCounts:           make(map[string]int),
		taskWorkers:          make(map[string]string),
		workerTasks:          make(map[string]string),
		completedTasks:       make(map[string]bool),
		taskStarted:          make(map[string]time.Time),
		taskCompleted:        make(map[string]time.Time),
		lastPaneOutput:       make(map[string]string),
		unchangedOutputCount: make(map[string]int),
		leaderLastRun:        make(map[string]time.Time),
		failureTracker:       health.NewFailureTracker(10),
		apiErrorTracker:      health.NewAPIErrorTracker(5 * time.Minute),
	}
}

// checkWorkerHealth verifies all workers marked Active have existing tmux sessions
// with Claude actually running. Stale workers are reset and leader state is cleared.
// For development workers, completed tasks are moved to review.
func checkWorkerHealth(ctx context.Context, reg *worker.Registry, state *supervisorState, store *kanban.Store, cfg supervisorConfig) {
	workers := reg.List(worker.StatusActive)

	for _, w := range workers {
		if w.SessionID == "" {
			continue
		}

		session := tmux.NewSession(w.SessionID, "", "")
		sessionExists := session.Exists()
		claudeRunning := sessionExists && session.IsClaudeRunning()

		// Determine reason for reset
		var reason string
		if !sessionExists {
			reason = "session gone"
		} else if !claudeRunning {
			reason = "Claude exited"
		}

		if reason != "" {
			// For leaders, check if they actually completed by looking for promise
			promiseFound := false
			if isLeaderRole(w.Role) {
				promiseFound = checkLeaderPromise(w, session, sessionExists)
			}

			// For development workers with tasks, move to review before resetting
			taskID := w.CurrentTask
			if w.Role == worker.RoleWorker && taskID != "" && !state.completedTasks[taskID] {
				fmt.Printf("✅ Worker %s finished task %s (%s)\n", w.DisplayName(), taskID, reason)

				// Record metrics before moving task
				startedAt := state.taskStarted[taskID]
				if startedAt.IsZero() {
					startedAt = w.LastActive // fallback
				}
				if err := worker.RecordWorkerTaskComplete(w.ID, w.Name, taskID, startedAt, w.SessionID, true); err != nil {
					fmt.Printf("   ⚠️  Error recording metrics: %v\n", err)
				}

				// Get task details for webhook
				task, _ := store.Get(taskID)
				var taskTitle string
				var taskPriority webhooks.Priority
				if task != nil {
					taskTitle = task.Title
					taskPriority = webhooks.Priority(task.Priority)
				}

				// Move task to review
				if err := store.Move(taskID, kanban.StatusReview); err != nil {
					fmt.Printf("   ⚠️  Error moving task to review: %v\n", err)
				} else {
					fmt.Printf("   📋 Moved task to Review\n")
					state.completedTasks[taskID] = true
					state.failureTracker.Record(true) // task completed successfully

					// Dispatch task_completed webhook
					if cfg.webhookDispatcher != nil {
						event := webhooks.NewTaskCompletedEvent(taskID, taskTitle, w.DisplayName(), taskPriority, 0)
						cfg.webhookDispatcher.Dispatch(ctx, event)
					}
				}
			} else if isLeaderRole(w.Role) {
				if promiseFound {
					fmt.Printf("✅ Leader %s completed (%s)\n", w.DisplayName(), reason)
				} else {
					fmt.Printf("⚠️  Leader %s exited without completion promise (%s)\n", w.DisplayName(), reason)
				}
			} else {
				fmt.Printf("🔄 Resetting stale worker %s (%s)\n", w.DisplayName(), reason)
			}

			// Stop and reset the worker
			if err := worker.Stop(reg, w.ID); err != nil {
				fmt.Printf("   ⚠️  Error stopping worker: %v\n", err)
			}
			if err := worker.Reset(reg, w.ID); err != nil {
				fmt.Printf("   ⚠️  Error resetting worker: %v\n", err)
			}

			// Dispatch worker_stopped webhook
			if cfg.webhookDispatcher != nil {
				event := webhooks.NewWorkerStoppedEvent(w.ID, w.DisplayName(), taskID, reason)
				cfg.webhookDispatcher.Dispatch(ctx, event)
			}

			// Clear any leader state if this was a leader
			clearLeaderState(w.Role, state, promiseFound)

			// Clear task tracking state
			if taskID != "" {
				delete(state.taskWorkers, taskID)
			}
			delete(state.workerTasks, w.ID)
			delete(state.pokeCounts, w.ID)
		}
	}
}

// isLeaderRole returns true if the role is a leader type (not a regular worker).
func isLeaderRole(role worker.Role) bool {
	switch role {
	case worker.RolePlanner, worker.RoleReviewer, worker.RoleMerge, worker.RoleDeploy, worker.RoleGroomer:
		return true
	default:
		return false
	}
}

// checkLeaderPromise checks if a leader session output contains its completion promise.
func checkLeaderPromise(w *worker.Worker, session *tmux.Session, sessionExists bool) bool {
	// Determine the expected promise based on role
	var expectedPromise string
	switch w.Role {
	case worker.RolePlanner:
		expectedPromise = leader.PromisePlanner
	case worker.RoleReviewer:
		expectedPromise = leader.PromiseReviewer
	case worker.RoleMerge:
		expectedPromise = leader.PromiseMerge
	case worker.RoleDeploy:
		expectedPromise = leader.PromiseDeploy
	case worker.RoleGroomer:
		expectedPromise = leader.PromiseGroomer
	default:
		return false
	}

	d := detector.New(expectedPromise)

	// Try to get output from log file first (more reliable)
	logPath := worker.LogPath(w)
	if logPath != "" {
		if content, err := os.ReadFile(logPath); err == nil && len(content) > 0 {
			if d.IsComplete(string(content)) {
				return true
			}
		}
	}

	// Fall back to capturing pane if session still exists
	if sessionExists {
		if content, err := session.CapturePane(); err == nil && content != "" {
			if d.IsComplete(content) {
				return true
			}
		}
	}

	return false
}

// clearLeaderState clears the in-memory leader state for a given role.
// Only sets workflow completion flags for merge/deploy if promiseFound is true.
func clearLeaderState(role worker.Role, state *supervisorState, promiseFound bool) {
	switch role {
	case worker.RolePlanner:
		state.planner.running = false
		state.planner.sessionID = ""
	case worker.RoleReviewer:
		state.reviewer.running = false
		state.reviewer.sessionID = ""
	case worker.RoleMerge:
		state.merge.running = false
		state.merge.sessionID = ""
		if promiseFound {
			state.mergeCompleted = true
			fmt.Printf("   ✅ Merge workflow completed (promise found)\n")
		} else {
			fmt.Printf("   ⚠️  Merge session ended without completing\n")
		}
	case worker.RoleDeploy:
		state.deploy.running = false
		state.deploy.sessionID = ""
		if promiseFound {
			state.deployCompleted = true
			fmt.Printf("   ✅ Deploy workflow completed (promise found)\n")
		} else {
			fmt.Printf("   ⚠️  Deploy session ended without completing\n")
		}
	case worker.RoleGroomer:
		state.groomer.running = false
		state.groomer.sessionID = ""
	}
}

// recoverMisplacedTasks checks for tasks in "done" that have unmerged branches
// and moves them back to "review". This handles cases where workers incorrectly
// moved tasks directly to done, bypassing the review step.
//
// NOTE: This is disabled by default. The proper workflow is:
// 1. Worker completes -> Review
// 2. Reviewer validates -> Done
// 3. Merge leader merges branches for Done tasks
//
// If tasks are in Done with unmerged branches, that's expected - the merge
// leader will handle them. This recovery only makes sense if we want to
// enforce that branches must be merged before marking done.
func recoverMisplacedTasks(store *kanban.Store, state *supervisorState, workDir string) {
	// Skip recovery - let merge leader handle unmerged branches
	// Tasks in Done are waiting for merge, not incorrectly placed
}

// recoverOrphanedTasks moves in-progress tasks with no assigned worker back to todo.
// This handles cases where workers crashed/reset but tasks weren't cleaned up.
func recoverOrphanedTasks(store *kanban.Store, reg *worker.Registry, state *supervisorState) {
	inProgressTasks, err := store.List(kanban.StatusInProgress)
	if err != nil {
		return
	}

	// Build set of tasks that have active workers
	activeTaskIDs := make(map[string]bool)
	for _, w := range reg.List(worker.StatusActive) {
		if w.CurrentTask != "" && w.Role == worker.RoleWorker {
			activeTaskIDs[w.CurrentTask] = true
		}
	}

	// Also check state.taskWorkers for tasks being actively tracked
	for taskID := range state.taskWorkers {
		activeTaskIDs[taskID] = true
	}

	for _, task := range inProgressTasks {
		if !activeTaskIDs[task.ID] && !state.completedTasks[task.ID] {
			// Task is in progress but no worker is working on it
			fmt.Printf("🔄 Recovering orphaned task %s back to todo\n", task.ID)
			fmt.Printf("   Title: %s\n", task.Title)
			if err := store.Move(task.ID, kanban.StatusTodo); err != nil {
				fmt.Printf("   ⚠️  Error moving task: %v\n", err)
			} else {
				fmt.Printf("   ✅ Moved to todo\n")
				// Clear tracking state
				delete(state.taskStarted, task.ID)
			}
		}
	}
}

// syncLeaderStateFromRegistry initializes leaderState from registry on startup.
// This restores state for leaders that were running before supervisor restart.
func syncLeaderStateFromRegistry(reg *worker.Registry, state *supervisorState) {
	workers := reg.List(worker.StatusActive)

	for _, w := range workers {
		if w.SessionID == "" {
			continue
		}

		// Check if session actually exists
		session := tmux.NewSession(w.SessionID, "", "")
		if !session.Exists() {
			continue // Will be cleaned up by health check
		}

		// Restore leader state
		switch w.Role {
		case worker.RolePlanner:
			state.planner.running = true
			state.planner.sessionID = w.SessionID
			state.planner.startedAt = w.LastActive
			fmt.Printf("📋 Restored planner state from %s\n", w.DisplayName())
		case worker.RoleReviewer:
			state.reviewer.running = true
			state.reviewer.sessionID = w.SessionID
			state.reviewer.startedAt = w.LastActive
			fmt.Printf("🔍 Restored reviewer state from %s\n", w.DisplayName())
		case worker.RoleMerge:
			state.merge.running = true
			state.merge.sessionID = w.SessionID
			state.merge.startedAt = w.LastActive
			fmt.Printf("🔀 Restored merge state from %s\n", w.DisplayName())
		case worker.RoleDeploy:
			state.deploy.running = true
			state.deploy.sessionID = w.SessionID
			state.deploy.startedAt = w.LastActive
			fmt.Printf("🚀 Restored deploy state from %s\n", w.DisplayName())
		case worker.RoleGroomer:
			state.groomer.running = true
			state.groomer.sessionID = w.SessionID
			state.groomer.startedAt = w.LastActive
			fmt.Printf("🧹 Restored groomer state from %s\n", w.DisplayName())
		}
	}
}

// cleanupOrphanedSessions finds tmux sessions with forge- prefix not in registry.
func cleanupOrphanedSessions(reg *worker.Registry, dryRun bool) {
	// Prefixes for forge-related tmux sessions (including legacy naming)
	prefixes := []string{"forge-", "mforge-", "mf-"}

	// Collect all sessions matching any prefix
	var allSessions []string
	for _, prefix := range prefixes {
		sessions, err := tmux.ListSessions(prefix)
		if err != nil {
			fmt.Printf("⚠️  Error listing tmux sessions with prefix %s: %v\n", prefix, err)
			continue
		}
		allSessions = append(allSessions, sessions...)
	}

	if len(allSessions) == 0 {
		return
	}

	// Build set of known session IDs from registry
	knownSessions := make(map[string]bool)
	for _, w := range reg.List() {
		if w.SessionID != "" {
			knownSessions[w.SessionID] = true
		}
	}

	// Find and cleanup orphaned sessions
	orphanCount := 0
	for _, sessionID := range allSessions {
		if !knownSessions[sessionID] {
			orphanCount++
			if dryRun {
				fmt.Printf("🗑️  Would cleanup orphan: %s\n", sessionID)
			} else {
				fmt.Printf("🗑️  Cleaning up orphan: %s\n", sessionID)
				session := tmux.NewSession(sessionID, "", "")
				if err := session.Kill(); err != nil {
					fmt.Printf("   ⚠️  Error killing session: %v\n", err)
				}
			}
		}
	}

	if orphanCount == 0 {
		fmt.Println("✅ No orphaned sessions found")
	} else if dryRun {
		fmt.Printf("📊 Found %d orphaned session(s) (dry-run, no action taken)\n", orphanCount)
	} else {
		fmt.Printf("📊 Cleaned up %d orphaned session(s)\n", orphanCount)
	}
}

// runDashboardMode runs the supervisor with a live TUI dashboard.
func runDashboardMode(cfg supervisorConfig) error {
	m := dashboard.NewModel(cfg.workDir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runSupervisor(ctx context.Context, cfg supervisorConfig) error {
	// Dashboard mode - run TUI instead of standard output
	if cfg.dashboardMode {
		return runDashboardMode(cfg)
	}

	fmt.Printf("🎯 Supervisor starting\n")
	fmt.Printf("   Interval: %s\n", cfg.interval)
	fmt.Printf("   Work dir: %s\n", cfg.workDir)
	fmt.Printf("   Max workers: %d\n", cfg.maxConcurrentWorkers)
	fmt.Printf("   Auto-assign: %v\n", cfg.autoAssign)

	// Display enabled leaders
	if len(cfg.enabledLeaders) > 0 {
		var enabled []string
		for _, role := range allLeaderRoles {
			if cfg.enabledLeaders[role] {
				enabled = append(enabled, role)
			}
		}
		fmt.Printf("   Leaders: %s\n", strings.Join(enabled, ", "))
	} else if cfg.withLeaders {
		fmt.Printf("   Leaders: all\n")
	} else {
		fmt.Printf("   Leaders: none\n")
	}

	if cfg.noThrottle {
		fmt.Printf("   Throttle: disabled\n")
	} else {
		fmt.Printf("   Throttle: CPU %.0f%%, Mem %.0f%%, Fail %.0f%%\n",
			cfg.healthThresholds.CPUDegraded, cfg.healthThresholds.MemoryDegraded, cfg.healthThresholds.FailureThreshold*100)
	}

	// Load webhook configuration
	webhookCfg, err := webhooks.LoadConfig(cfg.workDir)
	if err != nil {
		fmt.Printf("⚠️  Error loading webhooks config: %v\n", err)
	} else if webhookCfg != nil {
		cfg.webhookDispatcher = webhooks.NewDispatcher(webhookCfg)
		fmt.Printf("   Webhooks: %d endpoint(s) configured\n", len(webhookCfg.Webhooks))
	}

	fmt.Printf("   Press Ctrl+C to stop\n\n")

	// Set up signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n👋 Supervisor stopping...")
		cancel()
	}()

	state := newSupervisorState()
	startedAt := time.Now()
	cycleCount := 0

	// Load registry for startup initialization
	reg, err := worker.LoadRegistry()
	if err != nil {
		fmt.Printf("⚠️  Error loading workers for startup: %v\n", err)
	} else {
		// Try to load persisted state
		fmt.Println("🔄 Checking for persisted state...")
		if ps, err := loadSupervisorState(cfg.workDir); err != nil {
			fmt.Printf("⚠️  Error loading persisted state: %v\n", err)
		} else if ps != nil {
			startedAt = ps.StartedAt
			cycleCount = ps.CycleCount
			syncStateFromPersisted(state, ps, reg)
			fmt.Printf("✅ Restored state from previous session (cycle %d)\n", cycleCount)
		}

		// Sync leader state from registry (recovers state after restart)
		fmt.Println("🔄 Checking for running leaders...")
		syncLeaderStateFromRegistry(reg, state)

		// Optional cleanup of orphaned sessions
		if cfg.cleanupOrphans {
			fmt.Println("🧹 Checking for orphaned sessions...")
			cleanupOrphanedSessions(reg, cfg.dryRun)
		}
	}

	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	// Determine which cycle function to use
	cycleFunc := runCycle
	if cfg.smartMode {
		fmt.Printf("   🤖 Smart mode: AI-powered orchestration\n\n")
		cycleFunc = runSmartCycle
	}

	// Helper to run cycle and save state
	runAndSave := func() {
		cycleCount++
		cycleFunc(ctx, cfg, state)

		// Save state after each cycle
		ps := buildPersistentState(state, startedAt, cycleCount)
		if err := saveSupervisorState(cfg.workDir, ps); err != nil {
			fmt.Printf("⚠️  Error saving state: %v\n", err)
		}
	}

	// Initial run
	runAndSave()

	for {
		select {
		case <-ctx.Done():
			// Save final state on shutdown
			ps := buildPersistentState(state, startedAt, cycleCount)
			if err := saveSupervisorState(cfg.workDir, ps); err != nil {
				fmt.Printf("⚠️  Error saving final state: %v\n", err)
			}
			return nil
		case <-ticker.C:
			runAndSave()
		}
	}
}

func runCycle(ctx context.Context, cfg supervisorConfig, state *supervisorState) {
	now := time.Now().Format("15:04:05")
	fmt.Printf("\n━━━ Cycle %s ━━━\n", now)

	// Load kanban store
	store, err := getKanbanStoreForDir(cfg.workDir)
	if err != nil {
		fmt.Printf("⚠️  Error loading kanban: %v\n", err)
		return
	}
	defer func() { _ = store.Close() }()

	// Load worker registry
	reg, err := worker.LoadRegistry()
	if err != nil {
		fmt.Printf("⚠️  Error loading workers: %v\n", err)
		return
	}

	// 0. System health check - throttle if resources are constrained
	if !cfg.noThrottle {
		metrics, err := health.Check(cfg.healthThresholds, state.failureTracker, state.apiErrorTracker, health.GetResourceMetrics)
		if err != nil {
			fmt.Printf("⚠️  Health check error: %v\n", err)
		} else {
			state.lastHealth = metrics
			switch metrics.Status {
			case health.StatusCritical:
				fmt.Printf("🛑 Critical: %s — pausing all assignment\n", metrics.Reason)
				printHealthMetrics(metrics)
				printSummary(store, reg, state)
				return
			case health.StatusDegraded:
				fmt.Printf("⚠️  Degraded: %s — reducing worker limit\n", metrics.Reason)
				printHealthMetrics(metrics)
				cfg.maxConcurrentWorkers = max(1, cfg.maxConcurrentWorkers/2)
			}
		}
	}

	// 0a. Worker health check - detect stale workers with missing tmux sessions
	checkWorkerHealth(ctx, reg, state, store, cfg)

	// 0.5. Recovery - check for misplaced tasks (done with unmerged branches)
	// This catches tasks that workers incorrectly moved to done
	recoverMisplacedTasks(store, state, cfg.workDir)

	// 0.6. Recovery - check for orphaned in-progress tasks (no active worker)
	recoverOrphanedTasks(store, reg, state)

	// 1. Check for completed workers and update tasks
	checkCompletedWorkers(store, reg, state, cfg.workDir)

	// 2. Analyze tasks - check for stuck/abandoned tasks that need requeuing
	analyzeAndRequeueTasks(ctx, store, reg, state, cfg)

	// 3. Check leader sessions
	if cfg.withLeaders {
		checkLeaderSessions(reg, state)
	}

	// 4. Poke active workers
	pokeActiveWorkers(reg, state, cfg.maxPokes)

	// 5. Assign idle workers to todo tasks
	if cfg.autoAssign {
		assignTasks(ctx, store, reg, state, cfg)
	}

	// 6. Run leader workflow if enabled
	if cfg.withLeaders {
		runLeaderWorkflow(store, reg, state, cfg)
	}

	// 7. Run task analyzer if needed (check for new tasks from completed work)
	if cfg.withLeaders {
		checkForNewTasks(store, reg, state, cfg.workDir, cfg)
	}

	// 8. Summary
	printSummary(store, reg, state)

	// 9. GitHub sync (if enabled)
	if cfg.githubSync {
		syncToGitHub(state)
	}

	// 10. State logging (if enabled)
	if cfg.stateLog != "" {
		writeStateLog(cfg.stateLog, store, reg, state)
	}
}

func checkCompletedWorkers(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string) {
	workers := reg.List(worker.StatusActive)

	for _, w := range workers {
		if w.SessionID == "" {
			continue
		}

		// Skip leaders - handled separately
		if w.Role != worker.RoleWorker {
			continue
		}

		session := tmux.NewSession(w.SessionID, "", "")
		if !session.Exists() {
			// Session gone - worker completed or crashed
			taskID := w.CurrentTask
			worktreePath := w.Worktree
			if taskID != "" && !state.completedTasks[taskID] {
				fmt.Printf("✅ Worker %s finished task %s\n", w.DisplayName(), taskID)

				// Record metrics before cleanup (need session state for iteration count)
				startedAt := state.taskStarted[taskID]
				if startedAt.IsZero() {
					startedAt = w.LastActive // fallback
				}
				if err := worker.RecordWorkerTaskComplete(w.ID, w.Name, taskID, startedAt, w.SessionID, true); err != nil {
					fmt.Printf("   ⚠️  Error recording metrics: %v\n", err)
				} else {
					fmt.Printf("   📊 Recorded task metrics\n")
				}

				// Move task to review (not done - let reviewer check it)
				if err := store.Move(taskID, kanban.StatusReview); err != nil {
					fmt.Printf("   ⚠️  Error moving task to review: %v\n", err)
				} else {
					fmt.Printf("   📋 Moved task to Review\n")
				}

				// Clean up task worktree (if it was created for this task)
				if worktreePath != "" && worktreePath != workDir {
					if err := removeTaskWorktree(workDir, taskID); err != nil {
						fmt.Printf("   ⚠️  Error removing worktree: %v\n", err)
					} else {
						fmt.Printf("   🗑️  Cleaned up worktree\n")
					}
				}

				state.completedTasks[taskID] = true
				state.failureTracker.Record(true)        // task completed successfully
				state.taskCompleted[taskID] = time.Now() // Track completion time
				delete(state.taskWorkers, taskID)
				delete(state.workerTasks, w.ID)
				delete(state.pokeCounts, w.ID)
				delete(state.taskStarted, taskID)
			}

			// Stop the worker first (clears active state), then reset
			if err := worker.Stop(reg, w.ID); err != nil {
				fmt.Printf("   ⚠️  Error stopping worker: %v\n", err)
			}
			if err := worker.Reset(reg, w.ID); err != nil {
				fmt.Printf("   ⚠️  Error resetting worker: %v\n", err)
			}
		}
	}
}

// checkStaleReviewTasks checks for tasks stuck in review and auto-moves them to merge queue.
// This handles infrastructure tasks and other non-code tasks that don't have branches to merge.
func checkStaleReviewTasks(store *kanban.Store, state *supervisorState) {
	reviewTasks, err := store.List(kanban.StatusReview)
	if err != nil {
		return
	}

	staleThreshold := 30 * time.Minute // Tasks in review for > 30 minutes are considered stale
	for _, task := range reviewTasks {
		// Check if task has been completed and is waiting in review
		if completedAt, ok := state.taskCompleted[task.ID]; ok {
			timeSinceCompletion := time.Since(completedAt)
			if timeSinceCompletion > staleThreshold {
				// Task has been in review for too long after worker completion
				// This likely means the reviewer approved it but didn't move it
				// OR the task doesn't have code to merge (infrastructure task)
				fmt.Printf("   ⏰ Task %s stuck in review for %v\n", task.ID[:12], timeSinceCompletion.Truncate(time.Minute))

				// Auto-move to merge queue for deployment verification
				if err := store.Move(task.ID, kanban.StatusMerge); err != nil {
					fmt.Printf("      ⚠️  Could not auto-move to merge: %v\n", err)
				} else {
					fmt.Printf("      📋 Auto-moved to merge queue for deployment verification\n")
				}
			}
		}
	}
}

// analyzeAndRequeueTasks checks for tasks that are stuck or abandoned and requeues them
func analyzeAndRequeueTasks(ctx context.Context, store *kanban.Store, reg *worker.Registry, state *supervisorState, cfg supervisorConfig) {
	if !cfg.autoRequeue {
		return
	}

	// First, check for stale review tasks that need auto-progression
	checkStaleReviewTasks(store, state)

	// Get all in_progress tasks
	inProgressTasks, err := store.List(kanban.StatusInProgress)
	if err != nil {
		return
	}

	// Build maps for quick lookup
	activeWorkers := reg.List(worker.StatusActive)
	activeWorkerIDs := make(map[string]bool)
	workerTaskMap := make(map[string]string) // task ID -> worker ID from registry
	for _, w := range activeWorkers {
		activeWorkerIDs[w.ID] = true
		if w.CurrentTask != "" {
			workerTaskMap[w.CurrentTask] = w.ID
		}
	}

	for _, task := range inProgressTasks {
		// Skip if this task was recently completed (avoid race condition)
		if completedAt, wasCompleted := state.taskCompleted[task.ID]; wasCompleted {
			// Grace period: 5 minutes after completion before considering for requeue
			if time.Since(completedAt) < 5*time.Minute {
				continue
			}
			// Old completion record - remove it
			delete(state.taskCompleted, task.ID)
		}

		// Skip if marked as completed (belt and suspenders)
		if state.completedTasks[task.ID] {
			continue
		}

		// Track when we first saw this task in progress
		if _, tracked := state.taskStarted[task.ID]; !tracked {
			state.taskStarted[task.ID] = time.Now()
		}

		// Check if this task has an active worker (from in-memory state OR registry)
		assignedWorker := state.taskWorkers[task.ID]
		if assignedWorker == "" {
			// Check registry for worker claiming this task
			assignedWorker = workerTaskMap[task.ID]
			if assignedWorker != "" {
				// Restore in-memory state from registry
				state.taskWorkers[task.ID] = assignedWorker
				state.workerTasks[assignedWorker] = task.ID
			}
		}
		hasActiveWorker := assignedWorker != "" && activeWorkerIDs[assignedWorker]

		if !hasActiveWorker {
			// No worker on this task - check how long it's been stuck
			timeInProgress := time.Since(state.taskStarted[task.ID])

			if timeInProgress > cfg.stuckThreshold {
				// Task has been stuck without a worker - requeue it
				fmt.Printf("🔄 Requeuing stuck task %s (no worker for %s)\n", task.ID[:8], timeInProgress.Round(time.Second))
				fmt.Printf("   Title: %s\n", task.Title)

				if err := store.Move(task.ID, kanban.StatusTodo); err != nil {
					fmt.Printf("   ⚠️  Error requeuing: %v\n", err)
				} else {
					fmt.Printf("   ✅ Moved back to Todo\n")
					state.failureTracker.Record(false) // stuck task counts as failure
					delete(state.taskStarted, task.ID)
					delete(state.taskWorkers, task.ID)

					// Dispatch task_failed webhook
					if cfg.webhookDispatcher != nil {
						reason := fmt.Sprintf("stuck without worker for %s", timeInProgress.Round(time.Second))
						event := webhooks.NewTaskFailedEvent(task.ID, task.Title, reason, "", webhooks.Priority(task.Priority))
						cfg.webhookDispatcher.Dispatch(ctx, event)
					}
				}
			}
		}
	}

	// Also check review tasks - if they've been in review too long without reviewer
	reviewTasks, err := store.List(kanban.StatusReview)
	if err != nil {
		return
	}

	reviewThreshold := cfg.stuckThreshold * 2 // Review can take longer

	for _, task := range reviewTasks {
		if _, tracked := state.taskStarted[task.ID]; !tracked {
			state.taskStarted[task.ID] = time.Now()
		}

		// If no reviewer is active and task has been waiting too long
		if !state.reviewer.running {
			timeInReview := time.Since(state.taskStarted[task.ID])
			if timeInReview > reviewThreshold {
				fmt.Printf("⏰ Task %s waiting for review: %s\n", task.ID[:8], timeInReview.Round(time.Second))
			}
		}
	}
}

// checkForNewTasks analyzes completed work and determines if new tasks are needed
func checkForNewTasks(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, cfg supervisorConfig) {
	// Only run analysis periodically (not every cycle)
	if time.Since(state.lastAnalysis) < cfg.analyzeInterval {
		return
	}

	// Don't analyze if we're busy with other work
	if state.analyzer.running || state.reviewer.running || state.planner.running {
		return
	}

	// Check if any worker is active
	for _, w := range reg.List(worker.StatusActive) {
		if w.Role == worker.RoleWorker {
			return // Don't analyze while workers are active
		}
	}

	// Get board state
	board, err := store.GetBoard()
	if err != nil {
		return
	}

	// Count tasks
	var doneCount, backlogCount, todoCount int
	for _, col := range board.Columns {
		switch col.Status {
		case kanban.StatusDone:
			doneCount = len(col.Issues)
		case kanban.StatusBacklog:
			backlogCount = len(col.Issues)
		case kanban.StatusTodo:
			todoCount = len(col.Issues)
		}
	}

	// Only analyze if we have completed tasks and low pipeline
	if doneCount == 0 || (backlogCount+todoCount) > 5 {
		return // Enough work in the pipeline
	}

	// Find an analyzer worker (or use planner role)
	analyzer := findIdleLeader(reg, worker.RolePlanner)
	if analyzer == nil {
		return
	}

	fmt.Printf("🔬 Starting task analysis (checking for follow-up work)\n")

	prompt := buildTaskAnalyzerPrompt(board, workDir)
	state.lastAnalysis = time.Now()
	startLeader(reg, analyzer, &state.analyzer, workDir, prompt, leader.PromiseAnalyzer, cfg.webhookDispatcher)
}

func checkLeaderSessions(reg *worker.Registry, state *supervisorState) {
	// Track which leaders finished this cycle for cooldown reset
	leadersFinished := make(map[string]bool)

	// Check each leader type
	checkLeader := func(role worker.Role, ls *leaderState, name string, roleKey string) {
		if !ls.running {
			return
		}

		session := tmux.NewSession(ls.sessionID, "", "")
		if !session.Exists() {
			fmt.Printf("🏁 %s finished\n", name)
			leadersFinished[roleKey] = true

			// Set completion flags for merge and deploy
			switch role {
			case worker.RoleMerge:
				state.mergeCompleted = true
				fmt.Printf("   ✅ Merge workflow completed\n")
			case worker.RoleDeploy:
				state.deployCompleted = true
				fmt.Printf("   ✅ Deploy workflow completed\n")
			}

			// Save session ID before clearing state
			finishedSessionID := ls.sessionID
			ls.running = false
			ls.sessionID = ""

			// Find and reset the leader worker
			workers := reg.List()
			for _, w := range workers {
				if w.Role == role && w.SessionID == finishedSessionID {
					_ = worker.Stop(reg, w.ID)
					_ = worker.Reset(reg, w.ID)
					fmt.Printf("   ♻️  Reset %s to idle\n", w.DisplayName())
					break
				}
			}
		}
	}

	checkLeader(worker.RolePlanner, &state.planner, "Planner", "planner")
	checkLeader(worker.RoleReviewer, &state.reviewer, "Reviewer", "reviewer")
	checkLeader(worker.RoleMerge, &state.merge, "Merge leader", "merge")
	checkLeader(worker.RoleDeploy, &state.deploy, "Deploy leader", "deploy")
	checkLeader(worker.RoleGroomer, &state.groomer, "Groomer", "groomer")
	checkLeader(worker.RoleMonitor, &state.monitor, "Monitor", "monitor")
	checkLeader(worker.RoleTester, &state.tester, "Tester", "tester")
	checkLeader(worker.RolePM, &state.pm, "Project Manager", "pm")
	checkLeader(worker.RoleCICD, &state.cicd, "CI/CD Leader", "cicd")

	// Analyzer uses planner role but different state
	if state.analyzer.running {
		session := tmux.NewSession(state.analyzer.sessionID, "", "")
		if !session.Exists() {
			fmt.Printf("🏁 Task analyzer finished\n")
			state.analyzer.running = false
			state.analyzer.sessionID = ""
		}
	}

	// Store finished leaders for cooldown reset in next cycle
	state.leadersJustFinished = leadersFinished
}

func pokeActiveWorkers(reg *worker.Registry, state *supervisorState, maxPokes int) {
	workers := reg.List(worker.StatusActive)

	for _, w := range workers {
		if w.SessionID == "" {
			continue
		}

		session := tmux.NewSession(w.SessionID, "", "")
		if !session.Exists() {
			continue
		}

		// Check if poke is actually needed (smart assessment)
		shouldPoke, reason := shouldPokeWorker(session, w, state)
		if !shouldPoke {
			if reason != "" {
				fmt.Printf("🔇 %s %s: skipped poke (%s)\n", w.RoleIcon(), w.DisplayName(), reason)
			}
			continue
		}

		// Send nudge
		state.pokeCounts[w.ID]++
		count := state.pokeCounts[w.ID]
		nudge := buildNudgeMessage(w, count)

		if err := session.SendKeys(nudge); err != nil {
			fmt.Printf("⚠️  %s: failed to poke: %v\n", w.DisplayName(), err)
			continue
		}

		fmt.Printf("📣 %s %s: poked (#%d) - %s\n", w.RoleIcon(), w.DisplayName(), count, reason)
	}
}

// shouldPokeWorker determines if a worker needs a poke by analyzing their session.
// Returns (shouldPoke, reason).
func shouldPokeWorker(session *tmux.Session, w *worker.Worker, state *supervisorState) (bool, string) {
	// Capture recent pane output
	output := captureRecentOutput(session, 50)

	// Track output for change detection
	lastOutput := state.lastPaneOutput[w.ID]
	state.lastPaneOutput[w.ID] = output

	// Quick heuristic checks
	if isActivelyWorking(output) {
		return false, "actively working"
	}

	// Check if output is changing (worker making progress)
	if lastOutput != "" && output != lastOutput {
		// Output changed since last check - worker is progressing
		state.unchangedOutputCount[w.ID] = 0
		return false, "making progress"
	}

	// Output unchanged - might be stuck
	state.unchangedOutputCount[w.ID]++
	unchangedCount := state.unchangedOutputCount[w.ID]

	// If output unchanged for just 1-2 cycles, give more time
	if unchangedCount < 3 {
		return false, "waiting for progress"
	}

	// Output unchanged for 3+ cycles - use Haiku to assess if stuck
	if unchangedCount == 3 {
		// First time hitting threshold - do Haiku assessment
		needsPoke, assessment := assessWithHaiku(output, w)
		if !needsPoke {
			return false, assessment
		}
		return true, assessment
	}

	// Already assessed, poke periodically (every 3 unchanged cycles)
	if unchangedCount%3 == 0 {
		return true, "appears idle"
	}

	return false, "recently poked"
}

// captureRecentOutput captures recent lines from the tmux pane.
func captureRecentOutput(session *tmux.Session, lines int) string {
	out, err := tmux.CapturePaneFrom(session.Name, lines)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// isActivelyWorking checks if the output indicates active Claude work.
func isActivelyWorking(output string) bool {
	// Signs of active work
	activeIndicators := []string{
		"Reading", "Writing", "Editing", // File operations
		"Running", "Executing", // Command execution
		"Searching", "Analyzing", // Analysis
		"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", // Spinner characters
		"thinking", "Thinking", // Thinking indicators
		"Tool:", "Using tool", // Tool use
		"```", // Code blocks being output
	}

	for _, indicator := range activeIndicators {
		if strings.Contains(output, indicator) {
			return true
		}
	}

	return false
}

// runClaudeHaiku runs a prompt through Claude Haiku in print mode.
// Uses a temp file to safely pass the prompt without shell escaping issues.
func runClaudeHaiku(prompt string) ([]byte, error) {
	// Write prompt to temp file for reliable delivery
	promptFile, err := os.CreateTemp("", "forge-haiku-*.txt")
	if err != nil {
		return nil, fmt.Errorf("creating prompt file: %w", err)
	}
	promptPath := promptFile.Name()
	defer func() { _ = os.Remove(promptPath) }()

	if _, err := promptFile.WriteString(prompt); err != nil {
		_ = promptFile.Close()
		return nil, fmt.Errorf("writing prompt file: %w", err)
	}
	_ = promptFile.Close()

	// Run claude with prompt from file via shell
	// Using shell to pipe file content to claude -p
	cmd := exec.Command("sh", "-c", fmt.Sprintf("cat %s | claude -p --model haiku", promptPath))
	return cmd.Output()
}

// assessWithHaiku uses Claude Haiku to intelligently assess if worker needs a poke.
func assessWithHaiku(output string, w *worker.Worker) (bool, string) {
	// Truncate output if too long
	if len(output) > 2000 {
		output = output[len(output)-2000:]
	}

	// Skip if output is essentially empty
	if len(strings.TrimSpace(output)) < 50 {
		return true, "minimal output"
	}

	prompt := fmt.Sprintf(`You are assessing a Claude Code worker session to determine if it needs a nudge.

Worker: %s (role: %s)
Task: %s

Recent terminal output (last 50 lines):
---
%s
---

Assess whether this worker:
1. Is ACTIVELY WORKING (processing, thinking, running tools) - respond: NO_POKE: <brief reason>
2. Is WAITING for something or PAUSED - respond: NO_POKE: <brief reason>
3. Appears STUCK or IDLE and needs encouragement - respond: POKE: <brief reason>
4. Has COMPLETED its work but hasn't output the promise - respond: POKE: needs to output completion promise

Respond with ONLY one line: either "NO_POKE: reason" or "POKE: reason"`,
		w.DisplayName(), w.Role, w.CurrentTask, output)

	// Run haiku assessment using shell with proper escaping
	out, err := runClaudeHaiku(prompt)
	if err != nil {
		// On error, default to poking
		return true, "assessment failed"
	}

	response := strings.TrimSpace(string(out))

	if strings.HasPrefix(response, "NO_POKE:") {
		reason := strings.TrimPrefix(response, "NO_POKE:")
		return false, strings.TrimSpace(reason)
	}

	if strings.HasPrefix(response, "POKE:") {
		reason := strings.TrimPrefix(response, "POKE:")
		return true, strings.TrimSpace(reason)
	}

	// Unclear response, default to not poking
	return false, "unclear assessment"
}

func assignTasks(ctx context.Context, store *kanban.Store, reg *worker.Registry, state *supervisorState, cfg supervisorConfig) {
	// Count active development workers
	activeWorkers := reg.List(worker.StatusActive)
	activeCount := 0
	var activeNames []string
	for _, w := range activeWorkers {
		if w.Role == worker.RoleWorker {
			activeCount++
			activeNames = append(activeNames, w.DisplayName())
		}
	}

	// Respect concurrent limit
	if activeCount >= cfg.maxConcurrentWorkers {
		fmt.Printf("⏸️  At max workers (%d/%d active: %v)\n", activeCount, cfg.maxConcurrentWorkers, activeNames)
		return
	}

	// Get todo tasks
	todoTasks, err := store.List(kanban.StatusTodo)
	if err != nil {
		fmt.Printf("⚠️  Error listing todo tasks: %v\n", err)
		return
	}

	if len(todoTasks) == 0 {
		return
	}

	// Get idle workers (only role=worker, not leaders)
	allIdle := reg.List(worker.StatusIdle)
	var idleWorkers []*worker.Worker
	for _, w := range allIdle {
		if w.Role == worker.RoleWorker {
			idleWorkers = append(idleWorkers, w)
		}
	}

	if len(idleWorkers) == 0 {
		fmt.Printf("💤 %d tasks waiting, no idle workers (role=worker)\n", len(todoTasks))
		return
	}

	// Calculate available slots
	slotsAvailable := cfg.maxConcurrentWorkers - activeCount
	assigned := 0
	taskIdx := 0

	// Assign tasks to idle workers up to available slots
	for _, w := range idleWorkers {
		if assigned >= slotsAvailable {
			break
		}

		// Find next unassigned task
		var task *kanban.Issue
		for taskIdx < len(todoTasks) {
			candidate := todoTasks[taskIdx]
			taskIdx++
			if state.taskWorkers[candidate.ID] == "" {
				task = candidate
				break
			}
		}

		// No more tasks available
		if task == nil {
			break
		}

		// Start the worker on this task
		fmt.Printf("🚀 Assigning %s to worker %s\n", task.ID[:8], w.DisplayName())
		fmt.Printf("   Task: %s\n", task.Title)

		// Create worktree for this task (enables parallel execution)
		worktreePath, err := createTaskWorktree(cfg.workDir, task.ID)
		if err != nil {
			fmt.Printf("   ⚠️  Error creating worktree: %v\n", err)
			fmt.Printf("   ⏭️  Skipping task (worktree required for parallel execution)\n")
			// Move task back to todo so it can be retried
			_ = store.Move(task.ID, kanban.StatusTodo)
			continue
		}

		// Don't use main worktree for workers (would block other agents)
		if worktreePath == cfg.workDir {
			fmt.Printf("   ⏭️  Skipping task (would use shared worktree)\n")
			_ = store.Move(task.ID, kanban.StatusTodo)
			continue
		}

		// Move task to in_progress
		if err := store.Move(task.ID, kanban.StatusInProgress); err != nil {
			fmt.Printf("   ⚠️  Error moving task: %v\n", err)
			continue
		}

		// Build prompt from task
		prompt := buildTaskPrompt(task)
		// Use unique suffix for promise name (e.g., "0vk" from "blocks-forge-0vk")
		promiseID := task.ID
		if idx := strings.LastIndex(task.ID, "-"); idx > 0 && idx < len(task.ID)-1 {
			promiseID = task.ID[idx+1:]
		} else if len(promiseID) > 8 {
			promiseID = promiseID[:8]
		}
		promise := fmt.Sprintf("TASK_%s_DONE", promiseID)

		// Track assignment before starting
		state.taskWorkers[task.ID] = w.ID
		state.workerTasks[w.ID] = task.ID

		// Start worker (in goroutine to not block)
		go func(w *worker.Worker, task *kanban.Issue, prompt, promise, worktree string, dispatcher *webhooks.Dispatcher) {
			opts := worker.StartOptions{
				TaskID:   task.ID,
				Worktree: worktree,
				Prompt:   prompt,
				Promise:  promise,
			}

			workerCtx := context.Background()
			if err := worker.Start(workerCtx, reg, w.ID, opts); err != nil {
				fmt.Printf("⚠️  Error starting %s: %v\n", w.DisplayName(), err)
				// Dispatch worker_error webhook
				if dispatcher != nil {
					event := webhooks.NewWorkerErrorEvent(w.ID, w.DisplayName(), task.ID, err.Error())
					dispatcher.Dispatch(workerCtx, event)
				}
			} else {
				// Dispatch worker_started webhook
				if dispatcher != nil {
					event := webhooks.NewWorkerStartedEvent(w.ID, w.DisplayName(), task.ID, task.Title, worktree)
					dispatcher.Dispatch(workerCtx, event)
				}
			}
		}(w, task, prompt, promise, worktreePath, cfg.webhookDispatcher)

		assigned++
	}

	if assigned > 0 {
		fmt.Printf("📋 Assigned %d task(s)\n", assigned)
	}
}

func runLeaderWorkflow(store *kanban.Store, reg *worker.Registry, state *supervisorState, cfg supervisorConfig) {
	fmt.Printf("   🔄 Running leader workflow...\n")
	board, err := store.GetBoard()
	if err != nil {
		return
	}

	workDir := cfg.workDir
	counts := make(map[kanban.Status]int)
	for _, col := range board.Columns {
		counts[col.Status] = len(col.Issues)
	}
	fmt.Printf("   📊 Board counts: backlog=%d, todo=%d, wip=%d, review=%d, merge=%d, done=%d\n",
		counts[kanban.StatusBacklog], counts[kanban.StatusTodo], counts[kanban.StatusInProgress],
		counts[kanban.StatusReview], counts[kanban.StatusMerge], counts[kanban.StatusDone])

	// Check if any worker is using the worktree
	workerActive := false
	for _, w := range reg.List(worker.StatusActive) {
		if w.Role == worker.RoleWorker {
			workerActive = true
			break
		}
	}

	// Calculate workflow state for deploy
	mergeCount := counts[kanban.StatusMerge]
	hasMergeTasks := mergeCount > 0
	doneCount := counts[kanban.StatusDone]
	pendingWork := counts[kanban.StatusBacklog] + counts[kanban.StatusTodo] +
		counts[kanban.StatusInProgress] + counts[kanban.StatusReview] + counts[kanban.StatusMerge]
	state.allTasksDone = pendingWork == 0 && doneCount > 0

	// Reset deployCompleted if there are tasks in merge queue
	if hasMergeTasks && state.deployCompleted {
		fmt.Printf("🔄 Merge queue has work, resetting deploy state\n")
		state.deployCompleted = false
	}

	// Log merge queue status
	if mergeCount > 0 {
		fmt.Printf("   📊 Merge queue: %d tasks (running=%v, completed=%v)\n", mergeCount, state.deploy.running, state.deployCompleted)
	}

	// Use smart leader scheduling with triggers and cooldowns
	// 1. GROOMER: Run when there are items in backlog
	if shouldStartLeader("groomer", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "groomer")
		startGroomer(store, reg, state, workDir, board, cfg)
	}

	// 2. REVIEWER: Run when there are tasks in review
	if shouldStartLeader("reviewer", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "reviewer")
		startReviewer(store, reg, state, workDir, board, cfg)
	}

	// 3. PLANNER: Run when no work in progress and we need to plan
	if shouldStartLeader("planner", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "planner")
		startPlanner(store, reg, state, workDir, board, cfg)
		return // Planner blocks other leaders
	}

	// 4. DEPLOY: Run when there are tasks in merge queue
	if shouldStartLeader("deploy", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "deploy")
		startDeploy(store, reg, state, workDir, board, cfg)
	}

	// 5. MONITOR: Run continuously to observe infrastructure health
	if shouldStartLeader("monitor", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "monitor")
		startMonitor(reg, state, workDir, cfg)
	}

	// 6. TESTER: Run when there's active work to test
	if shouldStartLeader("tester", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "tester")
		startTester(reg, state, workDir, cfg)
	}

	// 7. PM: Run when there are done tasks to analyze
	if shouldStartLeader("pm", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "pm")
		startPM(store, reg, state, workDir, board, cfg)
	}

	// 8. CICD: Run periodically to check CI health
	if shouldStartLeader("cicd", counts, state, cfg, workerActive) {
		recordLeaderStart(state, "cicd")
		startCICD(reg, state, workDir, cfg)
	}

	// Update lastMergeCount after all checks
	state.lastMergeCount = mergeCount
}

func findIdleLeader(reg *worker.Registry, role worker.Role) *worker.Worker {
	workers := reg.List(worker.StatusIdle)
	for _, w := range workers {
		if w.Role == role {
			return w
		}
	}
	return nil
}

func startLeader(reg *worker.Registry, w *worker.Worker, ls *leaderState, workDir, prompt, promise string, dispatcher *webhooks.Dispatcher) {
	ls.running = true
	ls.sessionID = w.TmuxSessionName()
	ls.startedAt = time.Now()

	// Determine worktree for this leader
	leaderWorkDir := workDir

	// First, find the git repo (workspace may have repo in .forge/repos/)
	gitRepo, err := findGitRepo(workDir)
	if err == nil {
		leaderWorkDir = gitRepo
	}

	// Groomer, reviewer, planner, monitor, tester, pm use dedicated worktrees; merge, deploy work in main repo
	switch w.Role {
	case worker.RoleGroomer, worker.RoleReviewer, worker.RolePlanner, worker.RoleMonitor, worker.RoleTester, worker.RolePM:
		if wt, err := createLeaderWorktree(workDir, string(w.Role)); err == nil {
			leaderWorkDir = wt
			fmt.Printf("   📁 Using worktree: %s\n", wt)
		}
	case worker.RoleMerge, worker.RoleDeploy:
		// Merge and deploy work in main git repo (need to push to main)
		fmt.Printf("   📂 Working in git repo: %s\n", leaderWorkDir)
	}

	// Dispatch leader_launched webhook
	if dispatcher != nil {
		event := webhooks.NewLeaderLaunchedEvent(string(w.Role), w.ID, w.DisplayName(), ls.sessionID)
		dispatcher.Dispatch(context.Background(), event)
	}

	go func() {
		opts := worker.StartOptions{
			TaskID:   string(w.Role) + "-session",
			Worktree: leaderWorkDir,
			Prompt:   prompt,
			Promise:  promise,
		}

		// Continuous leaders get high iteration limits
		// They run in a ralph loop until work is exhausted
		switch w.Role {
		case worker.RoleGroomer, worker.RoleReviewer, worker.RoleDeploy, worker.RoleMonitor, worker.RoleTester, worker.RolePM:
			opts.MaxIterations = 500 // High limit for continuous operation
		case worker.RolePlanner, worker.RoleMerge:
			opts.MaxIterations = 200 // Moderate for planning/merge
		default:
			opts.MaxIterations = 100 // Default
		}

		ctx := context.Background()
		if err := worker.Start(ctx, reg, w.ID, opts); err != nil {
			fmt.Printf("⚠️  Error starting %s: %v\n", w.Role, err)
			ls.running = false
		}
	}()
}

func startReviewer(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RoleReviewer)
	if w == nil {
		fmt.Printf("⚠️  Review queue has items but no idle reviewer worker\n")
		fmt.Printf("   Create one with: foundry worker create --role reviewer\n")
		return
	}

	fmt.Printf("🔍 Starting reviewer %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/reviewer.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("reviewer")
	if err != nil {
		fmt.Printf("⚠️  Error loading reviewer prompt: %v\n", err)
	}

	// If custom prompt found, append dynamic board state
	if prompt != "" {
		prompt += "\n\n" + buildReviewQueueSection(board, workDir)
	} else {
		// Fall back to built-in prompt
		prompt = buildReviewerPrompt(board, workDir)
	}

	startLeader(reg, w, &state.reviewer, workDir, prompt, leader.PromiseReviewer, cfg.webhookDispatcher)
}

func startPlanner(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RolePlanner)
	if w == nil {
		fmt.Printf("⚠️  Backlog needs planning but no idle planner worker\n")
		fmt.Printf("   Create one with: foundry worker create --role planner\n")
		return
	}

	fmt.Printf("📋 Starting planner %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/planner.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("planner")
	if err != nil {
		fmt.Printf("⚠️  Error loading planner prompt: %v\n", err)
	}

	// If custom prompt found, append dynamic board state
	if prompt != "" {
		prompt += "\n\n" + buildBoardStateSection(board, workDir, leader.PromisePlanner)
	} else {
		// Fall back to built-in prompt
		prompt = buildPlannerPrompt(board, workDir)
	}

	startLeader(reg, w, &state.planner, workDir, prompt, leader.PromisePlanner, cfg.webhookDispatcher)
}

func startMerge(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RoleMerge)
	if w == nil {
		fmt.Printf("⚠️  All tasks done but no idle merge worker\n")
		fmt.Printf("   Create one with: foundry worker create --role merge\n")
		return
	}

	fmt.Printf("🔀 Starting merge leader %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/merge.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("merge")
	if err != nil {
		fmt.Printf("⚠️  Error loading merge prompt: %v\n", err)
	}

	// If custom prompt found, append dynamic board state
	if prompt != "" {
		prompt += "\n\n" + buildMergeQueueSection(board, workDir)
	} else {
		// Fall back to built-in prompt
		prompt = buildMergePrompt(board, workDir)
	}

	startLeader(reg, w, &state.merge, workDir, prompt, leader.PromiseMerge, cfg.webhookDispatcher)
}

func startDeploy(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RoleDeploy)
	if w == nil {
		fmt.Printf("⚠️  Merge complete but no idle deploy worker\n")
		fmt.Printf("   Create one with: foundry worker create --role deploy\n")
		return
	}

	fmt.Printf("🚀 Starting deploy leader %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/deploy.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("deploy")
	if err != nil {
		fmt.Printf("⚠️  Error loading deploy prompt: %v\n", err)
	}

	// If custom prompt found, append dynamic board state
	if prompt != "" {
		prompt += "\n\n" + buildDoneTasksSection(board, workDir)
	} else {
		// Fall back to built-in prompt
		prompt = buildDeployPrompt(board, workDir)
	}

	// Dispatch deployment_triggered webhook with task info
	if cfg.webhookDispatcher != nil {
		var taskIDs []string
		var taskCount int
		for _, col := range board.Columns {
			if col.Status == kanban.StatusMerge {
				taskCount = len(col.Issues)
				for _, issue := range col.Issues {
					taskIDs = append(taskIDs, issue.ID)
				}
				break
			}
		}
		event := webhooks.NewDeploymentTriggeredEvent(w.ID, w.DisplayName(), w.TmuxSessionName(), taskCount, taskIDs)
		cfg.webhookDispatcher.Dispatch(context.Background(), event)
	}

	startLeader(reg, w, &state.deploy, workDir, prompt, leader.PromiseDeploy, cfg.webhookDispatcher)
}

func startGroomer(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RoleGroomer)
	if w == nil {
		fmt.Printf("⚠️  Backlog has items but no idle groomer worker\n")
		fmt.Printf("   Create one with: foundry worker create --role groomer\n")
		return
	}

	fmt.Printf("🧹 Starting backlog groomer %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/groomer.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("groomer")
	if err != nil {
		fmt.Printf("⚠️  Error loading groomer prompt: %v\n", err)
	}

	// If custom prompt found, append dynamic board state
	if prompt != "" {
		prompt += "\n\n" + buildBacklogSection(board, workDir)
	} else {
		// Fall back to built-in prompt
		prompt = buildGroomerPrompt(board, workDir)
	}

	startLeader(reg, w, &state.groomer, workDir, prompt, leader.PromiseGroomer, cfg.webhookDispatcher)
}

func startMonitor(reg *worker.Registry, state *supervisorState, workDir string, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RoleMonitor)
	if w == nil {
		// Monitor is optional - don't warn if not configured
		return
	}

	fmt.Printf("📡 Starting infrastructure monitor %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/monitor.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("monitor")
	if err != nil {
		fmt.Printf("⚠️  Error loading monitor prompt: %v\n", err)
		return
	}

	// If no custom prompt, use default
	if prompt == "" {
		prompt = buildMonitorPrompt(workDir)
	}

	startLeader(reg, w, &state.monitor, workDir, prompt, leader.PromiseMonitor, cfg.webhookDispatcher)
}

func startTester(reg *worker.Registry, state *supervisorState, workDir string, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RoleTester)
	if w == nil {
		// Tester is optional - don't warn if not configured
		return
	}

	fmt.Printf("🧪 Starting UI/API tester %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/tester.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("tester")
	if err != nil {
		fmt.Printf("⚠️  Error loading tester prompt: %v\n", err)
		return
	}

	// If no custom prompt, use default
	if prompt == "" {
		prompt = buildTesterPrompt(workDir)
	}

	startLeader(reg, w, &state.tester, workDir, prompt, leader.PromiseTester, cfg.webhookDispatcher)
}

func startPM(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RolePM)
	if w == nil {
		// PM is optional - don't warn if not configured
		return
	}

	fmt.Printf("📊 Starting Project Manager %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/pm.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("pm")
	if err != nil {
		fmt.Printf("⚠️  Error loading PM prompt: %v\n", err)
		return
	}

	// If custom prompt found, append dynamic board state
	if prompt != "" {
		prompt += "\n\n" + buildDoneTasksSection(board, workDir)
	} else {
		// Fall back to built-in prompt
		prompt = buildPMPrompt(board, workDir)
	}

	startLeader(reg, w, &state.pm, workDir, prompt, leader.PromisePM, cfg.webhookDispatcher)
}

func startCICD(reg *worker.Registry, state *supervisorState, workDir string, cfg supervisorConfig) {
	w := findIdleLeader(reg, worker.RoleCICD)
	if w == nil {
		// CICD is optional - don't warn if not configured
		return
	}

	fmt.Printf("🔧 Starting CI/CD Leader %s\n", w.DisplayName())

	// Try to load custom prompt from .forge/prompts/cicd.md
	loader := leader.NewPromptLoader(workDir)
	prompt, err := loader.Load("cicd")
	if err != nil {
		fmt.Printf("⚠️  Error loading CICD prompt: %v\n", err)
		return
	}

	// If custom prompt found, use it; otherwise use built-in
	if prompt == "" {
		prompt = buildCICDPrompt(workDir)
	}

	startLeader(reg, w, &state.cicd, workDir, prompt, leader.PromiseCICD, cfg.webhookDispatcher)
}

// Prompt builders for each leader role

// extractTaskSuffix extracts the suffix from a task ID (e.g., "blocks-forge-abc" -> "abc")
func extractTaskSuffix(taskID string) string {
	parts := strings.Split(taskID, "-")
	if len(parts) >= 3 {
		return parts[len(parts)-1]
	}
	return taskID
}

func buildReviewerPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a CODE REVIEWER for the Forge supervisor. Your job is to review completed work.\n\n")

	sb.WriteString("## Tasks in Review\n\n")
	reviewCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusReview {
			for _, issue := range col.Issues {
				// Include full task ID and branch suffix so reviewer can run foundry task move <id>
				suffix := extractTaskSuffix(issue.ID)
				sb.WriteString(fmt.Sprintf("- [%s] **%s** (branch: task/%s): %s\n", issue.Priority, issue.ID, suffix, issue.Title))
				reviewCount++
			}
		}
	}
	if reviewCount == 0 {
		sb.WriteString("(no tasks in review)\n")
	}

	sb.WriteString("\n## Review Checklist\n\n")
	sb.WriteString("For each task, verify:\n")
	sb.WriteString("- [ ] **CI/CD PASSES**: Run `gh run list --branch task/<suffix>` to check CI status\n")
	sb.WriteString("- [ ] Code compiles/runs without errors (`go build ./...`)\n")
	sb.WriteString("- [ ] Tests pass (`go test ./...`)\n")
	sb.WriteString("- [ ] Code follows project conventions\n")
	sb.WriteString("- [ ] No obvious bugs or security issues\n")
	sb.WriteString("- [ ] Changes match the task description\n\n")
	sb.WriteString("**IMPORTANT**: Do NOT approve tasks with failing CI. If CI fails, REJECT the task.\n\n")

	sb.WriteString("## Viewing Task Details\n\n")
	sb.WriteString("Use these commands to inspect tasks:\n")
	sb.WriteString("- `foundry task show <id>` - Full task details (description, labels, branch)\n")
	sb.WriteString("- `foundry task diff <id>` - See code changes for a task\n")
	sb.WriteString("- `foundry task log <id>` - Git commit history for a task\n")
	sb.WriteString("- `foundry task branches` - List all task branches with status\n")
	sb.WriteString("- `foundry task list --status review` - List tasks in review queue\n\n")

	sb.WriteString("## Parallel Workflow\n\n")
	sb.WriteString("Workers run in parallel, each in their own worktree with a task branch.\n")
	sb.WriteString("- Task branches follow pattern: `task/<task-id-suffix>`\n")
	sb.WriteString("- Review a branch: `foundry task diff <id>` or `foundry task log <id>`\n")
	sb.WriteString("- Workers may still be active - review completed work as it arrives\n\n")

	sb.WriteString("## MANDATORY: Execute Move Commands\n\n")
	sb.WriteString("**CRITICAL**: For EVERY task in review, you MUST run `foundry task move` after reviewing.\n")
	sb.WriteString("DO NOT just write 'APPROVED' or 'REJECTED' in your output - you must EXECUTE the command.\n\n")

	sb.WriteString("## Your Workflow\n\n")
	sb.WriteString("For EACH task in the review queue:\n\n")
	sb.WriteString("1. **View task details**: `foundry task show <full-task-id>`\n")
	sb.WriteString("2. **Check branch commits**: `git log main..task/<suffix> --oneline`\n")
	sb.WriteString("3. **Review changes**: `git diff main..task/<suffix>`\n")
	sb.WriteString("4. **Make decision and EXECUTE move command**:\n")
	sb.WriteString("   - If APPROVED: `foundry task move <full-task-id> m`\n")
	sb.WriteString("   - If REJECTED: `foundry task move <full-task-id> t` (add feedback to task description first)\n\n")

	sb.WriteString("**Example workflow for task blocks-forge-abc:**\n")
	sb.WriteString("```bash\n")
	sb.WriteString("foundry task show blocks-forge-abc\n")
	sb.WriteString("git log main..task/abc --oneline\n")
	sb.WriteString("git diff main..task/abc\n")
	sb.WriteString("# After review, EXECUTE:\n")
	sb.WriteString("foundry task move blocks-forge-abc m   # Move to merge if approved\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Continuous Operation (Ralph Loop)\n\n")
	sb.WriteString("You run CONTINUOUSLY until all review items are processed:\n")
	sb.WriteString("1. For each task above, review AND move it\n")
	sb.WriteString("2. After processing all, check for more: `foundry task list --status review`\n")
	sb.WriteString("3. If more items exist, continue reviewing AND moving them\n")
	sb.WriteString("4. ONLY output the completion promise when review queue is EMPTY\n\n")
	sb.WriteString("**VERIFICATION**: Before completing, run `foundry task list --status review` to confirm empty.\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen review queue is EMPTY, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseReviewer))

	return sb.String()
}

// buildReviewQueueSection builds just the dynamic review queue section for custom prompts.
func buildReviewQueueSection(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("## Current Tasks in Review\n\n")
	reviewCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusReview {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID, issue.Title))
				if issue.Description != "" {
					sb.WriteString(fmt.Sprintf("  Description: %s\n", issue.Description))
				}
				reviewCount++
			}
		}
	}
	if reviewCount == 0 {
		sb.WriteString("(no tasks in review - you may output the completion promise)\n")
	}

	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n", workDir))
	sb.WriteString(fmt.Sprintf("\nWhen review queue is EMPTY, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseReviewer))

	return sb.String()
}

func buildPlannerPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a PROJECT PLANNER for the Forge supervisor. Your job is to plan and prioritize work.\n\n")

	sb.WriteString("## Current Board State\n\n")
	for _, col := range board.Columns {
		sb.WriteString(fmt.Sprintf("### %s (%d)\n", col.Status, len(col.Issues)))
		for _, issue := range col.Issues {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
			if issue.Description != "" {
				sb.WriteString(fmt.Sprintf("  Description: %s\n", issue.Description))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Prioritization Guidelines\n\n")
	sb.WriteString("- **critical**: Blocks everything, must do immediately\n")
	sb.WriteString("- **high**: Important for current sprint\n")
	sb.WriteString("- **medium**: Should do soon\n")
	sb.WriteString("- **low**: Nice to have, backlog\n\n")

	sb.WriteString("Consider:\n")
	sb.WriteString("- Dependencies (what unblocks the most?)\n")
	sb.WriteString("- Risk (tackle unknowns early)\n")
	sb.WriteString("- Value (user impact)\n\n")

	sb.WriteString("## Task Management Commands\n\n")
	sb.WriteString("Use these commands to view and manage tasks:\n")
	sb.WriteString("- `foundry task list` - View all tasks by status\n")
	sb.WriteString("- `foundry task show <id>` - Full task details\n")
	sb.WriteString("- `foundry task move <id> <status>` - Move tasks between columns\n")
	sb.WriteString("- `foundry task add \"title\" -p high -d \"desc\"` - Create new tasks\n")
	sb.WriteString("- `foundry task edit <id> -p critical` - Update priority\n\n")

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Load planning context: `search_nodes({ query: \"project roadmap goals\" })`\n")
	sb.WriteString("2. Review backlog tasks: `foundry task list --status backlog`\n")
	sb.WriteString("3. Inspect tasks: `foundry task show <id>` for full details\n")
	sb.WriteString("4. Move prioritized items to todo: `foundry task move <id> todo`\n")
	sb.WriteString("5. Add missing tasks: `foundry task add \"title\" -p high -s todo -d \"description\"`\n")
	sb.WriteString("6. Update priorities if needed: `foundry task edit <id> -p critical`\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished planning, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromisePlanner))

	return sb.String()
}

func buildMergePrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a MERGE COORDINATOR for the Forge supervisor. Tasks have passed review and are ready for merge.\n\n")

	sb.WriteString("## Tasks Ready to Merge\n\n")
	mergeCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusMerge {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- %s: %s (branch: task/%s)\n", issue.ID[:8], issue.Title, issue.ID[:8]))
				mergeCount++
			}
		}
	}
	if mergeCount == 0 {
		sb.WriteString("(no tasks in merge queue)\n")
	}

	sb.WriteString("\n## Viewing Task Details\n\n")
	sb.WriteString("Use these commands to inspect tasks and branches:\n")
	sb.WriteString("- `foundry task show <id>` - Full task details with branch info\n")
	sb.WriteString("- `foundry task diff <id>` - See code changes for a task\n")
	sb.WriteString("- `foundry task log <id>` - Git commit history for a task\n")
	sb.WriteString("- `foundry task branches` - List all task branches with status\n\n")

	sb.WriteString("## Pre-Merge Checklist\n\n")
	sb.WriteString("- [ ] All tests pass: `go test ./...`\n")
	sb.WriteString("- [ ] Code builds: `go build ./...`\n")
	sb.WriteString("- [ ] No merge conflicts between branches\n\n")

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Check handoffs: `search_memory_facts({ query: \"forge-handoff TO: merge\" })`\n")
	sb.WriteString("2. List task branches to merge: `git branch | grep task/`\n")
	sb.WriteString("3. For each task in merge queue:\n")
	sb.WriteString("   - Merge to main: `git checkout main && git merge task/<id> --no-ff -m \"Merge task/<id>: <title>\"`\n")
	sb.WriteString("   - Resolve any conflicts\n")
	sb.WriteString("   - Delete branch: `git branch -d task/<id>`\n")
	sb.WriteString("   - **Move to done**: `foundry task move <id> d`\n")
	sb.WriteString("4. Run final tests: `go test ./...`\n")
	sb.WriteString("5. Push to remote: `git push origin main`\n\n")

	sb.WriteString("**IMPORTANT**: After merging each task, move it to done with `foundry task move <id> d`\n\n")

	sb.WriteString(leader.SequentialThinkingTriggers)
	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseMerge))

	return sb.String()
}

func buildDeployPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a DEPLOY LEADER for the Forge supervisor. Your job is to merge reviewed work to main and deploy.\n\n")

	// Collect and categorize tasks from merge queue
	var cicdTasks, fixTasks, featureTasks []*kanban.Issue
	for _, col := range board.Columns {
		if col.Status == kanban.StatusMerge {
			for _, issue := range col.Issues {
				// Categorize by labels
				isCICD := hasAnyLabel(issue, "ci", "cicd", "ci/cd", "pipeline", "build")
				isFix := hasAnyLabel(issue, "bug", "fix", "hotfix", "bugfix", "patch")
				if isCICD {
					cicdTasks = append(cicdTasks, issue)
				} else if isFix {
					fixTasks = append(fixTasks, issue)
				} else {
					featureTasks = append(featureTasks, issue)
				}
			}
		}
	}

	totalTasks := len(cicdTasks) + len(fixTasks) + len(featureTasks)

	sb.WriteString("## Priority Order for Merging\n\n")
	sb.WriteString("Merge in this order to maintain stability:\n")
	sb.WriteString("1. **CI/CD fixes** - restore build pipeline first\n")
	sb.WriteString("2. **Bug fixes** - fix broken functionality\n")
	sb.WriteString("3. **Features** - add new capabilities last\n\n")

	sb.WriteString("## Tasks Ready to Merge\n\n")

	if len(cicdTasks) > 0 {
		sb.WriteString("### 🔧 CI/CD (merge first)\n")
		for _, issue := range cicdTasks {
			branchSuffix := extractBranchSuffix(issue.ID)
			sb.WriteString(fmt.Sprintf("- `%s`: %s (branch: task/%s)\n", issue.ID[:8], issue.Title, branchSuffix))
		}
		sb.WriteString("\n")
	}

	if len(fixTasks) > 0 {
		sb.WriteString("### 🐛 Fixes (merge second)\n")
		for _, issue := range fixTasks {
			branchSuffix := extractBranchSuffix(issue.ID)
			sb.WriteString(fmt.Sprintf("- `%s`: %s (branch: task/%s)\n", issue.ID[:8], issue.Title, branchSuffix))
		}
		sb.WriteString("\n")
	}

	if len(featureTasks) > 0 {
		sb.WriteString("### ✨ Features (merge last)\n")
		for _, issue := range featureTasks {
			branchSuffix := extractBranchSuffix(issue.ID)
			sb.WriteString(fmt.Sprintf("- `%s`: %s (branch: task/%s)\n", issue.ID[:8], issue.Title, branchSuffix))
		}
		sb.WriteString("\n")
	}

	if totalTasks == 0 {
		sb.WriteString("(no tasks in merge queue)\n\n")
	}

	sb.WriteString("## Viewing Task Details\n\n")
	sb.WriteString("Use these commands to inspect tasks before merging:\n")
	sb.WriteString("- `foundry task show <id>` - Full task details with branch info\n")
	sb.WriteString("- `foundry task diff <id>` - See code changes for a task\n")
	sb.WriteString("- `foundry task log <id>` - Git commit history for a task\n")
	sb.WriteString("- `foundry task branches` - List all task branches with status\n")
	sb.WriteString("- `foundry task list --status merge` - Tasks in merge queue\n\n")

	sb.WriteString("## FIRST: Check CI Health\n\n")
	sb.WriteString("Before merging any tasks, CHECK if CI is currently passing:\n")
	sb.WriteString("```bash\n")
	sb.WriteString("gh run list --limit 5  # Check recent CI runs\n")
	sb.WriteString("```\n\n")
	sb.WriteString("**IF CI IS FAILING on main:**\n")
	sb.WriteString("1. Identify the failure: `gh run view <run-id> --log-failed`\n")
	sb.WriteString("2. FIX the CI issue FIRST before merging any new tasks\n")
	sb.WriteString("3. Common fixes: missing dependencies in BUILD files, test failures\n")
	sb.WriteString("4. After fixing, verify CI passes, then continue with merges\n\n")
	sb.WriteString("**DO NOT merge new features if CI is broken** - fix CI first!\n\n")
	sb.WriteString("## SECOND: Sync with Remote\n\n")
	sb.WriteString("After CI is green, sync main with origin:\n")
	sb.WriteString("```bash\n")
	sb.WriteString("git checkout main\n")
	sb.WriteString("git fetch origin\n")
	sb.WriteString("git merge origin/main  # or: git pull origin main\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Workflow\n\n")
	sb.WriteString("For each task in priority order:\n")
	sb.WriteString("1. **Check branch**: `git log main..task/<id> --oneline`\n")
	sb.WriteString("2. **Run tests first**: `go test ./...` (or `make test`)\n")
	sb.WriteString("3. **Merge to main**: `git checkout main && git merge task/<id> --no-ff -m \"Merge task/<id>: <title>\"`\n")
	sb.WriteString("4. **Resolve conflicts** if any\n")
	sb.WriteString("5. **Verify build**: `go build ./...` (or `make build`)\n")
	sb.WriteString("6. **Push to remote**: `git push origin main` (REQUIRED after each merge)\n")
	sb.WriteString("7. **Move to done**: `foundry task move <id> done`\n")
	sb.WriteString("8. **Delete branch**: `git branch -d task/<id>`\n\n")

	sb.WriteString("## After All Merges\n\n")
	sb.WriteString("1. Verify all changes pushed: `git status` (should show 'up to date')\n")
	sb.WriteString("2. Check CI status: `gh run list --limit 1`\n")
	sb.WriteString("3. If CI fails, create a high-priority task to fix it\n\n")

	sb.WriteString("## Continuous Operation (Ralph Loop)\n\n")
	sb.WriteString("You run CONTINUOUSLY until all merge tasks are processed:\n")
	sb.WriteString("1. Process tasks in priority order (CI/CD → fixes → features)\n")
	sb.WriteString("2. After each merge, check for more: `foundry task list --status merge`\n")
	sb.WriteString("3. If more items exist, continue merging them\n")
	sb.WriteString("4. ONLY output the completion promise when merge queue is EMPTY\n\n")

	sb.WriteString(leader.SequentialThinkingTriggers)
	sb.WriteString(leader.ErrorHandlingGuidance)
	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseDeploy))

	return sb.String()
}

// hasAnyLabel checks if an issue has any of the specified labels (case-insensitive).
func hasAnyLabel(issue *kanban.Issue, labels ...string) bool {
	for _, issueLabel := range issue.Labels {
		lower := strings.ToLower(issueLabel)
		for _, target := range labels {
			if lower == target || strings.Contains(lower, target) {
				return true
			}
		}
	}
	return false
}

// extractBranchSuffix extracts the branch suffix from a task ID.
// Task IDs are like "blocks-forge-0vk" and branches are "task/0vk".
// Returns the part after the last hyphen.
func extractBranchSuffix(taskID string) string {
	lastHyphen := strings.LastIndex(taskID, "-")
	if lastHyphen >= 0 && lastHyphen < len(taskID)-1 {
		return taskID[lastHyphen+1:]
	}
	// Fallback: use first 8 chars if no hyphen
	if len(taskID) >= 8 {
		return taskID[:8]
	}
	return taskID
}

func buildTaskPrompt(task *kanban.Issue) string {
	var sb strings.Builder

	sb.WriteString("## Task: ")
	sb.WriteString(task.Title)
	sb.WriteString("\n\n")

	if task.Description != "" {
		sb.WriteString(task.Description)
		sb.WriteString("\n\n")
	}

	if task.Priority == kanban.PriorityCritical || task.Priority == kanban.PriorityHigh {
		sb.WriteString("**Priority**: HIGH - please focus on completing this efficiently.\n\n")
	}

	// Add workflow rules
	sb.WriteString("## IMPORTANT: Workflow Rules\n\n")
	sb.WriteString("- Do NOT move your task to 'done' - the supervisor handles task transitions\n")
	sb.WriteString("- When finished, just output your completion promise\n")
	sb.WriteString("- The supervisor will move your task to 'review' automatically\n")
	sb.WriteString("- The reviewer will validate your work and move it to 'done'\n")
	sb.WriteString("- Do NOT run `foundry task move <id> done` - this breaks the workflow\n\n")

	// Add parallel workflow guidance
	sb.WriteString("## Parallel Workflow\n\n")
	sb.WriteString("You are working in a task-specific worktree with its own branch.\n")
	sb.WriteString("- Commit your changes to YOUR branch (check `git branch`)\n")
	sb.WriteString("- The reviewer will see your changes via the branch\n")
	sb.WriteString("- Do NOT merge to main - let the merge leader handle that\n")
	sb.WriteString("- Run tests to verify your changes work in isolation\n\n")

	return sb.String()
}

func buildGroomerPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a BACKLOG GROOMER for the Forge supervisor. Your job is to research and detail backlog items, then move well-defined items to todo.\n\n")

	sb.WriteString("## Backlog Items\n\n")
	backlogCount := 0
	totalBacklog := 0
	maxItems := 10 // Limit to prevent huge prompts
	for _, col := range board.Columns {
		if col.Status == kanban.StatusBacklog {
			totalBacklog = len(col.Issues)
			for _, issue := range col.Issues {
				if backlogCount >= maxItems {
					break
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
				if issue.Description != "" {
					// Truncate long descriptions
					desc := issue.Description
					if len(desc) > 200 {
						desc = desc[:200] + "..."
					}
					sb.WriteString(fmt.Sprintf("  Description: %s\n", desc))
				} else {
					sb.WriteString("  Description: (none - NEEDS DETAIL)\n")
				}
				backlogCount++
			}
		}
	}
	if backlogCount == 0 {
		sb.WriteString("(no items in backlog)\n")
	} else if totalBacklog > maxItems {
		sb.WriteString(fmt.Sprintf("\n(showing %d of %d total backlog items - prioritize these first)\n", maxItems, totalBacklog))
	}

	sb.WriteString("\n## Grooming Criteria\n\n")
	sb.WriteString("A backlog item is READY for todo when it has:\n\n")
	sb.WriteString("1. **Clear Title**: Actionable and specific (e.g., \"Add user authentication\" not \"Auth stuff\")\n")
	sb.WriteString("2. **Detailed Description**: Including:\n")
	sb.WriteString("   - What needs to be done (specific requirements)\n")
	sb.WriteString("   - Acceptance criteria (how to verify it's done)\n")
	sb.WriteString("   - Technical approach (if non-obvious)\n")
	sb.WriteString("   - Dependencies (what must be done first)\n")
	sb.WriteString("3. **Appropriate Priority**: Based on urgency and importance\n")
	sb.WriteString("4. **Reasonable Scope**: Can be completed in one session (break up large items)\n\n")

	sb.WriteString("## Viewing Backlog Items\n\n")
	sb.WriteString("Use these commands to inspect tasks:\n")
	sb.WriteString("- `foundry task show <id>` - Full task details and description\n")
	sb.WriteString("- `foundry task list --status backlog` - All backlog items\n\n")

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. **Research each backlog item**:\n")
	sb.WriteString("   - View task details: `foundry task show <id>`\n")
	sb.WriteString("   - Understand the codebase context: `Grep` and `Read` relevant files\n")
	sb.WriteString("   - Check for existing patterns: How is similar functionality implemented?\n")
	sb.WriteString("   - Identify dependencies: What other code/tasks does this depend on?\n")
	sb.WriteString("   - Check Graphiti for context: `search_nodes({ query: \"<item title>\" })`\n\n")

	sb.WriteString("2. **Update item descriptions** with your research:\n")
	sb.WriteString("   ```bash\n")
	sb.WriteString("   foundry task edit <id> -d \"<detailed description>\"\n")
	sb.WriteString("   ```\n\n")

	sb.WriteString("3. **Break down large items** if needed:\n")
	sb.WriteString("   - Create sub-tasks: `foundry task add \"<subtask>\" -p medium -s backlog -d \"<description>\"`\n")
	sb.WriteString("   - Reference parent: Include \"Part of: <parent-id>\" in description\n\n")

	sb.WriteString("4. **Move ready items to todo**:\n")
	sb.WriteString("   ```bash\n")
	sb.WriteString("   foundry task move <id> todo\n")
	sb.WriteString("   ```\n\n")

	sb.WriteString("5. **Prioritize strategically**:\n")
	sb.WriteString("   - `critical`: Blocking other work, must do immediately\n")
	sb.WriteString("   - `high`: Important for current goals\n")
	sb.WriteString("   - `medium`: Should do soon\n")
	sb.WriteString("   - `low`: Nice to have\n")
	sb.WriteString("   - Update: `foundry task edit <id> -p <priority>`\n\n")

	sb.WriteString("## Continuous Operation (Ralph Loop)\n\n")
	sb.WriteString("You run CONTINUOUSLY until all backlog items are processed:\n")
	sb.WriteString("1. Process each backlog item shown above\n")
	sb.WriteString("2. After processing, check for more: `foundry task list --status backlog`\n")
	sb.WriteString("3. If more items exist, continue grooming them\n")
	sb.WriteString("4. ONLY output the completion promise when backlog is EMPTY or all items are detailed\n\n")

	sb.WriteString("You run in PARALLEL with workers - they may be implementing tasks while you groom.\n")
	sb.WriteString("- Focus on items WITHOUT active workers\n")
	sb.WriteString("- Don't move items that are already being worked on\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen backlog is EMPTY or fully detailed, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseGroomer))

	return sb.String()
}

// buildBoardStateSection builds the dynamic board state section for custom prompts.
func buildBoardStateSection(board *kanban.Board, workDir, promise string) string {
	var sb strings.Builder

	sb.WriteString("## Current Board State\n\n")
	for _, col := range board.Columns {
		sb.WriteString(fmt.Sprintf("### %s (%d)\n", col.Status, len(col.Issues)))
		for _, issue := range col.Issues {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID, issue.Title))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Working directory: %s\n", workDir))
	sb.WriteString(fmt.Sprintf("\nWhen finished, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", promise))

	return sb.String()
}

// buildMergeQueueSection builds the dynamic merge queue section for custom prompts.
func buildMergeQueueSection(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("## Tasks Ready to Merge\n\n")
	mergeCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusMerge {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID, issue.Title))
				mergeCount++
			}
		}
	}
	if mergeCount == 0 {
		sb.WriteString("(no tasks ready to merge - you may output the completion promise)\n")
	}

	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n", workDir))
	sb.WriteString(fmt.Sprintf("\nWhen merge queue is EMPTY, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseMerge))

	return sb.String()
}

// buildDoneTasksSection builds the dynamic done tasks section for custom prompts.
func buildDoneTasksSection(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("## Completed Tasks for Deployment\n\n")
	doneCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusDone {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID, issue.Title))
				doneCount++
			}
		}
	}
	if doneCount == 0 {
		sb.WriteString("(no tasks completed - waiting for merge)\n")
	}

	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n", workDir))
	sb.WriteString(fmt.Sprintf("\nWhen deployment is complete, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseDeploy))

	return sb.String()
}

// buildBacklogSection builds the dynamic backlog section for custom prompts.
func buildBacklogSection(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("## Current Backlog Items\n\n")
	backlogCount := 0
	maxItems := 15
	for _, col := range board.Columns {
		if col.Status == kanban.StatusBacklog {
			for _, issue := range col.Issues {
				if backlogCount >= maxItems {
					sb.WriteString(fmt.Sprintf("\n(showing %d of %d items - use `foundry task list --status backlog` to see all)\n", maxItems, len(col.Issues)))
					break
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID, issue.Title))
				if issue.Description == "" {
					sb.WriteString("  ⚠️ NEEDS DETAIL\n")
				}
				backlogCount++
			}
		}
	}
	if backlogCount == 0 {
		sb.WriteString("(no items in backlog - you may output the completion promise)\n")
	}

	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n", workDir))
	sb.WriteString(fmt.Sprintf("\nWhen backlog is fully detailed, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseGroomer))

	return sb.String()
}

func buildTaskAnalyzerPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a TASK ANALYZER for the Forge supervisor. Your job is to review work and determine if tasks are stuck or new tasks are needed.\n\n")

	sb.WriteString("## Current Board State\n\n")
	for _, col := range board.Columns {
		sb.WriteString(fmt.Sprintf("### %s (%d)\n", col.Status, len(col.Issues)))
		for _, issue := range col.Issues {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
			if issue.Description != "" {
				sb.WriteString(fmt.Sprintf("  Description: %s\n", issue.Description))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Your Analysis Tasks\n\n")
	sb.WriteString("1. **Check for stuck tasks** in 'in_progress':\n")
	sb.WriteString("   - Look for worker blockers: `search_memory_facts({ query: \"blocker:worker\" })`\n")
	sb.WriteString("   - Check git for activity: `git log --oneline -10`\n")
	sb.WriteString("   - If task has no recent progress, consider requeuing\n\n")

	sb.WriteString("2. **Review completed tasks** in 'done':\n")
	sb.WriteString("   - Check if the work is truly complete\n")
	sb.WriteString("   - Look for follow-up tasks that should be created\n")
	sb.WriteString("   - Check if the task revealed new requirements\n\n")

	sb.WriteString("3. **Check for orphaned/incomplete work**:\n")
	sb.WriteString("   - Use `git status` and `git diff` to see uncommitted changes\n")
	sb.WriteString("   - Check for TODO comments: `grep -r 'TODO\\|FIXME' . --include='*.go'`\n")
	sb.WriteString("   - Look for incomplete features or missing tests\n\n")

	sb.WriteString("4. **Requeue stuck tasks**:\n")
	sb.WriteString("   - Move back to todo: `foundry task move <id> todo`\n")
	sb.WriteString("   - Add note explaining why: `foundry task edit <id> -d \"Requeued: <reason>\"`\n\n")

	sb.WriteString("5. **Create new tasks if needed**:\n")
	sb.WriteString("   - Follow-up work: `foundry task add \"title\" -d \"description\" -p medium -s todo`\n")
	sb.WriteString("   - Bugs found: `foundry task add \"Fix: issue\" -p high -s todo`\n")
	sb.WriteString("   - Refactoring: `foundry task add \"Refactor: area\" -p low -s backlog`\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))

	sb.WriteString("## Output Format\n\n")
	sb.WriteString("Structure your findings as follows:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"requeued\": [\n")
	sb.WriteString("    {\"id\": \"abc123\", \"reason\": \"No progress in 30 minutes\"}\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"created\": [\n")
	sb.WriteString("    {\"title\": \"Fix: ...\", \"priority\": \"high\"}\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"health\": \"green|yellow|red\",\n")
	sb.WriteString("  \"summary\": \"Brief text summary\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")

	sb.WriteString(fmt.Sprintf("When finished, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseAnalyzer))

	return sb.String()
}

func buildMonitorPrompt(workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are an INFRASTRUCTURE MONITOR for the Forge supervisor.\n")
	sb.WriteString("Your job is to observe and report on infrastructure health.\n\n")

	sb.WriteString("## What to Monitor\n\n")

	sb.WriteString("### 1. K0s Cluster Health\n")
	sb.WriteString("- Node status and readiness\n")
	sb.WriteString("- Pod health across namespaces\n")
	sb.WriteString("- Resource utilization (CPU, memory, disk)\n")
	sb.WriteString("- Cluster events and warnings\n\n")

	sb.WriteString("### 2. Prometheus Metrics\n")
	sb.WriteString("- Active alerts and their severity\n")
	sb.WriteString("- Key metrics trending (error rates, latency)\n")
	sb.WriteString("- Scrape target health\n")
	sb.WriteString("- Alert rule evaluation\n\n")

	sb.WriteString("### 3. Loki Logs\n")
	sb.WriteString("- Error patterns across services\n")
	sb.WriteString("- Warning frequency and trends\n")
	sb.WriteString("- Log volume anomalies\n")
	sb.WriteString("- Critical service logs\n\n")

	sb.WriteString("### 4. Distributed Tracing\n")
	sb.WriteString("- Trace error rates\n")
	sb.WriteString("- Latency percentiles (p50, p95, p99)\n")
	sb.WriteString("- Service dependency health\n")
	sb.WriteString("- Slow endpoints\n\n")

	sb.WriteString("### 5. Flux GitOps Status\n")
	sb.WriteString("- Kustomization reconciliation status\n")
	sb.WriteString("- HelmRelease health\n")
	sb.WriteString("- GitRepository sync status\n")
	sb.WriteString("- Image update automation status\n\n")

	sb.WriteString("### 6. Service Health\n")
	sb.WriteString("- Deployment readiness\n")
	sb.WriteString("- Service endpoint availability\n")
	sb.WriteString("- Ingress/route status\n")
	sb.WriteString("- Certificate expiration\n\n")

	sb.WriteString("## Loading Context from Memory\n\n")
	sb.WriteString("At the start of your session, load relevant context from Graphiti:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("# Search for infrastructure context\n")
	sb.WriteString("search_nodes({ query: \"infrastructure k0s prometheus\" })\n\n")
	sb.WriteString("# Search for recent alerts or issues\n")
	sb.WriteString("search_memory_facts({ query: \"alert error infrastructure\" })\n\n")
	sb.WriteString("# Search for monitoring configuration\n")
	sb.WriteString("search_memory_facts({ query: \"monitor config threshold\" })\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Monitoring Commands\n\n")
	sb.WriteString("Use kubectl and CLI tools to gather data:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# K0s cluster status\n")
	sb.WriteString("kubectl get nodes -o wide\n")
	sb.WriteString("kubectl get pods -A --field-selector=status.phase!=Running\n\n")
	sb.WriteString("# Prometheus alerts\n")
	sb.WriteString("kubectl exec -n monitoring prometheus-0 -- wget -qO- http://localhost:9090/api/v1/alerts | jq '.data.alerts[] | select(.state==\"firing\")'\n\n")
	sb.WriteString("# Flux status\n")
	sb.WriteString("flux get all -A\n\n")
	sb.WriteString("# Service health\n")
	sb.WriteString("kubectl get deployments -A\n")
	sb.WriteString("kubectl get ingress -A\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Reporting Format\n\n")
	sb.WriteString("When you find issues, report them clearly:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("### Health Report\n\n")
	sb.WriteString("**Cluster Status**: [healthy/degraded/critical]\n")
	sb.WriteString("**Active Alerts**: [count]\n")
	sb.WriteString("**Services Down**: [list]\n\n")
	sb.WriteString("#### Issues Found\n")
	sb.WriteString("1. [Issue description]\n")
	sb.WriteString("   - Severity: [critical/warning/info]\n")
	sb.WriteString("   - Component: [affected component]\n")
	sb.WriteString("   - Recommendation: [suggested action]\n\n")
	sb.WriteString("#### Metrics Summary\n")
	sb.WriteString("- Error rate: [value]\n")
	sb.WriteString("- P95 latency: [value]\n")
	sb.WriteString("- Pod restarts (24h): [count]\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Writing Health Status File\n\n")
	sb.WriteString("**IMPORTANT**: After each monitoring cycle, write the health status to a JSON file.\n")
	sb.WriteString("This allows other tools to read the current system health.\n\n")
	sb.WriteString("Write to: `.forge/health/status.json` in the workspace root.\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# Create health directory if needed\n")
	sb.WriteString("mkdir -p .forge/health\n\n")
	sb.WriteString("# Write status file (use cat with heredoc for JSON)\n")
	sb.WriteString("cat > .forge/health/status.json << 'EOF'\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"timestamp\": \"2024-02-05T12:00:00Z\",\n")
	sb.WriteString("  \"status\": \"healthy|degraded|critical\",\n")
	sb.WriteString("  \"cluster\": {\n")
	sb.WriteString("    \"nodes_ready\": 3,\n")
	sb.WriteString("    \"nodes_total\": 3,\n")
	sb.WriteString("    \"pods_running\": 45,\n")
	sb.WriteString("    \"pods_failed\": 0\n")
	sb.WriteString("  },\n")
	sb.WriteString("  \"alerts\": {\n")
	sb.WriteString("    \"critical\": 0,\n")
	sb.WriteString("    \"warning\": 2,\n")
	sb.WriteString("    \"info\": 5\n")
	sb.WriteString("  },\n")
	sb.WriteString("  \"issues\": [\n")
	sb.WriteString("    {\n")
	sb.WriteString("      \"severity\": \"warning\",\n")
	sb.WriteString("      \"component\": \"prometheus\",\n")
	sb.WriteString("      \"message\": \"High memory usage on prometheus-0\",\n")
	sb.WriteString("      \"recommendation\": \"Consider increasing memory limits\"\n")
	sb.WriteString("    }\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"services\": {\n")
	sb.WriteString("    \"healthy\": [\"api\", \"web\", \"worker\"],\n")
	sb.WriteString("    \"degraded\": [],\n")
	sb.WriteString("    \"down\": []\n")
	sb.WriteString("  },\n")
	sb.WriteString("  \"metrics\": {\n")
	sb.WriteString("    \"error_rate_percent\": 0.1,\n")
	sb.WriteString("    \"p95_latency_ms\": 250,\n")
	sb.WriteString("    \"pod_restarts_24h\": 3\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	sb.WriteString("EOF\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Update this file after EACH monitoring check with current values.\n")
	sb.WriteString("The file is read by `foundry health status` command.\n\n")

	sb.WriteString("## Saving to Memory\n\n")
	sb.WriteString("Also save important findings to Graphiti for historical context:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("add_memory({\n")
	sb.WriteString("  group_id: \"forge-monitor\",\n")
	sb.WriteString("  content: `\n")
	sb.WriteString("    TIMESTAMP: [current time]\n")
	sb.WriteString("    CLUSTER_STATUS: [overall health]\n")
	sb.WriteString("    ALERTS: [active alert summary]\n")
	sb.WriteString("    ISSUES: [issues found]\n")
	sb.WriteString("    RECOMMENDATIONS: [suggested actions]\n")
	sb.WriteString("  `\n")
	sb.WriteString("})\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Continuous Operation\n\n")
	sb.WriteString("You run continuously until:\n")
	sb.WriteString("1. All systems are healthy (no active alerts)\n")
	sb.WriteString("2. All issues have been documented\n")
	sb.WriteString("3. Recommendations have been recorded\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen monitoring is complete and all systems are healthy, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseMonitor))

	return sb.String()
}

func buildTesterPrompt(workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a UI/API TESTER for the Forge supervisor.\n")
	sb.WriteString("Your job is to validate UI changes and test APIs using browser automation.\n\n")

	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("You have access to the Chrome extension MCP tools for browser automation:\n")
	sb.WriteString("- `mcp__claude-in-chrome__navigate` - Navigate to URLs\n")
	sb.WriteString("- `mcp__claude-in-chrome__read_page` - Read page content\n")
	sb.WriteString("- `mcp__claude-in-chrome__take_screenshot` - Capture screenshots\n")
	sb.WriteString("- `mcp__claude-in-chrome__form_input` - Fill form fields\n")
	sb.WriteString("- `mcp__claude-in-chrome__computer` - Click, scroll, interact\n")
	sb.WriteString("- `mcp__claude-in-chrome__javascript_tool` - Execute JavaScript\n\n")

	sb.WriteString("## What to Test\n\n")

	sb.WriteString("### 1. UI Validation\n")
	sb.WriteString("- Page loads correctly\n")
	sb.WriteString("- UI components render properly\n")
	sb.WriteString("- Forms work as expected\n")
	sb.WriteString("- Error states display correctly\n")
	sb.WriteString("- Responsive design works\n\n")

	sb.WriteString("### 2. API Testing\n")
	sb.WriteString("- Endpoints return expected responses\n")
	sb.WriteString("- Error handling works correctly\n")
	sb.WriteString("- Authentication flows work\n")
	sb.WriteString("- Data validation is enforced\n\n")

	sb.WriteString("### 3. Integration Testing\n")
	sb.WriteString("- UI correctly displays API data\n")
	sb.WriteString("- Form submissions work end-to-end\n")
	sb.WriteString("- Navigation flows work correctly\n\n")

	sb.WriteString("## Creating Tasks for Issues\n\n")
	sb.WriteString("When you find issues, CREATE A BACKLOG TASK:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# For UI bugs\n")
	sb.WriteString("foundry task add \"Fix: <UI issue>\" -p high -s backlog -d \"<detailed description with steps to reproduce>\"\n\n")
	sb.WriteString("# For API issues\n")
	sb.WriteString("foundry task add \"Fix: <API issue>\" -p high -s backlog -d \"<endpoint, expected vs actual>\"\n\n")
	sb.WriteString("# For performance issues\n")
	sb.WriteString("foundry task add \"Optimize: <performance issue>\" -p medium -s backlog -d \"<what's slow and where>\"\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Test Report Format\n\n")
	sb.WriteString("After each test cycle, report:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("## Test Report - [timestamp]\n\n")
	sb.WriteString("**Tests Run**: [count]\n")
	sb.WriteString("**Passed**: [count]\n")
	sb.WriteString("**Failed**: [count]\n\n")
	sb.WriteString("### Failed Tests\n")
	sb.WriteString("1. [Test name]: [failure reason]\n")
	sb.WriteString("   - Steps to reproduce: [...]\n")
	sb.WriteString("   - Expected: [...]\n")
	sb.WriteString("   - Actual: [...]\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Saving to Memory\n\n")
	sb.WriteString("Save test results to Graphiti:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("add_memory({\n")
	sb.WriteString("  group_id: \"forge-tester\",\n")
	sb.WriteString("  content: `\n")
	sb.WriteString("    TIMESTAMP: [current time]\n")
	sb.WriteString("    TESTS_RUN: [count]\n")
	sb.WriteString("    PASSED: [count]\n")
	sb.WriteString("    FAILED: [count]\n")
	sb.WriteString("    ISSUES_CREATED: [task IDs]\n")
	sb.WriteString("    COVERAGE: [areas tested]\n")
	sb.WriteString("  `\n")
	sb.WriteString("})\n")
	sb.WriteString("```\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen testing is complete, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseTester))

	return sb.String()
}

func buildPMPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a PROJECT MANAGER for the Forge supervisor.\n")
	sb.WriteString("Your job is to identify the next best high-value tasks based on completed work.\n\n")

	sb.WriteString("## Your Mission\n\n")
	sb.WriteString("Analyze our completed work and the current state of the project to identify:\n")
	sb.WriteString("1. **5 best next tasks** - ordered by impact and confidence\n")
	sb.WriteString("2. Only include items with **80%+ confidence** that they're the right next step\n")
	sb.WriteString("3. Focus on **customer value** - what would users want most?\n\n")

	sb.WriteString("## Focus Areas\n\n")
	sb.WriteString("When identifying tasks, prioritize:\n\n")
	sb.WriteString("### 1. New Features (user-facing value)\n")
	sb.WriteString("- What features would delight users?\n")
	sb.WriteString("- What gaps exist in our current offering?\n")
	sb.WriteString("- What would differentiate us from alternatives?\n\n")

	sb.WriteString("### 2. Reliability & Stability\n")
	sb.WriteString("- What areas are prone to errors or failures?\n")
	sb.WriteString("- What needs better error handling or recovery?\n")
	sb.WriteString("- What lacks proper testing or validation?\n\n")

	sb.WriteString("### 3. User Experience\n")
	sb.WriteString("- What frustrations do users face?\n")
	sb.WriteString("- What workflows are awkward or slow?\n")
	sb.WriteString("- What documentation or help is missing?\n\n")

	sb.WriteString("### 4. Technical Debt\n")
	sb.WriteString("- What code is hard to maintain?\n")
	sb.WriteString("- What would speed up future development?\n")
	sb.WriteString("- What patterns need standardization?\n\n")

	// Show completed tasks
	sb.WriteString(buildDoneTasksSection(board, workDir))

	// Show current backlog for context
	sb.WriteString("\n## Current Backlog\n\n")
	backlogCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusBacklog {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
				backlogCount++
			}
		}
	}
	if backlogCount == 0 {
		sb.WriteString("(no items in backlog)\n")
	}

	sb.WriteString("\n## Task Creation\n\n")
	sb.WriteString("For each recommended task, create it in the backlog:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("foundry task add \"<title>\" -p <priority> -s backlog -d \"<description>\"\n")
	sb.WriteString("```\n\n")

	sb.WriteString("Priority levels:\n")
	sb.WriteString("- `critical` - Urgent, blocking issues\n")
	sb.WriteString("- `high` - Important for next release\n")
	sb.WriteString("- `medium` - Should do soon\n")
	sb.WriteString("- `low` - Nice to have\n\n")

	sb.WriteString("## Output Format\n\n")
	sb.WriteString("Present your analysis as:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("## PM Analysis - [timestamp]\n\n")
	sb.WriteString("### Completed Work Summary\n")
	sb.WriteString("[Brief summary of what was accomplished]\n\n")
	sb.WriteString("### Top 5 Recommended Tasks\n")
	sb.WriteString("1. **[Task Title]** (Confidence: X%)\n")
	sb.WriteString("   - Category: [Feature/Reliability/UX/TechDebt]\n")
	sb.WriteString("   - Rationale: [Why this is important]\n")
	sb.WriteString("   - Expected Impact: [What users/devs gain]\n\n")
	sb.WriteString("[...repeat for 2-5]\n\n")
	sb.WriteString("### Tasks Created\n")
	sb.WriteString("- [task-id]: [title]\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Saving to Memory\n\n")
	sb.WriteString("Save your analysis to Graphiti:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("add_memory({\n")
	sb.WriteString("  group_id: \"forge-pm\",\n")
	sb.WriteString("  content: `\n")
	sb.WriteString("    TIMESTAMP: [current time]\n")
	sb.WriteString("    CYCLE: [cycle number]\n")
	sb.WriteString("    COMPLETED_REVIEWED: [count]\n")
	sb.WriteString("    TASKS_CREATED: [task IDs]\n")
	sb.WriteString("    TOP_PRIORITY: [most important task]\n")
	sb.WriteString("    THEMES: [patterns observed]\n")
	sb.WriteString("  `\n")
	sb.WriteString("})\n")
	sb.WriteString("```\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen you've identified and created your recommended tasks, output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromisePM))

	return sb.String()
}

func buildCICDPrompt(workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a CI/CD LEADER for the Forge supervisor.\n")
	sb.WriteString("Your job is to monitor CI/CD health, fix broken builds, and ensure deployments are reliable.\n\n")

	sb.WriteString("## Your Mission\n\n")
	sb.WriteString("1. Check if CI is currently healthy\n")
	sb.WriteString("2. If broken, diagnose and fix the issue\n")
	sb.WriteString("3. Create tasks for any issues you can't fix immediately\n")
	sb.WriteString("4. Report on CI/CD status and recommend improvements\n\n")

	sb.WriteString("## FIRST: Check CI Health\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# Check recent CI runs\n")
	sb.WriteString("gh run list --limit 10\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## If CI is FAILING\n\n")
	sb.WriteString("1. **Get failure details**:\n")
	sb.WriteString("```bash\n")
	sb.WriteString("gh run view <run-id> --log-failed\n")
	sb.WriteString("```\n\n")

	sb.WriteString("2. **Common failures and fixes**:\n\n")
	sb.WriteString("| Error Pattern | Cause | Fix |\n")
	sb.WriteString("|--------------|-------|-----|\n")
	sb.WriteString("| `missing strict dependencies` | BUILD file missing dep | Add to deps in BUILD.bazel |\n")
	sb.WriteString("| `undefined: X` | Missing source file | Add to srcs in BUILD |\n")
	sb.WriteString("| `test failed` | Broken test | Fix test or code |\n")
	sb.WriteString("| `timeout` | Slow build | Optimize or add caching |\n\n")

	sb.WriteString("3. **Fix the issue directly** if possible:\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# Edit the file\n")
	sb.WriteString("# Then commit and push\n")
	sb.WriteString("git add <files>\n")
	sb.WriteString("git commit -m \"fix(bazel): <description>\"\n")
	sb.WriteString("git push origin main\n")
	sb.WriteString("```\n\n")

	sb.WriteString("4. **Verify the fix**:\n")
	sb.WriteString("```bash\n")
	sb.WriteString("gh run list --limit 1  # Check if new run started\n")
	sb.WriteString("gh run watch <run-id>  # Wait for completion\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## If you cannot fix immediately\n\n")
	sb.WriteString("Create a task for the issue:\n")
	sb.WriteString("```bash\n")
	sb.WriteString("foundry task add \"Fix: <specific error>\" -p critical -s todo -l bug,cicd\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## CI/CD Status Report\n\n")
	sb.WriteString("After checking CI, provide a status report:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("## CI/CD Health Report\n\n")
	sb.WriteString("| Metric | Status |\n")
	sb.WriteString("|--------|--------|\n")
	sb.WriteString("| **Overall Health** | 🟢 HEALTHY / 🟡 DEGRADED / 🔴 BROKEN |\n")
	sb.WriteString("| **Last Success** | <timestamp> |\n")
	sb.WriteString("| **Recent Failures** | <count> |\n")
	sb.WriteString("| **Pending Fixes** | <count> |\n\n")
	sb.WriteString("### Recent Runs\n")
	sb.WriteString("- ✅/❌ <commit-msg> - <timestamp>\n\n")
	sb.WriteString("### Recommendations\n")
	sb.WriteString("- <improvement suggestion>\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Saving to Memory\n\n")
	sb.WriteString("Save CI status to Graphiti:\n")
	sb.WriteString("```\n")
	sb.WriteString("add_memory({\n")
	sb.WriteString("  group_id: \"forge-cicd\",\n")
	sb.WriteString("  content: `\n")
	sb.WriteString("    TIMESTAMP: [current time]\n")
	sb.WriteString("    CI_STATUS: [healthy/degraded/broken]\n")
	sb.WriteString("    LAST_SUCCESS: [commit hash]\n")
	sb.WriteString("    FAILURES_FIXED: [count]\n")
	sb.WriteString("    PATTERNS: [common failure patterns]\n")
	sb.WriteString("    RECOMMENDATIONS: [improvements]\n")
	sb.WriteString("  `\n")
	sb.WriteString("})\n")
	sb.WriteString("```\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen CI is healthy (all recent builds passing), output EXACTLY this text (including the XML tags):\n```\n<promise>%s</promise>\n```\n", leader.PromiseCICD))

	return sb.String()
}

func printSummary(store *kanban.Store, reg *worker.Registry, state *supervisorState) {
	board, err := store.GetBoard()
	if err != nil {
		return
	}

	counts := make(map[kanban.Status]int)
	for _, col := range board.Columns {
		counts[col.Status] = len(col.Issues)
	}

	// Count workers by status and role
	workers := reg.List()
	var activeWorkers, idleWorkers, stoppedWorkers []string
	var idleLeaders []string
	for _, w := range workers {
		name := w.DisplayName()
		if w.Role == worker.RoleWorker {
			switch w.Status {
			case worker.StatusActive:
				if w.CurrentTask != "" {
					taskShort := w.CurrentTask
					if len(taskShort) > 8 {
						taskShort = taskShort[:8]
					}
					activeWorkers = append(activeWorkers, fmt.Sprintf("%s(%s)", name, taskShort))
				} else {
					activeWorkers = append(activeWorkers, name)
				}
			case worker.StatusIdle:
				idleWorkers = append(idleWorkers, name)
			case worker.StatusStopped:
				stoppedWorkers = append(stoppedWorkers, name)
			}
		} else if w.Status == worker.StatusIdle {
			idleLeaders = append(idleLeaders, fmt.Sprintf("%s(%s)", name, w.Role))
		}
	}

	fmt.Printf("\n📊 Tasks: %d backlog, %d todo, %d in-progress, %d review, %d merge, %d done\n",
		counts[kanban.StatusBacklog],
		counts[kanban.StatusTodo],
		counts[kanban.StatusInProgress],
		counts[kanban.StatusReview],
		counts[kanban.StatusMerge],
		counts[kanban.StatusDone],
	)

	// Show worker status
	fmt.Printf("👷 Workers: %d active, %d idle, %d stopped\n",
		len(activeWorkers), len(idleWorkers), len(stoppedWorkers))
	if len(activeWorkers) > 0 {
		fmt.Printf("   Active: %s\n", strings.Join(activeWorkers, ", "))
	}
	if len(idleLeaders) > 0 {
		fmt.Printf("   Idle leaders: %s\n", strings.Join(idleLeaders, ", "))
	}

	// Show active leaders
	var activeLeaders []string
	if state.planner.running {
		activeLeaders = append(activeLeaders, "📋planner")
	}
	if state.reviewer.running {
		activeLeaders = append(activeLeaders, "🔍reviewer")
	}
	if state.merge.running {
		activeLeaders = append(activeLeaders, "🔀merge")
	}
	if state.deploy.running {
		activeLeaders = append(activeLeaders, "🚀deploy")
	}
	if state.analyzer.running {
		activeLeaders = append(activeLeaders, "🔬analyzer")
	}
	if state.monitor.running {
		activeLeaders = append(activeLeaders, "📡monitor")
	}
	if state.tester.running {
		activeLeaders = append(activeLeaders, "🧪tester")
	}
	if state.groomer.running {
		activeLeaders = append(activeLeaders, "🧹groomer")
	}

	if len(activeLeaders) > 0 {
		fmt.Printf("👔 Leaders running: %s\n", strings.Join(activeLeaders, ", "))
	}

	// Show health status
	if state.lastHealth != nil {
		h := state.lastHealth
		switch h.Status {
		case health.StatusGood:
			fmt.Printf("💚 Health: good (CPU %.0f%%, Mem %.0f%%, fail %.0f%%)\n", h.CPUPercent, h.MemoryPercent, h.FailureRate*100)
		case health.StatusDegraded:
			fmt.Printf("🟡 Health: degraded — %s\n", h.Reason)
		case health.StatusCritical:
			fmt.Printf("🔴 Health: critical — %s\n", h.Reason)
		}
	}

	// Check milestones
	if state.allTasksDone {
		fmt.Printf("🎉 All tasks completed!\n")
	}
	if state.mergeCompleted {
		fmt.Printf("✅ Merge completed!\n")
	}
	if state.deployCompleted {
		fmt.Printf("🚀 Deployed!\n")
	}
}

func printHealthMetrics(m *health.Metrics) {
	fmt.Printf("   CPU: %.0f%% | Mem: %.0f%% | Failures: %.0f%% | API errors: %d\n",
		m.CPUPercent, m.MemoryPercent, m.FailureRate*100, m.APIErrors)
}

func getKanbanStoreForDir(dir string) (*kanban.Store, error) {
	// Store wraps bd CLI which looks for .beads/ in the working directory
	return kanban.NewStore(dir)
}

func buildNudgeMessage(w *worker.Worker, pokeCount int) string {
	// Use JSON-structured poke message
	return leader.BuildPokeMessage(pokeCount)
}

// worktreesDir returns the directory for task worktrees.
func worktreesDir(baseDir string) string {
	return filepath.Join(baseDir, ".forge", "worktrees")
}

// findGitRepo finds the primary git repository for worktree operations.
// If baseDir is a git repo, returns it. Otherwise looks in .forge/repos/.
func findGitRepo(baseDir string) (string, error) {
	// Check if baseDir itself is a git repo
	cmd := exec.Command("git", "-C", baseDir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err == nil {
		return baseDir, nil
	}

	// Look for repos in .forge/repos/
	reposDir := filepath.Join(baseDir, ".forge", "repos")
	entries, err := os.ReadDir(reposDir)
	if err != nil {
		return "", fmt.Errorf("workdir is not a git repo and no repos found in .forge/repos/")
	}

	// Find first directory that's a git repo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(reposDir, entry.Name())
		cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir")
		if err := cmd.Run(); err == nil {
			fmt.Printf("📂 Using git repo: %s\n", repoPath)
			return repoPath, nil
		}
	}

	return "", fmt.Errorf("no git repos found in .forge/repos/")
}

// createLeaderWorktree creates or reuses a worktree for a leader role.
// Leaders get persistent worktrees that survive across sessions.
func createLeaderWorktree(baseDir, role string) (string, error) {
	// Find the git repo (may be in .forge/repos/ if baseDir is a workspace)
	gitRepo, err := findGitRepo(baseDir)
	if err != nil {
		return baseDir, err
	}

	// Create worktrees directory (in the git repo, not baseDir)
	wtDir := worktreesDir(gitRepo)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return "", fmt.Errorf("creating worktrees directory: %w", err)
	}

	// Leader worktree path
	worktreePath := filepath.Join(wtDir, role+"-workspace")

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		// Already exists, reuse it - pull latest changes
		pullCmd := exec.Command("git", "-C", worktreePath, "pull", "--rebase", "--autostash")
		_ = pullCmd.Run() // Ignore errors, best effort
		return worktreePath, nil
	}

	// Create worktree on HEAD (tracks main branch)
	args := []string{
		"-C", gitRepo,
		"worktree", "add",
		worktreePath,
		"HEAD",
		"--detach", // Detached HEAD so it doesn't conflict with main
	}

	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("creating leader worktree: %w\n%s", err, string(output))
	}

	fmt.Printf("   📁 Created leader worktree: %s-workspace\n", role)
	return worktreePath, nil
}

// createTaskWorktree creates a git worktree for a task.
// Returns the worktree path. Returns error if no git repo found or creation fails.
func createTaskWorktree(baseDir, taskID string) (string, error) {
	// Find the git repo (may be in .forge/repos/ if baseDir is a workspace)
	gitRepo, err := findGitRepo(baseDir)
	if err != nil {
		return "", err
	}

	// Create worktrees directory (in the git repo, not baseDir)
	wtDir := worktreesDir(gitRepo)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return "", fmt.Errorf("creating worktrees directory: %w", err)
	}

	// Worktree path uses unique suffix of task ID
	// Task IDs like "blocks-forge-0vk" share prefix but have unique suffix
	shortID := taskID
	if idx := strings.LastIndex(taskID, "-"); idx > 0 && idx < len(taskID)-1 {
		// Use suffix after last dash (e.g., "0vk" from "blocks-forge-0vk")
		shortID = taskID[idx+1:]
	} else if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	worktreePath := filepath.Join(wtDir, shortID)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		// Already exists, reuse it
		return worktreePath, nil
	}

	// Get current branch from git repo
	branchCmd := exec.Command("git", "-C", gitRepo, "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, err := branchCmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	baseBranch := strings.TrimSpace(string(branchOut))

	// Create a new branch for this task
	taskBranch := fmt.Sprintf("task/%s", shortID)

	// Create worktree with new branch based on current HEAD
	args := []string{
		"-C", gitRepo,
		"worktree", "add",
		"-b", taskBranch,
		worktreePath,
		"HEAD",
	}

	cmd := exec.Command("git", args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		// If branch exists, try without -b
		args = []string{
			"-C", gitRepo,
			"worktree", "add",
			worktreePath,
			taskBranch,
		}
		cmd = exec.Command("git", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("creating worktree: %w\n%s", err, string(output))
		}
	}

	fmt.Printf("   📁 Created worktree: %s (branch: %s from %s)\n", shortID, taskBranch, baseBranch)
	return worktreePath, nil
}

// removeTaskWorktree removes a task's worktree.
func removeTaskWorktree(baseDir, taskID string) error {
	// Find the git repo
	gitRepo, err := findGitRepo(baseDir)
	if err != nil {
		return nil // No git repo, nothing to remove
	}

	// Use same shortID logic as createTaskWorktree
	shortID := taskID
	if idx := strings.LastIndex(taskID, "-"); idx > 0 && idx < len(taskID)-1 {
		shortID = taskID[idx+1:]
	} else if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	worktreePath := filepath.Join(worktreesDir(gitRepo), shortID)

	// Check if worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return nil // Already gone
	}

	// Remove via git worktree command
	cmd := exec.Command("git", "-C", gitRepo, "worktree", "remove", worktreePath, "--force")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Try manual removal as fallback
		if rmErr := os.RemoveAll(worktreePath); rmErr != nil {
			return fmt.Errorf("removing worktree: %w\n%s", err, string(output))
		}
	}

	return nil
}

// ============================================================================
// SMART SUPERVISOR - AI-powered orchestration using Haiku
// ============================================================================

// smartAction represents a decision from the AI supervisor.
type smartAction struct {
	Action   string `json:"action"` // assign_task, poke_worker, start_reviewer, start_groomer, etc.
	WorkerID string `json:"worker"` // worker name (for assign/poke actions)
	TaskID   string `json:"task"`   // task ID (for assign action)
	Reason   string `json:"reason"` // why this action was chosen
}

// runSmartCycle runs an AI-powered supervisor cycle.
// Instead of fixed rules, it uses Haiku to analyze state and decide actions.
func runSmartCycle(ctx context.Context, cfg supervisorConfig, state *supervisorState) {
	now := time.Now().Format("15:04:05")
	fmt.Printf("\n━━━ Smart Cycle %s ━━━\n", now)

	// Load kanban store
	store, err := getKanbanStoreForDir(cfg.workDir)
	if err != nil {
		fmt.Printf("⚠️  Error loading kanban: %v\n", err)
		return
	}
	defer func() { _ = store.Close() }()

	// Load worker registry
	reg, err := worker.LoadRegistry()
	if err != nil {
		fmt.Printf("⚠️  Error loading workers: %v\n", err)
		return
	}

	// Health check - detect stale workers (always do this, not AI-controlled)
	checkWorkerHealth(ctx, reg, state, store, cfg)

	// Recovery - check for misplaced tasks (always do this)
	recoverMisplacedTasks(store, state, cfg.workDir)

	// Recovery - check for orphaned in-progress tasks
	recoverOrphanedTasks(store, reg, state)

	// Check for completed workers (always do this)
	checkCompletedWorkers(store, reg, state, cfg.workDir)

	// Check leader sessions (always do this if leaders enabled)
	if cfg.withLeaders {
		checkLeaderSessions(reg, state)
	}

	// Build state prompt for AI
	statePrompt := buildSmartStatePrompt(store, reg, state, cfg)

	// Get AI decisions
	actions := getSmartDecisions(statePrompt, cfg)

	// Execute each action
	for _, action := range actions {
		executeSmartAction(action, store, reg, state, cfg)
	}

	// Summary
	printSummary(store, reg, state)
}

// buildSmartStatePrompt creates a comprehensive state description for the AI.
func buildSmartStatePrompt(store *kanban.Store, reg *worker.Registry, state *supervisorState, cfg supervisorConfig) string {
	var sb strings.Builder

	// Board state
	board, err := store.GetBoard()
	if err != nil {
		sb.WriteString("Error loading board state\n")
	} else {
		sb.WriteString("## Kanban Board\n\n")
		for _, col := range board.Columns {
			sb.WriteString(fmt.Sprintf("### %s (%d tasks)\n", col.Status, len(col.Issues)))
			for _, issue := range col.Issues {
				assignedTo := ""
				if workerID, ok := state.taskWorkers[issue.ID]; ok {
					if w := reg.Get(workerID); w != nil {
						assignedTo = fmt.Sprintf(" [assigned: %s]", w.DisplayName())
					}
				}
				sb.WriteString(fmt.Sprintf("- %s: %s%s\n", issue.ID[:8], issue.Title, assignedTo))
			}
			sb.WriteString("\n")
		}
	}

	// Worker state
	sb.WriteString("## Workers\n\n")
	workers := reg.List()
	for _, w := range workers {
		status := string(w.Status)
		extra := ""
		if w.CurrentTask != "" {
			taskShort := w.CurrentTask
			if len(taskShort) > 8 {
				taskShort = taskShort[:8]
			}
			extra = fmt.Sprintf(" (task: %s)", taskShort)
		}
		if w.Role != worker.RoleWorker {
			extra += fmt.Sprintf(" [role: %s]", w.Role)
		}
		sb.WriteString(fmt.Sprintf("- %s: %s%s\n", w.DisplayName(), status, extra))

		// Add activity info for active workers
		if w.Status == worker.StatusActive && w.SessionID != "" {
			session := tmux.NewSession(w.SessionID, "", "")
			if session.Exists() {
				output := captureRecentOutput(session, 10)
				if output != "" {
					// Truncate to last 200 chars
					if len(output) > 200 {
						output = output[len(output)-200:]
					}
					sb.WriteString(fmt.Sprintf("  Recent output: %s\n", strings.ReplaceAll(output, "\n", " | ")))
				}
			}
		}
	}

	// Config constraints
	sb.WriteString("\n## Constraints\n\n")
	sb.WriteString(fmt.Sprintf("- Max concurrent workers: %d\n", cfg.maxConcurrentWorkers))
	sb.WriteString(fmt.Sprintf("- Auto-assign enabled: %v\n", cfg.autoAssign))
	sb.WriteString(fmt.Sprintf("- Leaders enabled: %v\n", cfg.withLeaders))

	// Leader states
	if cfg.withLeaders {
		sb.WriteString("\n## Leader Status\n\n")
		if state.groomer.running {
			sb.WriteString("- Groomer: RUNNING\n")
		}
		if state.reviewer.running {
			sb.WriteString("- Reviewer: RUNNING\n")
		}
		if state.planner.running {
			sb.WriteString("- Planner: RUNNING\n")
		}
		if state.merge.running {
			sb.WriteString("- Merge: RUNNING\n")
		}
		if state.deploy.running {
			sb.WriteString("- Deploy: RUNNING\n")
		}
	}

	return sb.String()
}

// getSmartDecisions calls Haiku to analyze state and return actions.
func getSmartDecisions(statePrompt string, cfg supervisorConfig) []smartAction {
	prompt := fmt.Sprintf(`You are an AI supervisor for a development workflow system.

%s

## Available Actions

You can return a JSON array of actions to take. Available actions:
- {"action": "assign_task", "worker": "<name>", "task": "<id>", "reason": "..."}
- {"action": "poke_worker", "worker": "<name>", "reason": "..."}
- {"action": "start_reviewer", "reason": "..."}
- {"action": "start_groomer", "reason": "..."}
- {"action": "start_planner", "reason": "..."}
- {"action": "start_merge", "reason": "..."}
- {"action": "start_deploy", "reason": "..."}
- {"action": "skip", "reason": "..."}  (do nothing this cycle)

## Decision Guidelines

1. **Assign tasks**: If there are todo tasks and idle workers (role=worker), assign them
2. **Poke workers**: If a worker appears stuck (no recent output), send a nudge
3. **Start groomer**: If backlog has undetailed items and groomer is not running
4. **Start reviewer**: If review queue has items and reviewer is not running
5. **Start planner**: If backlog needs prioritization and no active workers
6. **Start merge**: If all tasks done and no pending work
7. **Start deploy**: After merge completes

## Response Format

Return ONLY a valid JSON array of actions. No markdown, no explanation.
Example: [{"action": "assign_task", "worker": "alpha", "task": "abc123", "reason": "idle worker, todo task available"}]

If no actions needed, return: [{"action": "skip", "reason": "system is healthy"}]`, statePrompt)

	// Call Haiku using shell with proper escaping
	out, err := runClaudeHaiku(prompt)
	if err != nil {
		fmt.Printf("⚠️  Smart decision error: %v\n", err)
		return nil
	}

	// Parse JSON response
	response := strings.TrimSpace(string(out))

	// Extract JSON if wrapped in markdown code block
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	var actions []smartAction
	if err := json.Unmarshal([]byte(response), &actions); err != nil {
		fmt.Printf("⚠️  Failed to parse smart decisions: %v\n", err)
		fmt.Printf("   Response: %s\n", response)
		return nil
	}

	return actions
}

// executeSmartAction executes a single action decided by the AI.
func executeSmartAction(action smartAction, store *kanban.Store, reg *worker.Registry, state *supervisorState, cfg supervisorConfig) {
	fmt.Printf("🤖 %s: %s\n", action.Action, action.Reason)

	switch action.Action {
	case "skip":
		// Do nothing

	case "assign_task":
		w := reg.Get(action.WorkerID)
		if w == nil {
			fmt.Printf("   ⚠️  Worker not found: %s\n", action.WorkerID)
			return
		}

		task, err := store.Get(action.TaskID)
		if err != nil {
			// Try with short ID
			tasks, _ := store.List(kanban.StatusTodo)
			for _, t := range tasks {
				if strings.HasPrefix(t.ID, action.TaskID) {
					task = t
					break
				}
			}
		}
		if task == nil {
			fmt.Printf("   ⚠️  Task not found: %s\n", action.TaskID)
			return
		}

		// Create worktree
		worktreePath, err := createTaskWorktree(cfg.workDir, task.ID)
		if err != nil {
			fmt.Printf("   ⚠️  Error creating worktree: %v\n", err)
			return
		}

		// Move task to in_progress
		if err := store.Move(task.ID, kanban.StatusInProgress); err != nil {
			fmt.Printf("   ⚠️  Error moving task: %v\n", err)
			return
		}

		// Build prompt
		prompt := buildTaskPrompt(task)
		promiseID := task.ID
		if idx := strings.LastIndex(task.ID, "-"); idx > 0 && idx < len(task.ID)-1 {
			promiseID = task.ID[idx+1:]
		} else if len(promiseID) > 8 {
			promiseID = promiseID[:8]
		}
		promise := fmt.Sprintf("TASK_%s_DONE", promiseID)

		// Track assignment
		state.taskWorkers[task.ID] = w.ID
		state.workerTasks[w.ID] = task.ID

		// Start worker
		go func() {
			opts := worker.StartOptions{
				TaskID:   task.ID,
				Worktree: worktreePath,
				Prompt:   prompt,
				Promise:  promise,
			}
			ctx := context.Background()
			if err := worker.Start(ctx, reg, w.ID, opts); err != nil {
				fmt.Printf("   ⚠️  Error starting %s: %v\n", w.DisplayName(), err)
			}
		}()

		fmt.Printf("   ✅ Assigned %s to %s\n", task.ID[:8], w.DisplayName())

	case "poke_worker":
		w := reg.Get(action.WorkerID)
		if w == nil || w.SessionID == "" {
			return
		}

		session := tmux.NewSession(w.SessionID, "", "")
		if !session.Exists() {
			return
		}

		state.pokeCounts[w.ID]++
		nudge := buildNudgeMessage(w, state.pokeCounts[w.ID])
		if err := session.SendKeys(nudge); err != nil {
			fmt.Printf("   ⚠️  Failed to poke: %v\n", err)
		}

	case "start_reviewer":
		if !cfg.withLeaders || state.reviewer.running {
			return
		}
		board, _ := store.GetBoard()
		startReviewer(store, reg, state, cfg.workDir, board, cfg)

	case "start_groomer":
		if !cfg.withLeaders || state.groomer.running {
			return
		}
		board, _ := store.GetBoard()
		startGroomer(store, reg, state, cfg.workDir, board, cfg)

	case "start_planner":
		if !cfg.withLeaders || state.planner.running {
			return
		}
		board, _ := store.GetBoard()
		startPlanner(store, reg, state, cfg.workDir, board, cfg)

	case "start_merge":
		if !cfg.withLeaders || state.merge.running {
			return
		}
		board, _ := store.GetBoard()
		startMerge(store, reg, state, cfg.workDir, board, cfg)

	case "start_deploy":
		if !cfg.withLeaders || state.deploy.running {
			return
		}
		board, _ := store.GetBoard()
		startDeploy(store, reg, state, cfg.workDir, board, cfg)

	default:
		fmt.Printf("   ⚠️  Unknown action: %s\n", action.Action)
	}
}

// syncToGitHub runs the GitHub Projects sync command
func syncToGitHub(state *supervisorState) {
	// Only sync every 5 cycles to avoid rate limiting
	if state.lastMergeCount%5 != 0 {
		return
	}

	cmd := exec.Command("foundry", "board", "--github", "--sync")
	cmd.Stdout = nil // Silent
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		fmt.Printf("⚠️  GitHub sync failed to start: %v\n", err)
		return
	}
	// Don't wait - let it run in background
	go func() {
		_ = cmd.Wait() // Silent failure - sync is best-effort
	}()
}

// StateLogEntry represents a single state log entry
type StateLogEntry struct {
	Timestamp       string          `json:"timestamp"`
	Tasks           map[string]int  `json:"tasks"`
	Workers         WorkerSummary   `json:"workers"`
	Leaders         map[string]bool `json:"leaders"`
	MergeCompleted  bool            `json:"merge_completed"`
	DeployCompleted bool            `json:"deploy_completed"`
	AllTasksDone    bool            `json:"all_tasks_done"`
}

// WorkerSummary summarizes worker state
type WorkerSummary struct {
	Active  int      `json:"active"`
	Idle    int      `json:"idle"`
	Stopped int      `json:"stopped"`
	Names   []string `json:"active_names,omitempty"`
}

// writeStateLog appends state to a JSON lines log file
func writeStateLog(logPath string, store *kanban.Store, reg *worker.Registry, state *supervisorState) {
	// Count tasks by status
	counts := make(map[string]int)
	for _, status := range []kanban.Status{
		kanban.StatusBacklog, kanban.StatusTodo, kanban.StatusInProgress,
		kanban.StatusReview, kanban.StatusDone,
	} {
		issues, _ := store.List(status)
		counts[string(status)] = len(issues)
	}

	// Count workers
	var activeNames []string
	activeCount, idleCount, stoppedCount := 0, 0, 0
	for _, w := range reg.List() {
		switch w.Status {
		case worker.StatusActive:
			activeCount++
			activeNames = append(activeNames, w.DisplayName())
		case worker.StatusIdle:
			idleCount++
		case worker.StatusStopped:
			stoppedCount++
		}
	}

	// Build leader status
	leaders := map[string]bool{
		"planner":  state.planner.running,
		"reviewer": state.reviewer.running,
		"merge":    state.merge.running,
		"deploy":   state.deploy.running,
		"analyzer": state.analyzer.running,
		"groomer":  state.groomer.running,
	}

	entry := StateLogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Tasks:     counts,
		Workers: WorkerSummary{
			Active:  activeCount,
			Idle:    idleCount,
			Stopped: stoppedCount,
			Names:   activeNames,
		},
		Leaders:         leaders,
		MergeCompleted:  state.mergeCompleted,
		DeployCompleted: state.deployCompleted,
		AllTasksDone:    state.allTasksDone,
	}

	// Append to log file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return // Silent failure
	}
	defer func() { _ = f.Close() }()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = f.Write(data)
	_, _ = f.WriteString("\n")
}

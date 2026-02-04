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

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/leader"
	"github.com/zach-source/forge/internal/tmux"
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
		autoRequeue          bool
		cleanupOrphans       bool
		dryRun               bool
		smartMode            bool
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
			return runSupervisor(cmd.Context(), supervisorConfig{
				interval:             interval,
				analyzeInterval:      analyzeInterval,
				stuckThreshold:       stuckThreshold,
				maxPokes:             maxPokes,
				maxConcurrentWorkers: maxConcurrentWorkers,
				workDir:              workDir,
				autoAssign:           autoAssign,
				withLeaders:          withLeaders,
				autoRequeue:          autoRequeue,
				cleanupOrphans:       cleanupOrphans,
				dryRun:               dryRun,
				smartMode:            smartMode,
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
	cmd.Flags().BoolVar(&autoRequeue, "auto-requeue", true, "Automatically requeue stuck tasks")
	cmd.Flags().BoolVar(&cleanupOrphans, "cleanup-orphans", true, "Clean up orphaned tmux sessions on startup (disable with --cleanup-orphans=false)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be cleaned without taking action")
	cmd.Flags().BoolVar(&smartMode, "smart", false, "Use AI (Haiku) to make orchestration decisions")

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
	autoRequeue          bool
	cleanupOrphans       bool
	dryRun               bool
	smartMode            bool
}

type leaderState struct {
	running   bool
	sessionID string
	startedAt time.Time
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

	// Workflow tracking
	allTasksDone    bool
	mergeCompleted  bool
	deployCompleted bool
	lastDoneCount   int // track Done count to detect new completions

	// Analysis tracking
	lastAnalysis time.Time // when we last ran the analyzer
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
	}
}

// checkWorkerHealth verifies all workers marked Active have existing tmux sessions
// with Claude actually running. Stale workers are reset and leader state is cleared.
// For development workers, completed tasks are moved to review.
func checkWorkerHealth(reg *worker.Registry, state *supervisorState, store *kanban.Store) {
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
			// For development workers with tasks, move to review before resetting
			taskID := w.CurrentTask
			if w.Role == worker.RoleWorker && taskID != "" && !state.completedTasks[taskID] {
				fmt.Printf("✅ Worker %s finished task %s (%s)\n", w.DisplayName(), taskID, reason)

				// Move task to review
				if err := store.Move(taskID, kanban.StatusReview); err != nil {
					fmt.Printf("   ⚠️  Error moving task to review: %v\n", err)
				} else {
					fmt.Printf("   📋 Moved task to Review\n")
					state.completedTasks[taskID] = true
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

			// Clear any leader state if this was a leader
			clearLeaderState(w.Role, state)

			// Clear task tracking state
			if taskID != "" {
				delete(state.taskWorkers, taskID)
			}
			delete(state.workerTasks, w.ID)
			delete(state.pokeCounts, w.ID)
		}
	}
}

// clearLeaderState clears the in-memory leader state for a given role.
func clearLeaderState(role worker.Role, state *supervisorState) {
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
	case worker.RoleDeploy:
		state.deploy.running = false
		state.deploy.sessionID = ""
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

func runSupervisor(ctx context.Context, cfg supervisorConfig) error {
	fmt.Printf("🎯 Supervisor starting\n")
	fmt.Printf("   Interval: %s\n", cfg.interval)
	fmt.Printf("   Work dir: %s\n", cfg.workDir)
	fmt.Printf("   Max workers: %d\n", cfg.maxConcurrentWorkers)
	fmt.Printf("   Auto-assign: %v\n", cfg.autoAssign)
	fmt.Printf("   Leaders: %v\n", cfg.withLeaders)
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

	// Load registry for startup initialization
	reg, err := worker.LoadRegistry()
	if err != nil {
		fmt.Printf("⚠️  Error loading workers for startup: %v\n", err)
	} else {
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

	// Initial run
	cycleFunc(cfg, state)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			cycleFunc(cfg, state)
		}
	}
}

func runCycle(cfg supervisorConfig, state *supervisorState) {
	now := time.Now().Format("15:04:05")
	fmt.Printf("\n━━━ Cycle %s ━━━\n", now)

	// Load kanban store
	store, err := getKanbanStoreForDir(cfg.workDir)
	if err != nil {
		fmt.Printf("⚠️  Error loading kanban: %v\n", err)
		return
	}
	defer store.Close()

	// Load worker registry
	reg, err := worker.LoadRegistry()
	if err != nil {
		fmt.Printf("⚠️  Error loading workers: %v\n", err)
		return
	}

	// 0. Health check - detect stale workers with missing tmux sessions
	checkWorkerHealth(reg, state, store)

	// 0.5. Recovery - check for misplaced tasks (done with unmerged branches)
	// This catches tasks that workers incorrectly moved to done
	recoverMisplacedTasks(store, state, cfg.workDir)

	// 0.6. Recovery - check for orphaned in-progress tasks (no active worker)
	recoverOrphanedTasks(store, reg, state)

	// 1. Check for completed workers and update tasks
	checkCompletedWorkers(store, reg, state, cfg.workDir)

	// 2. Analyze tasks - check for stuck/abandoned tasks that need requeuing
	analyzeAndRequeueTasks(store, reg, state, cfg)

	// 3. Check leader sessions
	if cfg.withLeaders {
		checkLeaderSessions(reg, state)
	}

	// 4. Poke active workers
	pokeActiveWorkers(reg, state, cfg.maxPokes)

	// 5. Assign idle workers to todo tasks
	if cfg.autoAssign {
		assignTasks(store, reg, state, cfg)
	}

	// 6. Run leader workflow if enabled
	if cfg.withLeaders {
		runLeaderWorkflow(store, reg, state, cfg.workDir)
	}

	// 7. Run task analyzer if needed (check for new tasks from completed work)
	if cfg.withLeaders {
		checkForNewTasks(store, reg, state, cfg.workDir, cfg)
	}

	// 8. Summary
	printSummary(store, reg, state)
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

// analyzeAndRequeueTasks checks for tasks that are stuck or abandoned and requeues them
func analyzeAndRequeueTasks(store *kanban.Store, reg *worker.Registry, state *supervisorState, cfg supervisorConfig) {
	if !cfg.autoRequeue {
		return
	}

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
					delete(state.taskStarted, task.ID)
					delete(state.taskWorkers, task.ID)
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
	startLeader(reg, analyzer, &state.analyzer, workDir, prompt, "ANALYZER_DONE")
}

func checkLeaderSessions(reg *worker.Registry, state *supervisorState) {
	// Check each leader type
	checkLeader := func(role worker.Role, ls *leaderState, name string) {
		if !ls.running {
			return
		}

		session := tmux.NewSession(ls.sessionID, "", "")
		if !session.Exists() {
			fmt.Printf("🏁 %s finished\n", name)

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
					worker.Stop(reg, w.ID)
					worker.Reset(reg, w.ID)
					fmt.Printf("   ♻️  Reset %s to idle\n", w.DisplayName())
					break
				}
			}
		}
	}

	checkLeader(worker.RolePlanner, &state.planner, "Planner")
	checkLeader(worker.RoleReviewer, &state.reviewer, "Reviewer")
	checkLeader(worker.RoleMerge, &state.merge, "Merge leader")
	checkLeader(worker.RoleDeploy, &state.deploy, "Deploy leader")
	checkLeader(worker.RoleGroomer, &state.groomer, "Groomer")

	// Analyzer uses planner role but different state
	if state.analyzer.running {
		session := tmux.NewSession(state.analyzer.sessionID, "", "")
		if !session.Exists() {
			fmt.Printf("🏁 Task analyzer finished\n")
			state.analyzer.running = false
			state.analyzer.sessionID = ""
		}
	}
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
	cmd := exec.Command("tmux", "capture-pane", "-t", session.Name, "-p", "-S", fmt.Sprintf("-%d", lines))
	out, err := cmd.Output()
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

	// Run haiku assessment (quick, cheap model for simple yes/no decisions)
	cmd := exec.Command("claude", "-p", prompt, "--model", "haiku")
	out, err := cmd.Output()
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

func assignTasks(store *kanban.Store, reg *worker.Registry, state *supervisorState, cfg supervisorConfig) {
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
			store.Move(task.ID, kanban.StatusTodo)
			continue
		}

		// Don't use main worktree for workers (would block other agents)
		if worktreePath == cfg.workDir {
			fmt.Printf("   ⏭️  Skipping task (would use shared worktree)\n")
			store.Move(task.ID, kanban.StatusTodo)
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
		go func(w *worker.Worker, task *kanban.Issue, prompt, promise, worktree string) {
			opts := worker.StartOptions{
				TaskID:   task.ID,
				Worktree: worktree,
				Prompt:   prompt,
				Promise:  promise,
			}

			ctx := context.Background()
			if err := worker.Start(ctx, reg, w.ID, opts); err != nil {
				fmt.Printf("⚠️  Error starting %s: %v\n", w.DisplayName(), err)
			}
		}(w, task, prompt, promise, worktreePath)

		assigned++
	}

	if assigned > 0 {
		fmt.Printf("📋 Assigned %d task(s)\n", assigned)
	}
}

func runLeaderWorkflow(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string) {
	board, err := store.GetBoard()
	if err != nil {
		return
	}

	counts := make(map[kanban.Status]int)
	for _, col := range board.Columns {
		counts[col.Status] = len(col.Issues)
	}

	// Check if any worker is using the worktree
	workerActive := false
	for _, w := range reg.List(worker.StatusActive) {
		if w.Role == worker.RoleWorker {
			workerActive = true
			break
		}
	}

	// 1. GROOMER: Run when there are items in backlog (can run alongside workers)
	// Groomer researches and details backlog items, moving ready ones to todo
	if counts[kanban.StatusBacklog] > 0 && !state.groomer.running {
		startGroomer(store, reg, state, workDir, board)
		// Don't return - groomer runs in parallel, continue checking other leaders
	}

	// 2. REVIEWER: Run when there are tasks in review (can run alongside workers)
	if counts[kanban.StatusReview] > 0 && !state.reviewer.running {
		startReviewer(store, reg, state, workDir, board)
		return // One leader at a time (after groomer which runs parallel)
	}

	// 3. PLANNER: Run when no work in progress and we need to plan
	needsPlanning := counts[kanban.StatusTodo] == 0 && counts[kanban.StatusInProgress] == 0 &&
		counts[kanban.StatusReview] == 0 && counts[kanban.StatusBacklog] > 0
	if needsPlanning && !state.planner.running && !workerActive {
		startPlanner(store, reg, state, workDir, board)
		return
	}

	// 4. Check if there are done tasks to merge
	doneCount := counts[kanban.StatusDone]
	hasDoneTasks := doneCount > 0
	pendingWork := counts[kanban.StatusBacklog] + counts[kanban.StatusTodo] +
		counts[kanban.StatusInProgress] + counts[kanban.StatusReview]
	state.allTasksDone = pendingWork == 0 && hasDoneTasks

	// Reset mergeCompleted if new tasks moved to Done (allows merge for new work)
	if doneCount > state.lastDoneCount && state.mergeCompleted {
		fmt.Printf("🔄 New tasks completed, resetting merge state\n")
		state.mergeCompleted = false
	}
	state.lastDoneCount = doneCount

	// 5. MERGE: Run when there are done tasks (can run alongside workers)
	// The merge leader will merge task branches for completed tasks
	// Don't restart if merge already completed (prevents infinite loop)
	if hasDoneTasks && !state.merge.running && !state.mergeCompleted {
		startMerge(store, reg, state, workDir, board)
		// Don't return - allow other leaders to run too
	}

	// 6. DEPLOY: Run after merge is completed (and all tasks done)
	if state.mergeCompleted && state.allTasksDone && !state.deployCompleted && !state.deploy.running {
		startDeploy(store, reg, state, workDir, board)
		return
	}
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

func startLeader(reg *worker.Registry, w *worker.Worker, ls *leaderState, workDir, prompt, promise string) {
	ls.running = true
	ls.sessionID = w.TmuxSessionName()
	ls.startedAt = time.Now()

	// Determine worktree for this leader
	// Merge and deploy need main worktree; others can use dedicated worktrees
	leaderWorkDir := workDir
	if w.Role == worker.RoleGroomer || w.Role == worker.RoleReviewer || w.Role == worker.RolePlanner {
		if wt, err := createLeaderWorktree(workDir, string(w.Role)); err == nil {
			leaderWorkDir = wt
			fmt.Printf("   📁 Using worktree: %s\n", wt)
		}
	}

	go func() {
		opts := worker.StartOptions{
			TaskID:   string(w.Role) + "-session",
			Worktree: leaderWorkDir,
			Prompt:   prompt,
			Promise:  promise,
		}

		ctx := context.Background()
		if err := worker.Start(ctx, reg, w.ID, opts); err != nil {
			fmt.Printf("⚠️  Error starting %s: %v\n", w.Role, err)
			ls.running = false
		}
	}()
}

func startReviewer(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RoleReviewer)
	if w == nil {
		fmt.Printf("⚠️  Review queue has items but no idle reviewer worker\n")
		fmt.Printf("   Create one with: foundry worker create --role reviewer\n")
		return
	}

	fmt.Printf("🔍 Starting reviewer %s\n", w.DisplayName())

	prompt := buildReviewerPrompt(board, workDir)
	startLeader(reg, w, &state.reviewer, workDir, prompt, "REVIEWER_DONE")
}

func startPlanner(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RolePlanner)
	if w == nil {
		fmt.Printf("⚠️  Backlog needs planning but no idle planner worker\n")
		fmt.Printf("   Create one with: foundry worker create --role planner\n")
		return
	}

	fmt.Printf("📋 Starting planner %s\n", w.DisplayName())

	prompt := buildPlannerPrompt(board, workDir)
	startLeader(reg, w, &state.planner, workDir, prompt, "PLANNER_DONE")
}

func startMerge(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RoleMerge)
	if w == nil {
		fmt.Printf("⚠️  All tasks done but no idle merge worker\n")
		fmt.Printf("   Create one with: foundry worker create --role merge\n")
		return
	}

	fmt.Printf("🔀 Starting merge leader %s\n", w.DisplayName())

	prompt := buildMergePrompt(board, workDir)
	startLeader(reg, w, &state.merge, workDir, prompt, "MERGE_DONE")
}

func startDeploy(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RoleDeploy)
	if w == nil {
		fmt.Printf("⚠️  Merge complete but no idle deploy worker\n")
		fmt.Printf("   Create one with: foundry worker create --role deploy\n")
		return
	}

	fmt.Printf("🚀 Starting deploy leader %s\n", w.DisplayName())

	prompt := buildDeployPrompt(board, workDir)
	startLeader(reg, w, &state.deploy, workDir, prompt, "DEPLOY_DONE")
}

func startGroomer(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RoleGroomer)
	if w == nil {
		fmt.Printf("⚠️  Backlog has items but no idle groomer worker\n")
		fmt.Printf("   Create one with: foundry worker create --role groomer\n")
		return
	}

	fmt.Printf("🧹 Starting backlog groomer %s\n", w.DisplayName())

	prompt := buildGroomerPrompt(board, workDir)
	startLeader(reg, w, &state.groomer, workDir, prompt, leader.PromiseGroomer)
}

// Prompt builders for each leader role

func buildReviewerPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a CODE REVIEWER for the Forge supervisor. Your job is to review completed work.\n\n")

	sb.WriteString("## Tasks in Review\n\n")
	reviewCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusReview {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
				if issue.Description != "" {
					sb.WriteString(fmt.Sprintf("  Description: %s\n", issue.Description))
				}
				reviewCount++
			}
		}
	}
	if reviewCount == 0 {
		sb.WriteString("(no tasks in review)\n")
	}

	sb.WriteString("\n## Review Checklist\n\n")
	sb.WriteString("For each task, verify:\n")
	sb.WriteString("- [ ] Code compiles/runs without errors\n")
	sb.WriteString("- [ ] Tests pass (if applicable)\n")
	sb.WriteString("- [ ] Code follows project conventions\n")
	sb.WriteString("- [ ] No obvious bugs or security issues\n")
	sb.WriteString("- [ ] Changes match the task description\n\n")

	sb.WriteString("## Parallel Workflow\n\n")
	sb.WriteString("Workers run in parallel, each in their own worktree with a task branch.\n")
	sb.WriteString("- Task branches follow pattern: `task/<task-id-first-8-chars>`\n")
	sb.WriteString("- List task branches: `git branch | grep task/`\n")
	sb.WriteString("- Review a branch: `git log main..task/<id>` and `git diff main..task/<id>`\n")
	sb.WriteString("- Workers may still be active - review completed work as it arrives\n\n")

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Check handoffs from workers: `search_memory_facts({ query: \"forge-handoff TO: reviewer\" })`\n")
	sb.WriteString("2. List task branches and match to review queue: `git branch | grep task/`\n")
	sb.WriteString("3. For each task in review:\n")
	sb.WriteString("   - Check branch: `git log main..task/<id> --oneline`\n")
	sb.WriteString("   - Review changes: `git diff main..task/<id>`\n")
	sb.WriteString("   - Run tests on the branch if needed\n")
	sb.WriteString("4. For approved tasks: `foundry kanban move <id> done`\n")
	sb.WriteString("5. For rejected tasks: `foundry kanban move <id> todo` and add feedback\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished reviewing all tasks, output: <promise>%s</promise>\n", leader.PromiseReviewer))

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

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Load planning context: `search_nodes({ query: \"project roadmap goals\" })`\n")
	sb.WriteString("2. Review backlog tasks and assess priorities\n")
	sb.WriteString("3. Move prioritized items to todo: `foundry kanban move <id> todo`\n")
	sb.WriteString("4. Add missing tasks: `foundry kanban add \"title\" -p high -s todo -d \"description\"`\n")
	sb.WriteString("5. Update priorities if needed: `foundry kanban edit <id> -p critical`\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished planning, output: <promise>%s</promise>\n", leader.PromisePlanner))

	return sb.String()
}

func buildMergePrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a MERGE COORDINATOR for the Forge supervisor. All tasks are complete and ready for merge.\n\n")

	sb.WriteString("## Completed Tasks\n\n")
	doneCount := 0
	for _, col := range board.Columns {
		if col.Status == kanban.StatusDone {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- %s: %s (branch: task/%s)\n", issue.ID[:8], issue.Title, issue.ID[:8]))
				doneCount++
			}
		}
	}
	if doneCount == 0 {
		sb.WriteString("(no completed tasks)\n")
	}

	sb.WriteString("\n## Task Branches\n\n")
	sb.WriteString("Each task was developed in its own branch (pattern: `task/<task-id>`).\n")
	sb.WriteString("- List all task branches: `git branch | grep task/`\n")
	sb.WriteString("- View branch changes: `git log main..task/<id> --oneline`\n\n")

	sb.WriteString("## Pre-Merge Checklist\n\n")
	sb.WriteString("- [ ] All tests pass: `make test`\n")
	sb.WriteString("- [ ] No linting errors: `make lint`\n")
	sb.WriteString("- [ ] Code is properly formatted: `make fmt`\n")
	sb.WriteString("- [ ] No merge conflicts between branches\n\n")

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Check handoffs: `search_memory_facts({ query: \"forge-handoff TO: merge\" })`\n")
	sb.WriteString("2. List task branches to merge: `git branch | grep task/`\n")
	sb.WriteString("3. For each task branch:\n")
	sb.WriteString("   - Merge to main: `git checkout main && git merge task/<id> --no-ff -m \"Merge task/<id>: <title>\"`\n")
	sb.WriteString("   - Resolve any conflicts\n")
	sb.WriteString("   - Delete branch after merge: `git branch -d task/<id>`\n")
	sb.WriteString("4. Run final tests on main: `make test`\n")
	sb.WriteString("5. Push to remote if appropriate: `git push origin main`\n\n")

	sb.WriteString(leader.SequentialThinkingTriggers)
	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished, output: <promise>%s</promise>\n", leader.PromiseMerge))

	return sb.String()
}

func buildDeployPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a DEPLOYMENT COORDINATOR for the Forge supervisor. Code is merged and ready for verification.\n\n")

	sb.WriteString("## Smoke Test Checklist\n\n")
	sb.WriteString("- [ ] Application starts successfully\n")
	sb.WriteString("- [ ] Core functionality works\n")
	sb.WriteString("- [ ] No obvious errors in console/logs\n")
	sb.WriteString("- [ ] Tests pass: `make test`\n\n")

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Check handoffs: `search_memory_facts({ query: \"forge-handoff TO: deploy\" })`\n")
	sb.WriteString("2. Verify build: `make build` or equivalent\n")
	sb.WriteString("3. Run smoke tests:\n")
	sb.WriteString("   - If web app: open in browser, check console for errors\n")
	sb.WriteString("   - If CLI: run basic commands\n")
	sb.WriteString("   - If library: run test suite\n")
	sb.WriteString("4. Document what was built in a summary\n")
	sb.WriteString("5. Tag the release if applicable: `git tag -a v<version> -m \"Release\"`\n\n")

	sb.WriteString(leader.SequentialThinkingTriggers)
	sb.WriteString(leader.ErrorHandlingGuidance)
	sb.WriteString(fmt.Sprintf("\nWorking directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished, output: <promise>%s</promise>\n", leader.PromiseDeploy))

	return sb.String()
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
	sb.WriteString("- Do NOT run `foundry kanban move <id> done` - this breaks the workflow\n\n")

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
	for _, col := range board.Columns {
		if col.Status == kanban.StatusBacklog {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
				if issue.Description != "" {
					sb.WriteString(fmt.Sprintf("  Description: %s\n", issue.Description))
				} else {
					sb.WriteString("  Description: (none - NEEDS DETAIL)\n")
				}
				backlogCount++
			}
		}
	}
	if backlogCount == 0 {
		sb.WriteString("(no items in backlog)\n")
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

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. **Research each backlog item**:\n")
	sb.WriteString("   - Understand the codebase context: `Grep` and `Read` relevant files\n")
	sb.WriteString("   - Check for existing patterns: How is similar functionality implemented?\n")
	sb.WriteString("   - Identify dependencies: What other code/tasks does this depend on?\n")
	sb.WriteString("   - Check Graphiti for context: `search_nodes({ query: \"<item title>\" })`\n\n")

	sb.WriteString("2. **Update item descriptions** with your research:\n")
	sb.WriteString("   ```bash\n")
	sb.WriteString("   foundry kanban edit <id> -d \"<detailed description>\"\n")
	sb.WriteString("   ```\n\n")

	sb.WriteString("3. **Break down large items** if needed:\n")
	sb.WriteString("   - Create sub-tasks: `foundry kanban add \"<subtask>\" -p medium -s backlog -d \"<description>\"`\n")
	sb.WriteString("   - Reference parent: Include \"Part of: <parent-id>\" in description\n\n")

	sb.WriteString("4. **Move ready items to todo**:\n")
	sb.WriteString("   ```bash\n")
	sb.WriteString("   foundry kanban move <id> todo\n")
	sb.WriteString("   ```\n\n")

	sb.WriteString("5. **Prioritize strategically**:\n")
	sb.WriteString("   - `critical`: Blocking other work, must do immediately\n")
	sb.WriteString("   - `high`: Important for current goals\n")
	sb.WriteString("   - `medium`: Should do soon\n")
	sb.WriteString("   - `low`: Nice to have\n")
	sb.WriteString("   - Update: `foundry kanban edit <id> -p <priority>`\n\n")

	sb.WriteString("## Parallel Workflow\n\n")
	sb.WriteString("You run in PARALLEL with workers - they may be implementing tasks while you groom.\n")
	sb.WriteString("- Focus on items WITHOUT active workers\n")
	sb.WriteString("- Don't move items that are already being worked on\n")
	sb.WriteString("- Coordinate via Graphiti if needed\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString(leader.OutputFormat)
	sb.WriteString(fmt.Sprintf("\nWhen finished grooming, output: <promise>%s</promise>\n", leader.PromiseGroomer))

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
	sb.WriteString("   - Move back to todo: `foundry kanban move <id> todo`\n")
	sb.WriteString("   - Add note explaining why: `foundry kanban edit <id> -d \"Requeued: <reason>\"`\n\n")

	sb.WriteString("5. **Create new tasks if needed**:\n")
	sb.WriteString("   - Follow-up work: `foundry kanban add \"title\" -d \"description\" -p medium -s todo`\n")
	sb.WriteString("   - Bugs found: `foundry kanban add \"Fix: issue\" -p high -s todo`\n")
	sb.WriteString("   - Refactoring: `foundry kanban add \"Refactor: area\" -p low -s backlog`\n\n")

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

	sb.WriteString(fmt.Sprintf("When finished, output: <promise>%s</promise>\n", leader.PromiseAnalyzer))

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

	fmt.Printf("\n📊 Tasks: %d backlog, %d todo, %d in-progress, %d review, %d done\n",
		counts[kanban.StatusBacklog],
		counts[kanban.StatusTodo],
		counts[kanban.StatusInProgress],
		counts[kanban.StatusReview],
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
	if state.groomer.running {
		activeLeaders = append(activeLeaders, "🧹groomer")
	}

	if len(activeLeaders) > 0 {
		fmt.Printf("👔 Leaders running: %s\n", strings.Join(activeLeaders, ", "))
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
		pullCmd.Run() // Ignore errors, best effort
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
func runSmartCycle(cfg supervisorConfig, state *supervisorState) {
	now := time.Now().Format("15:04:05")
	fmt.Printf("\n━━━ Smart Cycle %s ━━━\n", now)

	// Load kanban store
	store, err := getKanbanStoreForDir(cfg.workDir)
	if err != nil {
		fmt.Printf("⚠️  Error loading kanban: %v\n", err)
		return
	}
	defer store.Close()

	// Load worker registry
	reg, err := worker.LoadRegistry()
	if err != nil {
		fmt.Printf("⚠️  Error loading workers: %v\n", err)
		return
	}

	// Health check - detect stale workers (always do this, not AI-controlled)
	checkWorkerHealth(reg, state, store)

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

	// Call Haiku
	cmd := exec.Command("claude", "-p", prompt, "--model", "haiku")
	out, err := cmd.Output()
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
		startReviewer(store, reg, state, cfg.workDir, board)

	case "start_groomer":
		if !cfg.withLeaders || state.groomer.running {
			return
		}
		board, _ := store.GetBoard()
		startGroomer(store, reg, state, cfg.workDir, board)

	case "start_planner":
		if !cfg.withLeaders || state.planner.running {
			return
		}
		board, _ := store.GetBoard()
		startPlanner(store, reg, state, cfg.workDir, board)

	case "start_merge":
		if !cfg.withLeaders || state.merge.running {
			return
		}
		board, _ := store.GetBoard()
		startMerge(store, reg, state, cfg.workDir, board)

	case "start_deploy":
		if !cfg.withLeaders || state.deploy.running {
			return
		}
		board, _ := store.GetBoard()
		startDeploy(store, reg, state, cfg.workDir, board)

	default:
		fmt.Printf("   ⚠️  Unknown action: %s\n", action.Action)
	}
}

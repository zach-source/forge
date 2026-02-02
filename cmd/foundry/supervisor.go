package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
		interval        time.Duration
		analyzeInterval time.Duration
		stuckThreshold  time.Duration
		maxPokes        int
		workDir         string
		autoAssign      bool
		withLeaders     bool
		autoRequeue     bool
		cleanupOrphans  bool
		dryRun          bool
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
				interval:        interval,
				analyzeInterval: analyzeInterval,
				stuckThreshold:  stuckThreshold,
				maxPokes:        maxPokes,
				workDir:         workDir,
				autoAssign:      autoAssign,
				withLeaders:     withLeaders,
				autoRequeue:     autoRequeue,
				cleanupOrphans:  cleanupOrphans,
				dryRun:          dryRun,
			})
		},
	}

	cmd.Flags().DurationVarP(&interval, "interval", "i", 2*time.Minute, "Time between checks")
	cmd.Flags().DurationVar(&analyzeInterval, "analyze-interval", 10*time.Minute, "Time between task analysis runs")
	cmd.Flags().DurationVar(&stuckThreshold, "stuck", 10*time.Minute, "Requeue tasks stuck longer than this")
	cmd.Flags().IntVar(&maxPokes, "max-pokes", 10, "Max pokes per worker before escalating (0 = unlimited)")
	cmd.Flags().StringVarP(&workDir, "dir", "d", "", "Working directory (default: current)")
	cmd.Flags().BoolVar(&autoAssign, "auto-assign", true, "Automatically assign tasks to idle workers")
	cmd.Flags().BoolVar(&withLeaders, "leaders", false, "Enable all leader agents (planner, reviewer, merge, deploy)")
	cmd.Flags().BoolVar(&autoRequeue, "auto-requeue", true, "Automatically requeue stuck tasks")
	cmd.Flags().BoolVar(&cleanupOrphans, "cleanup-orphans", false, "Clean up orphaned tmux sessions on startup")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be cleaned without taking action")

	return cmd
}

type supervisorConfig struct {
	interval        time.Duration
	analyzeInterval time.Duration
	stuckThreshold  time.Duration
	maxPokes        int
	workDir         string
	autoAssign      bool
	withLeaders     bool
	autoRequeue     bool
	cleanupOrphans  bool
	dryRun          bool
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
	taskStarted map[string]time.Time // task ID -> when it entered in_progress

	// Leader states
	planner  leaderState
	reviewer leaderState
	merge    leaderState
	deploy   leaderState
	analyzer leaderState // for task analysis

	// Workflow tracking
	allTasksDone    bool
	mergeCompleted  bool
	deployCompleted bool

	// Analysis tracking
	lastAnalysis    time.Time // when we last ran the analyzer
	analysisResults []string  // recommendations from last analysis
}

func newSupervisorState() *supervisorState {
	return &supervisorState{
		pokeCounts:     make(map[string]int),
		taskWorkers:    make(map[string]string),
		workerTasks:    make(map[string]string),
		completedTasks: make(map[string]bool),
		taskStarted:    make(map[string]time.Time),
	}
}

// checkWorkerHealth verifies all workers marked Active have existing tmux sessions.
// Stale workers (session gone) are reset and any associated leader state is cleared.
func checkWorkerHealth(reg *worker.Registry, state *supervisorState) {
	workers := reg.List(worker.StatusActive)

	for _, w := range workers {
		if w.SessionID == "" {
			continue
		}

		session := tmux.NewSession(w.SessionID, "", "")
		if !session.Exists() {
			fmt.Printf("🔄 Resetting stale worker %s (session gone)\n", w.DisplayName())

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
			if w.CurrentTask != "" {
				delete(state.taskWorkers, w.CurrentTask)
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
		}
	}
}

// cleanupOrphanedSessions finds tmux sessions with forge- prefix not in registry.
func cleanupOrphanedSessions(reg *worker.Registry, dryRun bool) {
	// Get all forge-related tmux sessions
	sessions, err := tmux.ListSessions("forge-")
	if err != nil {
		fmt.Printf("⚠️  Error listing tmux sessions: %v\n", err)
		return
	}

	if len(sessions) == 0 {
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
	for _, sessionID := range sessions {
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

	// Initial run
	runCycle(cfg, state)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			runCycle(cfg, state)
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
	checkWorkerHealth(reg, state)

	// 1. Check for completed workers and update tasks
	checkCompletedWorkers(store, reg, state)

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
		assignTasks(store, reg, state, cfg.workDir)
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

func checkCompletedWorkers(store *kanban.Store, reg *worker.Registry, state *supervisorState) {
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
			if taskID != "" && !state.completedTasks[taskID] {
				fmt.Printf("✅ Worker %s finished task %s\n", w.DisplayName(), taskID)

				// Move task to review (not done - let reviewer check it)
				if err := store.Move(taskID, kanban.StatusReview); err != nil {
					fmt.Printf("   ⚠️  Error moving task to review: %v\n", err)
				} else {
					fmt.Printf("   📋 Moved task to Review\n")
				}

				state.completedTasks[taskID] = true
				delete(state.taskWorkers, taskID)
				delete(state.workerTasks, w.ID)
				delete(state.pokeCounts, w.ID)
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

	activeWorkers := reg.List(worker.StatusActive)
	activeWorkerIDs := make(map[string]bool)
	for _, w := range activeWorkers {
		activeWorkerIDs[w.ID] = true
	}

	for _, task := range inProgressTasks {
		// Track when we first saw this task in progress
		if _, tracked := state.taskStarted[task.ID]; !tracked {
			state.taskStarted[task.ID] = time.Now()
		}

		// Check if this task has an active worker
		assignedWorker := state.taskWorkers[task.ID]
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
			ls.running = false
			ls.sessionID = ""

			// Find and reset the leader worker
			workers := reg.List()
			for _, w := range workers {
				if w.Role == role && w.SessionID == ls.sessionID {
					worker.Stop(reg, w.ID)
					worker.Reset(reg, w.ID)
					break
				}
			}
		}
	}

	checkLeader(worker.RolePlanner, &state.planner, "Planner")
	checkLeader(worker.RoleReviewer, &state.reviewer, "Reviewer")
	checkLeader(worker.RoleMerge, &state.merge, "Merge leader")
	checkLeader(worker.RoleDeploy, &state.deploy, "Deploy leader")

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

		// Check max pokes
		if maxPokes > 0 && state.pokeCounts[w.ID] >= maxPokes {
			fmt.Printf("⏭️  %s: max pokes reached (%d)\n", w.DisplayName(), maxPokes)
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

		fmt.Printf("📣 %s %s: poked (#%d)\n", w.RoleIcon(), w.DisplayName(), count)
	}
}

func assignTasks(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string) {
	// Check if any worker is already active (avoid worktree conflicts)
	activeWorkers := reg.List(worker.StatusActive)
	for _, w := range activeWorkers {
		if w.Role == worker.RoleWorker {
			// A worker is active, wait for it
			return
		}
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
		fmt.Printf("💤 %d tasks waiting, no idle workers\n", len(todoTasks))
		return
	}

	// Assign just one task (first idle worker, first todo task)
	task := todoTasks[0]
	w := idleWorkers[0]

	// Skip if already being worked on
	if state.taskWorkers[task.ID] != "" {
		return
	}

	// Start the worker on this task
	fmt.Printf("🚀 Assigning %s to worker %s\n", task.ID, w.DisplayName())
	fmt.Printf("   Task: %s\n", task.Title)

	// Move task to in_progress
	if err := store.Move(task.ID, kanban.StatusInProgress); err != nil {
		fmt.Printf("   ⚠️  Error moving task: %v\n", err)
		return
	}

	// Build prompt from task
	prompt := buildTaskPrompt(task)
	promise := fmt.Sprintf("TASK_%s_DONE", task.ID[:8])

	// Track assignment before starting
	state.taskWorkers[task.ID] = w.ID
	state.workerTasks[w.ID] = task.ID

	// Start worker (in goroutine to not block)
	go func(w *worker.Worker, task *kanban.Issue, prompt, promise string) {
		opts := worker.StartOptions{
			TaskID:   task.ID,
			Worktree: workDir,
			Prompt:   prompt,
			Promise:  promise,
		}

		ctx := context.Background()
		if err := worker.Start(ctx, reg, w.ID, opts); err != nil {
			fmt.Printf("⚠️  Error starting %s: %v\n", w.DisplayName(), err)
		}
	}(w, task, prompt, promise)

	fmt.Printf("📋 Assigned 1 task\n")
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

	// 1. REVIEWER: Run when there are tasks in review
	if counts[kanban.StatusReview] > 0 && !state.reviewer.running && !workerActive {
		startReviewer(store, reg, state, workDir, board)
		return // One leader at a time
	}

	// 2. PLANNER: Run when no work in progress and we need to plan
	needsPlanning := counts[kanban.StatusTodo] == 0 && counts[kanban.StatusInProgress] == 0 &&
		counts[kanban.StatusReview] == 0 && counts[kanban.StatusBacklog] > 0
	if needsPlanning && !state.planner.running && !workerActive {
		startPlanner(store, reg, state, workDir, board)
		return
	}

	// 3. Check if all tasks are done
	pendingWork := counts[kanban.StatusBacklog] + counts[kanban.StatusTodo] +
		counts[kanban.StatusInProgress] + counts[kanban.StatusReview]
	state.allTasksDone = pendingWork == 0 && counts[kanban.StatusDone] > 0

	// 4. MERGE: Run when all tasks done and merge not completed
	if state.allTasksDone && !state.mergeCompleted && !state.merge.running && !workerActive {
		startMerge(store, reg, state, workDir, board)
		return
	}

	// 5. DEPLOY: Run after merge is completed
	if state.mergeCompleted && !state.deployCompleted && !state.deploy.running && !workerActive {
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

	go func() {
		opts := worker.StartOptions{
			TaskID:   string(w.Role) + "-session",
			Worktree: workDir,
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
		return
	}

	fmt.Printf("🔍 Starting reviewer %s\n", w.DisplayName())

	prompt := buildReviewerPrompt(board, workDir)
	startLeader(reg, w, &state.reviewer, workDir, prompt, "REVIEWER_DONE")
}

func startPlanner(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RolePlanner)
	if w == nil {
		return
	}

	fmt.Printf("📋 Starting planner %s\n", w.DisplayName())

	prompt := buildPlannerPrompt(board, workDir)
	startLeader(reg, w, &state.planner, workDir, prompt, "PLANNER_DONE")
}

func startMerge(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RoleMerge)
	if w == nil {
		return
	}

	fmt.Printf("🔀 Starting merge leader %s\n", w.DisplayName())

	prompt := buildMergePrompt(board, workDir)
	startLeader(reg, w, &state.merge, workDir, prompt, "MERGE_DONE")
}

func startDeploy(store *kanban.Store, reg *worker.Registry, state *supervisorState, workDir string, board *kanban.Board) {
	w := findIdleLeader(reg, worker.RoleDeploy)
	if w == nil {
		return
	}

	fmt.Printf("🚀 Starting deploy leader %s\n", w.DisplayName())

	prompt := buildDeployPrompt(board, workDir)
	startLeader(reg, w, &state.deploy, workDir, prompt, "DEPLOY_DONE")
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

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Check handoffs from workers: `search_memory_facts({ query: \"forge-handoff TO: reviewer\" })`\n")
	sb.WriteString("2. Review files changed for each task\n")
	sb.WriteString("3. Run `git diff` and `git status` to see changes\n")
	sb.WriteString("4. Run tests if available: `make test` or equivalent\n")
	sb.WriteString("5. For approved tasks: `foundry kanban move <id> done`\n")
	sb.WriteString("6. For rejected tasks: `foundry kanban move <id> todo` and create issue with feedback\n\n")

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
				sb.WriteString(fmt.Sprintf("- %s: %s\n", issue.ID[:8], issue.Title))
				doneCount++
			}
		}
	}
	if doneCount == 0 {
		sb.WriteString("(no completed tasks)\n")
	}

	sb.WriteString("\n## Pre-Merge Checklist\n\n")
	sb.WriteString("- [ ] All tests pass: `make test`\n")
	sb.WriteString("- [ ] No linting errors: `make lint`\n")
	sb.WriteString("- [ ] Code is properly formatted: `make fmt`\n")
	sb.WriteString("- [ ] No merge conflicts\n\n")

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Check handoffs: `search_memory_facts({ query: \"forge-handoff TO: merge\" })`\n")
	sb.WriteString("2. Review all changes: `git status` and `git diff`\n")
	sb.WriteString("3. Run tests to verify everything works\n")
	sb.WriteString("4. Stage and commit: `git add . && git commit -m \"...\"`\n")
	sb.WriteString("5. Verify the commit looks correct: `git log -1 --stat`\n\n")

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
	prompt := task.Title

	if task.Description != "" {
		prompt += "\n\n" + task.Description
	}

	if task.Priority == kanban.PriorityCritical || task.Priority == kanban.PriorityHigh {
		prompt += "\n\nThis is a high priority task - please focus on completing it efficiently."
	}

	return prompt
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
	workerCounts := make(map[string]int)
	for _, w := range workers {
		key := fmt.Sprintf("%s-%s", w.Role, w.Status)
		workerCounts[key]++
	}

	fmt.Printf("\n📊 Tasks: %d backlog, %d todo, %d in-progress, %d review, %d done\n",
		counts[kanban.StatusBacklog],
		counts[kanban.StatusTodo],
		counts[kanban.StatusInProgress],
		counts[kanban.StatusReview],
		counts[kanban.StatusDone],
	)

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

	if len(activeLeaders) > 0 {
		fmt.Printf("👔 Leaders: %s\n", strings.Join(activeLeaders, ", "))
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
	urgency := "How's it going?"
	if pokeCount >= 3 {
		urgency = "Please wrap up soon."
	}
	if pokeCount >= 5 {
		urgency = "Time to finish - output your completion promise now."
	}
	if pokeCount >= 8 {
		urgency = "URGENT: Please complete immediately and output your promise."
	}

	return fmt.Sprintf(
		"echo '🔔 Supervisor (#%d): %s'",
		pokeCount,
		urgency,
	)
}

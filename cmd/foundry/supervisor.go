package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/tmux"
	"github.com/zach-source/forge/internal/worker"
)

func newSupervisorCmd() *cobra.Command {
	var (
		interval    time.Duration
		maxPokes    int
		workDir     string
		autoAssign  bool
		withLeaders bool
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

Workflow: todo -> in_progress (worker) -> review (reviewer) -> done

Examples:
  foundry supervisor                    # Default 2 minute interval
  foundry supervisor --interval 1m      # Check every minute
  foundry supervisor --leaders          # Enable all leader agents
  foundry supervisor --no-auto-assign   # Only monitor, don't start new tasks`,
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
				interval:    interval,
				maxPokes:    maxPokes,
				workDir:     workDir,
				autoAssign:  autoAssign,
				withLeaders: withLeaders,
			})
		},
	}

	cmd.Flags().DurationVarP(&interval, "interval", "i", 2*time.Minute, "Time between checks")
	cmd.Flags().IntVar(&maxPokes, "max-pokes", 10, "Max pokes per worker before escalating (0 = unlimited)")
	cmd.Flags().StringVarP(&workDir, "dir", "d", "", "Working directory (default: current)")
	cmd.Flags().BoolVar(&autoAssign, "auto-assign", true, "Automatically assign tasks to idle workers")
	cmd.Flags().BoolVar(&withLeaders, "leaders", false, "Enable all leader agents (planner, reviewer, merge, deploy)")

	return cmd
}

type supervisorConfig struct {
	interval    time.Duration
	maxPokes    int
	workDir     string
	autoAssign  bool
	withLeaders bool
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

	// Leader states
	planner  leaderState
	reviewer leaderState
	merge    leaderState
	deploy   leaderState

	// Workflow tracking
	allTasksDone    bool
	mergeCompleted  bool
	deployCompleted bool
}

func newSupervisorState() *supervisorState {
	return &supervisorState{
		pokeCounts:     make(map[string]int),
		taskWorkers:    make(map[string]string),
		workerTasks:    make(map[string]string),
		completedTasks: make(map[string]bool),
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

	// 1. Check for completed workers and update tasks
	checkCompletedWorkers(store, reg, state)

	// 2. Check leader sessions
	if cfg.withLeaders {
		checkLeaderSessions(reg, state)
	}

	// 3. Poke active workers
	pokeActiveWorkers(reg, state, cfg.maxPokes)

	// 4. Assign idle workers to todo tasks
	if cfg.autoAssign {
		assignTasks(store, reg, state, cfg.workDir)
	}

	// 5. Run leader workflow if enabled
	if cfg.withLeaders {
		runLeaderWorkflow(store, reg, state, cfg.workDir)
	}

	// 6. Summary
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

	sb.WriteString("You are a CODE REVIEWER. Your job is to review completed work.\n\n")
	sb.WriteString("## Tasks in Review\n\n")

	for _, col := range board.Columns {
		if col.Status == kanban.StatusReview {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
			}
		}
	}

	sb.WriteString("\n## Your Tasks\n\n")
	sb.WriteString("1. Check the files created for each task in review\n")
	sb.WriteString("2. Verify code quality, completeness, and correctness\n")
	sb.WriteString("3. If task is good: `foundry kanban move <id> done`\n")
	sb.WriteString("4. If task needs work: `foundry kanban move <id> todo` and add a note\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString("When finished reviewing all tasks, output: <promise>REVIEWER_DONE</promise>\n")

	return sb.String()
}

func buildPlannerPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a PROJECT PLANNER. Your job is to plan and prioritize work.\n\n")
	sb.WriteString("## Current Board State\n\n")

	for _, col := range board.Columns {
		sb.WriteString(fmt.Sprintf("### %s (%d)\n", col.Status, len(col.Issues)))
		for _, issue := range col.Issues {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Priority, issue.ID[:8], issue.Title))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Review backlog tasks and move important ones to todo\n")
	sb.WriteString("2. Add any missing tasks needed for the project\n")
	sb.WriteString("3. Prioritize: `foundry kanban move <id> todo`\n")
	sb.WriteString("4. Add tasks: `foundry kanban add \"title\" -p high -s todo`\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString("When finished planning, output: <promise>PLANNER_DONE</promise>\n")

	return sb.String()
}

func buildMergePrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a MERGE COORDINATOR. All tasks are complete.\n\n")
	sb.WriteString("## Completed Tasks\n\n")

	for _, col := range board.Columns {
		if col.Status == kanban.StatusDone {
			for _, issue := range col.Issues {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", issue.ID[:8], issue.Title))
			}
		}
	}

	sb.WriteString("\n## Your Tasks\n\n")
	sb.WriteString("1. Review all changes with `git status` and `git diff`\n")
	sb.WriteString("2. Create a commit with all changes: `git add . && git commit -m \"...\"`\n")
	sb.WriteString("3. Verify the commit looks correct\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString("When finished, output: <promise>MERGE_DONE</promise>\n")

	return sb.String()
}

func buildDeployPrompt(board *kanban.Board, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a DEPLOYMENT COORDINATOR. Code is merged and ready.\n\n")
	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. Verify the application works: open index.html in browser or start a server\n")
	sb.WriteString("2. Run any tests if they exist\n")
	sb.WriteString("3. Document what was built in a brief summary\n\n")

	sb.WriteString(fmt.Sprintf("Working directory: %s\n\n", workDir))
	sb.WriteString("When finished, output: <promise>DEPLOY_DONE</promise>\n")

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
	foundryDir := filepath.Join(dir, ".foundry")
	if err := os.MkdirAll(foundryDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating .foundry directory: %w", err)
	}

	dbPath := filepath.Join(foundryDir, "kanban.db")
	return kanban.NewStore(dbPath)
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

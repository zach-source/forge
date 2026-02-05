package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/tmux"
	"github.com/zach-source/forge/internal/worker"
)

func newShutdownCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown all forge sessions and foundry workers",
		Long: `Stop all running forge sessions, foundry workers, and related tmux sessions.

This command gracefully stops:
- All active forge sessions
- All foundry workers (alpha, bravo, etc.)
- Leader sessions (planner, reviewer, merge, deploy)
- Board sync sessions (GitHub, Notion)
- Any orphaned tmux sessions

Examples:
  foundry shutdown           # Graceful shutdown
  foundry shutdown --force   # Force kill all sessions`,
		Aliases: []string{"stop-all", "killall"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShutdown(force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force kill without confirmation")

	return cmd
}

func runShutdown(force bool) error {
	fmt.Println("🛑 Shutting down all agents...")
	fmt.Println()

	var stopped, failed int

	// 1. Stop all foundry workers
	fmt.Println("📋 Stopping foundry workers...")
	reg, err := worker.LoadRegistry()
	if err == nil {
		activeWorkers := reg.List(worker.StatusActive)
		for _, w := range activeWorkers {
			if err := worker.Stop(reg, w.ID); err != nil {
				fmt.Printf("   ⚠️  Failed to stop %s: %v\n", w.DisplayName(), err)
				failed++
			} else {
				fmt.Printf("   ✅ Stopped worker: %s\n", w.DisplayName())
				stopped++
			}
		}
		if len(activeWorkers) == 0 {
			fmt.Println("   (no active workers)")
		}
	} else {
		fmt.Printf("   ⚠️  Could not load worker registry: %v\n", err)
	}

	// 2. Cancel all forge sessions
	fmt.Println()
	fmt.Println("🔥 Cancelling forge sessions...")
	forgeList, err := exec.Command("forge", "list", "--json").Output()
	if err == nil && len(forgeList) > 0 {
		// Parse and cancel each session
		lines := strings.Split(string(forgeList), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "SESSION") || strings.HasPrefix(line, "No ") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				sessionID := fields[0]
				cancelCmd := exec.Command("sh", "-c", fmt.Sprintf("echo 'y' | forge cancel %s", sessionID))
				if err := cancelCmd.Run(); err != nil {
					fmt.Printf("   ⚠️  Failed to cancel %s: %v\n", sessionID, err)
					failed++
				} else {
					fmt.Printf("   ✅ Cancelled session: %s\n", sessionID)
					stopped++
				}
			}
		}
	} else {
		fmt.Println("   (no active forge sessions)")
	}

	// 3. Kill orphaned tmux sessions (in forge tmux server)
	fmt.Println()
	fmt.Println("🧹 Cleaning up tmux sessions...")
	allSessions, err := tmux.ListSessions("")
	if err == nil {
		prefixes := []string{"forge-", "foundry-", "worker-", "you-are-", "github-sync"}
		for _, session := range allSessions {
			session = strings.TrimSpace(session)
			if session == "" {
				continue
			}
			for _, prefix := range prefixes {
				if strings.HasPrefix(session, prefix) {
					s := tmux.NewSession(session, "", "")
					if s.Exists() {
						s.Kill()
						fmt.Printf("   ✅ Killed tmux: %s\n", session)
						stopped++
					}
					break
				}
			}
		}
	}

	// 4. Summary
	fmt.Println()
	fmt.Println("━━━ Summary ━━━")
	fmt.Printf("✅ Stopped: %d\n", stopped)
	if failed > 0 {
		fmt.Printf("⚠️  Failed: %d\n", failed)
	}

	// 5. Verification
	fmt.Println()
	fmt.Println("📊 Verification:")

	// Check forge sessions
	forgeCheck, _ := exec.Command("forge", "list").Output()
	if strings.Contains(string(forgeCheck), "No active") {
		fmt.Println("   Forge sessions: ✅ all stopped")
	} else {
		fmt.Printf("   Forge sessions: ⚠️  some may still be running\n")
	}

	// Check workers
	if reg != nil {
		remaining := reg.List(worker.StatusActive)
		if len(remaining) == 0 {
			fmt.Println("   Foundry workers: ✅ all stopped")
		} else {
			fmt.Printf("   Foundry workers: ⚠️  %d still active\n", len(remaining))
		}
	}

	// Check tmux
	tmuxCheck, _ := exec.Command("sh", "-c", "tmux list-sessions 2>/dev/null | grep -E '(forge|foundry|worker|you-are)' | wc -l").Output()
	count := strings.TrimSpace(string(tmuxCheck))
	if count == "0" || count == "" {
		fmt.Println("   Tmux sessions: ✅ clean")
	} else {
		fmt.Printf("   Tmux sessions: ⚠️  %s remaining\n", count)
	}

	fmt.Println()
	if failed == 0 {
		fmt.Println("🎉 All agents shut down successfully!")
	} else {
		fmt.Println("⚠️  Some agents may still be running. Use --force to kill them.")
	}

	return nil
}

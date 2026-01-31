package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/session"
)

func newLogCmd() *cobra.Command {
	var (
		follow bool
		lines  int
	)

	cmd := &cobra.Command{
		Use:   "log [session-id]",
		Short: "View agent log output",
		Long: `View the log output from a forge agent session.

Example:
  forge log
  forge log --follow
  forge log forge-api-12345678 -n 50`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := session.NewManager()
			if err := mgr.Discover(); err != nil {
				return fmt.Errorf("discovering sessions: %w", err)
			}

			// Also check local directory
			wd, _ := os.Getwd()
			mgr.DiscoverLocal(wd)

			var sessionID string

			if len(args) > 0 {
				sessionID = args[0]
			} else {
				// Find active sessions
				sessions := mgr.List()
				active := make([]*session.Session, 0)
				for _, s := range sessions {
					if s.Status == session.StatusActive {
						active = append(active, s)
					}
				}

				if len(active) == 0 {
					fmt.Println("No active forge sessions found.")
					return nil
				}

				if len(active) == 1 {
					sessionID = active[0].ID
				} else {
					fmt.Println("Multiple active sessions. Specify which to view:")
					for _, s := range active {
						fmt.Printf("  %s - %s\n", s.ID, s.State.CompletionPromise)
					}
					return nil
				}
			}

			s := mgr.Get(sessionID)
			if s == nil {
				return fmt.Errorf("session %q not found", sessionID)
			}

			if s.LogFile == "" && s.State != nil {
				s.LogFile = s.State.LogFile
			}

			if s.LogFile == "" {
				// Fall back to tmux capture
				output, err := mgr.CaptureOutput(sessionID, lines)
				if err != nil {
					return fmt.Errorf("capturing output: %w", err)
				}
				for _, line := range output {
					fmt.Println(line)
				}
				return nil
			}

			// Read from log file
			if follow {
				return tailFollow(s.LogFile)
			}

			return tailLines(s.LogFile, lines)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVarP(&lines, "lines", "n", 20, "Number of lines to show")

	return cmd
}

func tailLines(path string, n int) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Log file not found.")
			return nil
		}
		return err
	}
	defer f.Close()

	// Read all lines and keep last n
	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	start := 0
	if len(allLines) > n {
		start = len(allLines) - n
	}

	for i := start; i < len(allLines); i++ {
		fmt.Println(allLines[i])
	}

	return scanner.Err()
}

func tailFollow(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Log file not found. Waiting...")
			// Wait for file to appear
			for {
				time.Sleep(1 * time.Second)
				f, err = os.Open(path)
				if err == nil {
					break
				}
			}
		} else {
			return err
		}
	}
	defer f.Close()

	// Seek to end
	f.Seek(0, io.SeekEnd)

	fmt.Printf("Following %s (Ctrl+C to stop)...\n\n", path)

	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if err != nil && err != io.EOF {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/satococoa/wtp/v2/internal/config"
	"github.com/satococoa/wtp/v2/internal/errors"
	"github.com/satococoa/wtp/v2/internal/git"
)

// NewCdCommand creates the cd command definition
func NewCdCommand() *cli.Command {
	return &cli.Command{
		Name:  "cd",
		Usage: "Output absolute path to worktree",
		Description: "Output the absolute path to the specified worktree.\n" +
			"If no worktree is specified, outputs the main worktree path (like cd goes to $HOME).\n\n" +
			"Usage:\n" +
			"  Direct:     cd \"$(jtp cd feature)\"\n" +
			"  With hook:  jtp cd feature\n" +
			"  Go home:    jtp cd\n\n" +
			"To enable the hook for easier navigation:\n" +
			"  Bash: eval \"$(jtp hook bash)\"\n" +
			"  Zsh:  eval \"$(jtp hook zsh)\"\n" +
			"  Fish: jtp hook fish | source",
		ArgsUsage:     "[worktree-name]",
		Action:        cdToWorktree,
		ShellComplete: completeWorktreesForCd,
	}
}

func cdToWorktree(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args()

	// Default to main worktree (@) when no argument provided, like cd goes to $HOME
	worktreeName := "@"
	if args.Len() > 0 {
		worktreeName = args.Get(0)
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return errors.DirectoryAccessFailed("access current", ".", err)
	}

	// Initialize repository to check if we're in a git repo
	repo, err := git.NewRepository(cwd)
	if err != nil {
		return errors.NotInGitRepository()
	}

	// Get the writer from cli.Command
	w := cmd.Root().Writer
	if w == nil {
		w = os.Stdout
	}

	worktrees, err := repo.GetWorktrees()
	if err != nil {
		return fmt.Errorf("failed to get workspaces: %w", err)
	}

	return cdCommandWithWorktrees(w, worktrees, worktreeName)
}

func cdCommandWithWorktrees(w io.Writer, worktrees []git.Worktree, worktreeName string) error {
	mainWorktreePath := findMainWorktreePath(worktrees)
	targetPath := resolveWorktreePathByName(worktreeName, worktrees, mainWorktreePath)
	if targetPath == "" {
		availableWorktrees := availableManagedWorktreeNames(worktrees, mainWorktreePath)
		return errors.WorktreeNotFound(worktreeName, availableWorktrees)
	}
	_, err := fmt.Fprintln(w, targetPath)
	return err
}

// getWorktreeNameFromPathCd calculates the worktree name from its path (cd version)
// For main worktree, returns "@"
// For other worktrees, returns relative path from base_dir
func getWorktreeNameFromPathCd(worktreePath string, cfg *config.Config, mainRepoPath string, isMain bool) string {
	if isMain {
		return "@"
	}

	// Get base_dir path
	baseDir := cfg.Defaults.BaseDir
	if !filepath.IsAbs(baseDir) {
		baseDir = filepath.Join(mainRepoPath, baseDir)
	}

	// Calculate relative path from base_dir
	relPath, err := filepath.Rel(baseDir, worktreePath)
	if err != nil {
		// Fallback to directory name
		return filepath.Base(worktreePath)
	}

	return relPath
}

// getWorktreesForCd gets worktrees for cd command with current position markers and writes them to writer (testable)
func getWorktreesForCd(w io.Writer) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Initialize repository
	repo, err := git.NewRepository(cwd)
	if err != nil {
		return err
	}

	// Get main worktree path
	mainRepoPath, err := repo.GetMainWorktreePath()
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.LoadConfig(mainRepoPath)
	if err != nil {
		return err
	}

	// Get all worktrees
	worktrees, err := repo.GetWorktrees()
	if err != nil {
		return err
	}

	if err := writeMainWorktreeForCd(w, worktrees, cwd); err != nil {
		return err
	}

	return writeManagedWorktreesForCd(w, worktrees, cfg, mainRepoPath, cwd)
}

func writeMainWorktreeForCd(w io.Writer, worktrees []git.Worktree, cwd string) error {
	for i := range worktrees {
		wt := &worktrees[i]
		if wt.IsMain {
			if wt.Path == cwd {
				if _, err := fmt.Fprintln(w, "@*"); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "@"); err != nil {
					return err
				}
			}
			break
		}
	}

	return nil
}

func writeManagedWorktreesForCd(
	w io.Writer,
	worktrees []git.Worktree,
	cfg *config.Config,
	mainRepoPath string,
	cwd string,
) error {
	for i := range worktrees {
		wt := &worktrees[i]
		if !wt.IsMain && isWorktreeManagedCommon(wt.Path, cfg, mainRepoPath, wt.IsMain) {
			name := getWorktreeNameFromPathCd(wt.Path, cfg, mainRepoPath, wt.IsMain)
			if wt.Path == cwd {
				if _, err := fmt.Fprintf(w, "%s*\n", name); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, name); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// completeWorktreesForCd provides worktree name completion for cd command (wrapper for getWorktreesForCd)
func completeWorktreesForCd(_ context.Context, cmd *cli.Command) {
	current, previous := completionArgsFromCommand(cmd)

	if maybeCompleteFlagSuggestions(cmd, current, previous) {
		return
	}

	currentNormalized := strings.TrimSuffix(current, "*")

	if currentNormalized == "" && len(previous) > 0 {
		return
	}

	var buf bytes.Buffer
	if err := getWorktreesForCd(&buf); err != nil {
		return
	}

	used := make(map[string]struct{}, len(previous))
	for _, arg := range previous {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		key := strings.TrimSuffix(arg, "*")
		used[key] = struct{}{}
	}

	// Output each line using fmt.Println for urfave/cli compatibility
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		raw := scanner.Text()
		candidate := strings.TrimSuffix(raw, "*")

		if candidate == "" {
			continue
		}

		if _, exists := used[candidate]; exists {
			continue
		}

		if currentNormalized != "" && candidate == currentNormalized {
			continue
		}

		if _, err := fmt.Println(candidate); err != nil {
			return
		}
	}
}

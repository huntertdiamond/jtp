// Package framework contains helpers for constructing repositories during end-to-end tests.
package framework

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	dirPerm  = 0755
	filePerm = 0600
)

// TestEnvironment manages the temporary state for an end-to-end test run.
type TestEnvironment struct {
	t         *testing.T
	tmpDir    string
	wtpBinary string
	cleanup   []func()
}

// NewTestEnvironment builds a new test environment and compiles the jtp binary when needed.
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	tmpDir := t.TempDir()
	env := &TestEnvironment{
		t:       t,
		tmpDir:  tmpDir,
		cleanup: []func(){},
	}

	env.buildWTP()

	return env
}

func (e *TestEnvironment) buildWTP() {
	e.t.Helper()

	wtpBinary := filepath.Join(e.tmpDir, "wtp")
	if runtime := os.Getenv("WTP_E2E_BINARY"); runtime != "" {
		wtpBinary = runtime
		if _, err := os.Stat(wtpBinary); err != nil {
			e.t.Fatalf("Specified WTP binary not found: %s", wtpBinary)
		}
	} else {
		projectRoot := e.findProjectRoot()
		// #nosec G204 -- test helper builds the binary in an isolated temp directory
		cmd := exec.Command("go", "build", "-o", wtpBinary, "./cmd/wtp")
		cmd.Dir = projectRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			e.t.Fatalf("Failed to build jtp binary: %v\nOutput: %s", err, output)
		}
	}

	// Validate the binary path
	wtpBinary = filepath.Clean(wtpBinary)
	if !filepath.IsAbs(wtpBinary) {
		absPath, err := filepath.Abs(wtpBinary)
		if err != nil {
			e.t.Fatalf("Failed to get absolute path for binary: %v", err)
		}
		wtpBinary = absPath
	}

	e.wtpBinary = wtpBinary
}

func (e *TestEnvironment) findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		e.t.Fatalf("Failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			e.t.Fatal("Could not find project root (go.mod)")
		}
		dir = parent
	}
}

// CreateTestRepo initializes a new jj repository within the test environment.
func (e *TestEnvironment) CreateTestRepo(name string) *TestRepo {
	e.t.Helper()

	repoDir := filepath.Join(e.tmpDir, name)
	if err := os.MkdirAll(repoDir, dirPerm); err != nil {
		e.t.Fatalf("Failed to create repository directory: %v", err)
	}

	e.runInDir(repoDir, "jj", "git", "init")

	readmePath := filepath.Join(repoDir, "README.md")
	e.writeFile(readmePath, "# Test Repository")
	e.runInDir(repoDir, "jj", "describe", "-m", "Initial commit")
	e.runInDir(repoDir, "jj", "bookmark", "create", "main", "-r", "@")

	return &TestRepo{
		env:     e,
		path:    repoDir,
		remotes: make(map[string]string),
	}
}

func (e *TestEnvironment) runInDir(dir, command string, args ...string) string {
	e.t.Helper()

	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("Command failed in %s: %s %s\nOutput: %s\nError: %v",
			dir, command, strings.Join(args, " "), output, err)
	}
	return string(output)
}

func (e *TestEnvironment) writeFile(path, content string) {
	e.t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		e.t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), filePerm); err != nil {
		e.t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// RunWTP executes the jtp binary with the provided arguments.
func (e *TestEnvironment) RunWTP(args ...string) (string, error) {
	// Validate args don't contain dangerous characters
	for _, arg := range args {
		if err := validateArg(arg); err != nil {
			return "", fmt.Errorf("invalid argument: %w", err)
		}
	}

	// Create command with validated binary path
	cmd := createSafeCommand(e.wtpBinary, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// TmpDir returns the temporary directory used by the test environment.
func (e *TestEnvironment) TmpDir() string {
	return e.tmpDir
}

// CreateNonRepoDir creates a directory that is not initialized as a jj repository.
func (e *TestEnvironment) CreateNonRepoDir(name string) *TestRepo {
	e.t.Helper()

	dir := filepath.Join(e.tmpDir, name)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		e.t.Fatalf("Failed to create directory: %v", err)
	}

	return &TestRepo{
		env:     e,
		path:    dir,
		remotes: make(map[string]string),
	}
}

// WriteFile writes file contents relative to the test environment root.
func (e *TestEnvironment) WriteFile(path, content string) {
	e.writeFile(path, content)
}

// FileExists checks whether a file exists relative to the test environment root.
func (*TestEnvironment) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// RunInDir executes a command in the specified directory and returns combined output.
func (e *TestEnvironment) RunInDir(dir, command string, args ...string) string {
	return e.runInDir(dir, command, args...)
}

// Cleanup runs registered cleanup callbacks for the environment.
func (e *TestEnvironment) Cleanup() {
	for _, fn := range e.cleanup {
		fn()
	}
}

// TestRepo wraps a jj repository created inside the test environment.
type TestRepo struct {
	env     *TestEnvironment
	path    string
	remotes map[string]string
}

// RunWTP executes the jtp binary from the repository directory.
func (r *TestRepo) RunWTP(args ...string) (string, error) {
	// Validate args don't contain dangerous characters
	for _, arg := range args {
		if err := validateArg(arg); err != nil {
			return "", fmt.Errorf("invalid argument: %w", err)
		}
	}

	// Create command with validated binary path
	cmd := createSafeCommand(r.env.wtpBinary, args...)
	cmd.Dir = r.path
	cmd.Env = append(os.Environ(), "HOME="+r.env.tmpDir)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// CreateBranch creates a new bookmark in the repository.
func (r *TestRepo) CreateBranch(name string) {
	r.env.runInDir(r.path, "jj", "bookmark", "create", name, "-r", "@")
}

// CheckoutBranch starts a new working-copy commit from the specified bookmark.
func (r *TestRepo) CheckoutBranch(name string) {
	r.env.runInDir(r.path, "jj", "new", name)
}

// CommitFile writes a file and describes the current jj change with the provided message.
func (r *TestRepo) CommitFile(filename, content, message string) {
	r.env.writeFile(filepath.Join(r.path, filename), content)
	r.env.runInDir(r.path, "jj", "describe", "-m", message)
}

// AddRemote adds a local jj-backed Git remote to the repository.
func (r *TestRepo) AddRemote(name, url string) {
	_ = url

	remotePath := filepath.Join(r.env.tmpDir, r.repoName()+"-"+name+"-remote")
	if err := os.MkdirAll(remotePath, dirPerm); err != nil {
		r.env.t.Fatalf("Failed to create remote repository directory: %v", err)
	}

	r.env.runInDir(remotePath, "jj", "git", "init")
	r.env.writeFile(filepath.Join(remotePath, "README.md"), "# Remote Repository")
	r.env.runInDir(remotePath, "jj", "describe", "-m", "Initial remote commit")
	r.env.runInDir(remotePath, "jj", "bookmark", "create", "main", "-r", "@")
	r.env.runInDir(remotePath, "jj", "git", "export")
	r.env.runInDir(r.path, "jj", "git", "remote", "add", name, remotePath)

	r.remotes[name] = remotePath
}

func (r *TestRepo) repoName() string {
	return filepath.Base(r.path)
}

// CreateRemoteBranch creates and fetches a bookmark in the named remote repository.
func (r *TestRepo) CreateRemoteBranch(remote, branch string) {
	remotePath, ok := r.remotes[remote]
	if !ok {
		r.env.t.Fatalf("Remote %s has not been added", remote)
	}

	output := r.env.runInDir(remotePath, "jj", "bookmark", "list", "--template", `name ++ "\n"`)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == branch {
			r.env.runInDir(r.path, "jj", "git", "fetch", "--remote", remote)
			return
		}
	}
	r.env.runInDir(remotePath, "jj", "bookmark", "create", branch, "-r", "@")
	r.env.runInDir(remotePath, "jj", "git", "export")
	r.env.runInDir(r.path, "jj", "git", "fetch", "--remote", remote)
}

// Path returns the filesystem path of the repository.
func (r *TestRepo) Path() string {
	return r.path
}

// WriteConfig writes a .wtp.yml configuration file into the repository.
func (r *TestRepo) WriteConfig(content string) {
	configPath := filepath.Join(r.path, ".wtp.yml")
	r.env.writeFile(configPath, content)
}

// HasFile reports whether a file exists relative to the repository root.
func (r *TestRepo) HasFile(path string) bool {
	fullPath := filepath.Join(r.path, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// ReadFile returns the contents of a file relative to the repository root.
func (r *TestRepo) ReadFile(path string) string {
	fullPath := filepath.Join(r.path, path)
	// #nosec G304 -- file paths are confined to the temporary test repository
	content, err := os.ReadFile(fullPath)
	if err != nil {
		r.env.t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}

// GitStatus returns the output of `jj status` for the repository.
func (r *TestRepo) GitStatus() string {
	return r.env.runInDir(r.path, "jj", "status")
}

// CurrentBranch returns the closest bookmark on the current working-copy commit.
func (r *TestRepo) CurrentBranch() string {
	output := r.env.runInDir(r.path, "jj", "log", "--no-graph", "-r", "@", "-T", `bookmarks ++ "\n"`)
	return strings.TrimSpace(output)
}

// GetCommitHash returns the current commit hash.
func (r *TestRepo) GetCommitHash() string {
	output := r.env.runInDir(r.path, "jj", "log", "--no-graph", "-r", "@", "-T", `commit_id ++ "\n"`)
	return strings.TrimSpace(output)
}

// GetBranchCommitHash returns the commit hash for the specified bookmark.
func (r *TestRepo) GetBranchCommitHash(branch string) string {
	output := r.env.runInDir(r.path, "jj", "log", "--no-graph", "-r", branch, "-T", `commit_id ++ "\n"`)
	return strings.TrimSpace(output)
}

// ListWorktrees returns the list of worktrees known to the repository.
func (r *TestRepo) ListWorktrees() []string {
	output := r.env.runInDir(r.path, "jj", "workspace", "list", "--template", `name ++ "\n"`)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var worktrees []string
	for _, line := range lines {
		workspaceName := strings.TrimSpace(line)
		if workspaceName != "" {
			root := r.env.runInDir(r.path, "jj", "workspace", "root", "--name", workspaceName)
			worktrees = append(worktrees, strings.TrimSpace(root))
		}
	}
	return worktrees
}

// WithTimeout adds a timeout to an exec command for use in helpers.
func WithTimeout(timeout time.Duration) func(cmd *exec.Cmd) {
	return func(cmd *exec.Cmd) {
		timer := time.AfterFunc(timeout, func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})
		_ = cmd.Start()
		_ = cmd.Wait()
		timer.Stop()
	}
}

// validateArg checks if an argument is safe to pass to exec.Command
func validateArg(arg string) error {
	// Allow common flags and paths
	// This is a whitelist approach for test arguments
	if arg == "" {
		return nil
	}

	// Check for shell metacharacters that could be dangerous
	// Note: { and } are allowed for branch names like branch@{upstream}
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// createSafeCommand creates an exec.Cmd with a validated binary path
func createSafeCommand(binary string, args ...string) *exec.Cmd {
	// The binary path has already been validated during initialization
	// This function separates the concern of command creation from validation
	return exec.Command(binary, args...)
}

package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// driftAuditCmd flags
var (
	driftAuditDays int
	driftAuditAll  bool
)

var driftAuditCmd = &cobra.Command{
	Use:   "drift-audit",
	Short: "Find rule files referencing deleted or renamed paths",
	Long: `Scan git history for files that have been deleted or renamed in the
last N days, then check your rule files (CLAUDE.md, .cursorrules,
AGENTS.md, etc.) for references to those paths.

This catches "stale rules" — instructions to your AI that point at
files which no longer exist.

Heuristic only — does not call an LLM, does not modify any rule file,
just reports candidates for manual review.

Examples:
  contextsync drift-audit                # last 90 days
  contextsync drift-audit --days=30      # last 30 days
  contextsync drift-audit --all          # full history`,
	Run: func(cmd *cobra.Command, args []string) {
		runDriftAudit()
	},
}

func init() {
	driftAuditCmd.Flags().IntVar(&driftAuditDays, "days", 90, "How many days of git history to scan")
	driftAuditCmd.Flags().BoolVar(&driftAuditAll, "all", false, "Scan full git history (overrides --days)")
}

// ruleFileCandidates lists rule files we'll grep for stale paths.
// Both project-local and home-dir variants are checked.
func ruleFileCandidates() []string {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "CLAUDE.md"),
		filepath.Join(cwd, ".cursorrules"),
		filepath.Join(cwd, "AGENTS.md"),
		filepath.Join(cwd, "GEMINI.md"),
		filepath.Join(cwd, ".windsurfrules"),
		filepath.Join(cwd, ".github", "copilot-instructions.md"),
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".cursorrules"),
		filepath.Join(home, ".gemini", "GEMINI.md"),
		filepath.Join(home, ".codex", "AGENTS.md"),
		filepath.Join(home, ".github", "copilot-instructions.md"),
		filepath.Join(home, ".contextsync", "rules.md"),
	}
	return candidates
}

func runDriftAudit() {
	titleStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	hitStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))

	if err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		fmt.Println(dimStyle.Render("  Not a git repository — `drift-audit` needs git history."))
		return
	}

	// Build the git log args for deleted/renamed paths.
	args := []string{"log", "--diff-filter=DR", "--name-status", "--pretty=format:"}
	if !driftAuditAll {
		args = append(args, fmt.Sprintf("--since=%d days ago", driftAuditDays))
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		fmt.Printf("git log failed: %v\n", err)
		return
	}

	deleted, renamed := parseNameStatus(string(out))

	fmt.Println(titleStyle.Render("\nContextSync Drift Audit\n"))
	if driftAuditAll {
		fmt.Println(dimStyle.Render("  Scope: full git history"))
	} else {
		fmt.Printf("  %s last %d days\n", dimStyle.Render("Scope: "), driftAuditDays)
	}
	fmt.Printf("  %s %d deleted, %d renamed\n", dimStyle.Render("Found: "), len(deleted), len(renamed))

	// Collect rule files that exist.
	var rulePaths []string
	for _, p := range ruleFileCandidates() {
		if _, err := os.Stat(p); err == nil {
			rulePaths = append(rulePaths, p)
		}
	}
	if len(rulePaths) == 0 {
		fmt.Println()
		fmt.Println(dimStyle.Render("  No rule files found in this directory or your home dir."))
		fmt.Println(dimStyle.Render("  (Looked for CLAUDE.md, .cursorrules, AGENTS.md, GEMINI.md, .windsurfrules, etc.)"))
		fmt.Println()
		return
	}
	fmt.Printf("  %s %d rule file(s) scanned\n", dimStyle.Render("Files: "), len(rulePaths))

	// Build candidate path set: deleted paths + the OLD path of renames.
	stalePaths := map[string]string{} // path -> reason ("deleted" / "renamed → newpath")
	for _, p := range deleted {
		stalePaths[p] = "deleted"
	}
	for from, to := range renamed {
		stalePaths[from] = "renamed → " + to
	}

	type drift struct {
		ruleFile string
		ruleLine int
		stale    string
		reason   string
		excerpt  string
	}
	var drifts []drift

	for _, ruleFile := range rulePaths {
		f, err := os.Open(ruleFile)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			text := scanner.Text()
			for stale, reason := range stalePaths {
				if pathReferenced(text, stale) {
					drifts = append(drifts, drift{
						ruleFile: ruleFile,
						ruleLine: lineNum,
						stale:    stale,
						reason:   reason,
						excerpt:  strings.TrimSpace(text),
					})
				}
			}
		}
		f.Close()
	}

	if len(drifts) == 0 {
		fmt.Println()
		fmt.Println(okStyle.Render("  No stale path references in your rule files."))
		fmt.Println()
		return
	}

	// Sort: by file, then line.
	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].ruleFile != drifts[j].ruleFile {
			return drifts[i].ruleFile < drifts[j].ruleFile
		}
		return drifts[i].ruleLine < drifts[j].ruleLine
	})

	fmt.Println()
	fmt.Println(titleStyle.Render(fmt.Sprintf("Stale references (%d):", len(drifts))))
	fmt.Println()
	home, _ := os.UserHomeDir()
	for _, d := range drifts {
		display := d.ruleFile
		if home != "" && strings.HasPrefix(display, home) {
			display = "~" + strings.TrimPrefix(display, home)
		}
		fmt.Printf("  %s %s\n",
			pathStyle.Render(fmt.Sprintf("%s:%d", display, d.ruleLine)),
			hitStyle.Render(d.stale))
		fmt.Printf("    %s %s\n", dimStyle.Render("git status:"), dimStyle.Render(d.reason))
		fmt.Printf("    %s %s\n", dimStyle.Render("excerpt:   "), dimStyle.Render(d.excerpt))
		fmt.Println()
	}
	fmt.Println(dimStyle.Render("  Review each match — your rule may still be valid if it referenced"))
	fmt.Println(dimStyle.Render("  the path conceptually rather than as a live pointer."))
	fmt.Println()
}

// parseNameStatus parses the output of `git log --diff-filter=DR --name-status`.
// Returns deleted file list and rename map (from -> to).
func parseNameStatus(out string) ([]string, map[string]string) {
	var deleted []string
	renamed := map[string]string{}
	seenDeleted := map[string]bool{}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		switch {
		case strings.HasPrefix(status, "D"):
			path := fields[1]
			if !seenDeleted[path] {
				deleted = append(deleted, path)
				seenDeleted[path] = true
			}
		case strings.HasPrefix(status, "R"):
			if len(fields) >= 3 {
				renamed[fields[1]] = fields[2]
			}
		}
	}
	return deleted, renamed
}

// pathReferenced returns true if `text` mentions `path` in a way that
// looks like a rule pointer rather than a coincidental substring.
//
// Strategy: require the path to appear delimited by whitespace, quotes,
// backticks, parens, or markdown syntax — not embedded in a longer word.
func pathReferenced(text, path string) bool {
	if path == "" {
		return false
	}
	idx := strings.Index(text, path)
	if idx < 0 {
		return false
	}
	leftOK := idx == 0 || isPathBoundary(rune(text[idx-1]))
	end := idx + len(path)
	rightOK := end >= len(text) || isPathBoundary(rune(text[end]))
	return leftOK && rightOK
}

func isPathBoundary(r rune) bool {
	switch r {
	case ' ', '\t', '"', '\'', '`', '(', ')', '[', ']', '{', '}', ',', ';', ':', '!', '?':
		return true
	}
	return false
}

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"contextsync/internal/config"
	"contextsync/internal/rules"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage rules",
}

var rulesEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open rules file in editor",
	Run: func(cmd *cobra.Command, args []string) {
		rulesPath := config.GetRulesPath()
		fmt.Printf("Rules file: %s\n", rulesPath)
		fmt.Println("Open this file in your favorite editor.")
	},
}

var rulesShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current rules",
	Run: func(cmd *cobra.Command, args []string) {
		engine := rules.NewEngine()
		content, err := engine.GetRules("")
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		fmt.Println(content)
	},
}

var rulesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync rules to all configured tools",
	Run: func(cmd *cobra.Command, args []string) {
		runRulesSync()
	},
}

func runRulesSync() {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	// Initialize database
	if err := initDatabase(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer closeDatabase()

	// Check license status
	isPro := false
	maxTools := 2
	if validator != nil {
		isPro = validator.IsPro()
		maxTools = validator.GetMaxTools()
	}

	// Get configured tools from database
	configuredTools := getConfiguredTools()

	if len(configuredTools) == 0 {
		fmt.Println(warnStyle.Render("  No tools configured. Run: contextsync init"))
		return
	}

	// Free tier: limit to maxTools
	if !isPro && len(configuredTools) > maxTools {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Free tier: syncing only first %d tools", maxTools)))
		configuredTools = configuredTools[:maxTools]
	}

	// Read rules content
	home, _ := os.UserHomeDir()
	rulesPath := filepath.Join(home, ".contextsync", "rules.md")
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		fmt.Printf("Error reading rules: %v\n", err)
		return
	}

	// Tool name to target file mapping
	targetMapping := map[string]string{
		"Claude Code":    filepath.Join(home, ".claude", "CLAUDE.md"),
		"Cursor":         filepath.Join(home, ".cursorrules"),
		"Windsurf":       filepath.Join(home, ".codeium", "windsurfrules"),
		"Gemini CLI":     filepath.Join(home, ".gemini", "GEMINI.md"),
		"Codex CLI":      filepath.Join(home, ".codex", "AGENTS.md"),
		"GitHub Copilot": filepath.Join(home, ".github", "copilot-instructions.md"),
		"Cline":          filepath.Join(home, ".cline", "CLINE.md"),
		"Roo Code":       filepath.Join(home, ".roo", "ROO.md"),
		"Aider":          filepath.Join(home, ".aider", "AIDER.md"),
		"Continue":       filepath.Join(home, ".continue", "CONTINUE.md"),
		"Zed":            filepath.Join(home, ".zed", "ZED.md"),
	}

	fmt.Println(infoStyle.Render("  Syncing rules to configured tools..."))

	syncedCount := 0
	for _, toolName := range configuredTools {
		targetPath, ok := targetMapping[toolName]
		if !ok {
			continue
		}

		// Ensure directory exists
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("  %s: failed to create directory\n", toolName)
			continue
		}

		// Write file
		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			fmt.Printf("  %s: %v\n", toolName, err)
		} else {
			fmt.Printf("  %s: synced\n", toolName)
			syncedCount++
		}
	}

	fmt.Println()
	fmt.Println(successStyle.Render(fmt.Sprintf("  Rules synced to %d tool(s).", syncedCount)))
}

func getConfiguredTools() []string {
	if database == nil {
		return nil
	}

	rows, err := database.DB().Query("SELECT tool_name FROM configured_tools")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var tools []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tools = append(tools, name)
		}
	}
	return tools
}

func init() {
	rulesCmd.AddCommand(rulesEditCmd)
	rulesCmd.AddCommand(rulesShowCmd)
	rulesCmd.AddCommand(rulesSyncCmd)
	rulesCmd.AddCommand(rulesCheckCmd)
	rulesCheckCmd.Flags().IntVar(&rulesCheckRange, "range", 5, "How many recent commits to scan (HEAD~N..HEAD)")
	rulesCheckCmd.Flags().BoolVar(&rulesCheckUnstaged, "unstaged", false, "Scan unstaged working tree changes instead of recent commits")
}

// ─── rules check ──────────────────────────────────────────────────────────────

var (
	rulesCheckRange    int
	rulesCheckUnstaged bool
)

var rulesCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Scan recent git changes for code that may violate your rules",
	Long: `Scan a git diff for lines that mention forbidden phrases pulled from
your ~/.contextsync/rules.md.

This is a heuristic — it does NOT call an LLM, does NOT block commits,
and only reports possible matches. Use it as a hint, not a gate.

Examples:
  contextsync rules check                    # last 5 commits
  contextsync rules check --range=20         # last 20 commits
  contextsync rules check --unstaged         # working tree only`,
	Run: func(cmd *cobra.Command, args []string) {
		runRulesCheck()
	},
}

// banPattern is a phrase extracted from rules.md that should NOT appear in code.
type banPattern struct {
	phrase string
	rule   string // the line of rules.md it came from
}

func runRulesCheck() {
	titleStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	hitStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))

	// Verify we're inside a git repo.
	if err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		fmt.Println(dimStyle.Render("  Not a git repository — `rules check` needs git history."))
		return
	}

	// Load rules.
	engine := rules.NewEngine()
	rulesContent, err := engine.GetRules("")
	if err != nil {
		fmt.Printf("Error reading rules: %v\n", err)
		return
	}

	patterns := extractBanPatterns(rulesContent)
	if len(patterns) == 0 {
		fmt.Println(dimStyle.Render("  No actionable ban-phrases found in your rules.md."))
		fmt.Println(dimStyle.Render("  Add lines like: - Never use Yup; use Zod instead"))
		fmt.Println(dimStyle.Render("                  - Don't import lodash, prefer native JS"))
		fmt.Println(dimStyle.Render("  See `contextsync rules show` for current content."))
		return
	}

	// Get diff.
	var out []byte
	if rulesCheckUnstaged {
		out, err = exec.Command("git", "diff", "--no-color").Output()
		if err != nil {
			fmt.Printf("git diff failed: %v\n", err)
			return
		}
	} else {
		out, err = gitDiffRange(rulesCheckRange)
		if err != nil {
			fmt.Printf("git diff failed: %v\n", err)
			return
		}
	}

	hits := scanDiff(string(out), patterns)

	fmt.Println(titleStyle.Render("\nContextSync Rules Check\n"))
	fmt.Printf("  %s %d ban-phrases\n", dimStyle.Render("Loaded:"), len(patterns))
	if rulesCheckUnstaged {
		fmt.Printf("  %s working tree (unstaged)\n", dimStyle.Render("Scope: "))
	} else {
		fmt.Printf("  %s last %d commits\n", dimStyle.Render("Scope: "), rulesCheckRange)
	}

	if len(hits) == 0 {
		fmt.Println()
		fmt.Println(okStyle.Render("  No rule violations detected."))
		fmt.Println(dimStyle.Render("  (Heuristic match only — does not catch semantic violations.)"))
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Println(titleStyle.Render(fmt.Sprintf("Possible violations (%d):", len(hits))))
	fmt.Println()
	for _, h := range hits {
		fmt.Printf("  %s %s\n", pathStyle.Render(h.file+":"+fmt.Sprintf("%d", h.line)), hitStyle.Render(`"`+h.phrase+`"`))
		fmt.Printf("    %s\n", dimStyle.Render("rule: "+h.rule))
		fmt.Printf("    %s\n", dimStyle.Render("code: "+strings.TrimSpace(h.codeLine)))
		fmt.Println()
	}

	fmt.Println(dimStyle.Render("  This is a heuristic — review each hit, false positives expected."))
	fmt.Println(dimStyle.Render("  To suppress a false positive, refine the rule wording in rules.md."))
	fmt.Println()
}

// extractBanPatterns pulls "ban phrases" out of rules.md.
//
// Heuristic: in markdown list items, find a trigger word (never / don't /
// avoid / no / stop), then walk forward over verb-like tokens
// (use, using, import, importing, install, installing, run) and take the
// next noun-ish token as the ban-phrase.
//
// "Never use Yup" → Yup
// "Don't import lodash" → lodash
// "Avoid jQuery" → jQuery
// "No console.log in committed code" → console.log
//
// Intentionally simple — false positives are expected and documented.
func extractBanPatterns(content string) []banPattern {
	triggerRE := regexp.MustCompile(`(?i)\b(never|don'?t|do not|avoid|stop|no)\b`)
	verbSkip := map[string]bool{
		"use": true, "using": true, "uses": true,
		"import": true, "importing": true, "imports": true,
		"install": true, "installing": true, "installs": true,
		"run": true, "running": true, "runs": true,
		"call": true, "calling": true, "calls": true,
	}
	tokenRE := regexp.MustCompile(`[A-Za-z][A-Za-z0-9_./@\-]*`)

	var out []banPattern
	seen := map[string]bool{}

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*") && !strings.HasPrefix(line, "+") {
			continue
		}

		loc := triggerRE.FindStringIndex(line)
		if loc == nil {
			continue
		}
		// Tokenize the slice after the trigger.
		rest := line[loc[1]:]
		tokens := tokenRE.FindAllString(rest, -1)

		// Skip verb-like tokens, take the next.
		var phrase string
		for _, tok := range tokens {
			if verbSkip[strings.ToLower(tok)] {
				continue
			}
			phrase = tok
			break
		}
		phrase = strings.Trim(phrase, ".,;:!?\"'`")
		if len(phrase) < 2 || isStopWord(phrase) {
			continue
		}
		key := strings.ToLower(phrase)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, banPattern{phrase: phrase, rule: line})
	}
	return out
}

// isStopWord filters trivial words that would generate noisy hits.
func isStopWord(s string) bool {
	stop := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"to": true, "of": true, "is": true, "are": true, "be": true,
		"this": true, "that": true, "it": true, "code": true, "rule": true,
		"rules": true, "file": true, "files": true, "any": true, "all": true,
		"new": true, "old": true, "use": true, "using": true,
	}
	return stop[strings.ToLower(s)]
}

// hit is a single candidate violation.
type hit struct {
	file     string
	line     int
	phrase   string
	rule     string
	codeLine string
}

// scanDiff walks a unified-diff and reports occurrences of ban phrases on
// added (+) lines.
func scanDiff(diff string, patterns []banPattern) []hit {
	var hits []hit

	var currentFile string
	var lineNumber int

	hunkHeader := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
	fileHeader := regexp.MustCompile(`^\+\+\+ b/(.+)$`)

	for _, line := range strings.Split(diff, "\n") {
		if m := fileHeader.FindStringSubmatch(line); m != nil {
			currentFile = m[1]
			continue
		}
		if m := hunkHeader.FindStringSubmatch(line); m != nil {
			fmt.Sscanf(m[1], "%d", &lineNumber)
			continue
		}
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			content := strings.TrimPrefix(line, "+")
			for _, p := range patterns {
				if containsWord(content, p.phrase) {
					hits = append(hits, hit{
						file:     currentFile,
						line:     lineNumber,
						phrase:   p.phrase,
						rule:     p.rule,
						codeLine: content,
					})
				}
			}
			lineNumber++
		} else if !strings.HasPrefix(line, "-") {
			lineNumber++
		}
	}

	// Stable order: by file then line.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].file != hits[j].file {
			return hits[i].file < hits[j].file
		}
		return hits[i].line < hits[j].line
	})
	return hits
}

// containsWord returns true if `phrase` appears in `text` at a word boundary,
// case-insensitive. Falls back to substring match for non-alphanumeric
// phrases (paths, package names, etc.).
func containsWord(text, phrase string) bool {
	lt, lp := strings.ToLower(text), strings.ToLower(phrase)
	if !strings.ContainsAny(lp, "abcdefghijklmnopqrstuvwxyz0123456789") {
		return strings.Contains(lt, lp)
	}
	// Use a simple boundary check rather than regexp for speed.
	idx := 0
	for {
		i := strings.Index(lt[idx:], lp)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(lp)
		left := start == 0 || !isWordChar(rune(lt[start-1]))
		right := end >= len(lt) || !isWordChar(rune(lt[end]))
		if left && right {
			return true
		}
		idx = end
	}
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// gitDiffRange returns the unified diff covering the last N commits.
// If the repo has fewer than N commits we clamp to count-1 (or use the
// empty tree as the base for a single-commit repo).
func gitDiffRange(n int) ([]byte, error) {
	countOut, err := exec.Command("git", "rev-list", "--count", "HEAD").Output()
	if err != nil {
		return nil, err
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(string(countOut)), "%d", &count)
	if count == 0 {
		return nil, fmt.Errorf("no commits yet")
	}
	if n >= count {
		n = count - 1
	}
	if n <= 0 {
		// Single-commit repo: diff against the empty tree.
		emptyTreeOut, err := exec.Command("git", "hash-object", "-t", "tree", "/dev/null").Output()
		emptyTree := "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // standard empty tree SHA
		if err == nil {
			if s := strings.TrimSpace(string(emptyTreeOut)); s != "" {
				emptyTree = s
			}
		}
		return exec.Command("git", "diff", "--no-color", emptyTree, "HEAD").Output()
	}
	ref := fmt.Sprintf("HEAD~%d..HEAD", n)
	return exec.Command("git", "diff", "--no-color", ref).Output()
}

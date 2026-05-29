package cli

import (
	"fmt"
	"sort"

	"contextsync/internal/config"
	"contextsync/internal/license"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show memory statistics and estimated tokens saved",
	Long: `Show memory statistics: how much you've taught your AI tools,
which projects and categories dominate, and an estimated token saving
based on the content length stored.

This is an estimate, not a metered measurement.`,
	Run: func(cmd *cobra.Command, args []string) {
		runStats()
	},
}

func runStats() {
	titleStyle := lipgloss.NewStyle().Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	ensureDatabase()
	repo := getMemoryRepo()
	es := repo.GetExtendedStats()

	tier := "free"
	if config.IsLoggedIn() {
		v := license.NewValidator(config.GetServerURL())
		v.SetDB(database)
		tier = v.GetTier()
	}

	fmt.Println(titleStyle.Render("\nContextSync Memory Stats\n"))

	// Headline
	fmt.Printf("  %-22s %s\n", labelStyle.Render("Memories stored:"), valueStyle.Render(fmt.Sprintf("%d", es.Total)))
	if es.Total == 0 {
		fmt.Println()
		fmt.Println(dimStyle.Render("  No memories yet. Save your first one with the save_memory MCP tool"))
		fmt.Println(dimStyle.Render("  from any configured AI tool, then run `contextsync stats` again."))
		fmt.Println()
		return
	}

	if es.Expiring > 0 {
		fmt.Printf("  %-22s %s\n", labelStyle.Render("Expiring within 3d:"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render(fmt.Sprintf("%d", es.Expiring)))
	}
	if es.UnsyncedCount > 0 {
		fmt.Printf("  %-22s %s %s\n", labelStyle.Render("Unsynced (local):"),
			valueStyle.Render(fmt.Sprintf("%d", es.UnsyncedCount)),
			dimStyle.Render("(run `contextsync sync` to push)"))
	}

	// Time span
	if es.OldestCreatedAt != nil && es.NewestCreatedAt != nil {
		span := es.NewestCreatedAt.Sub(*es.OldestCreatedAt)
		days := int(span.Hours() / 24)
		fmt.Printf("  %-22s %s %s\n", labelStyle.Render("Time span:"),
			valueStyle.Render(fmt.Sprintf("%d days", days)),
			dimStyle.Render(fmt.Sprintf("(%s → %s)",
				es.OldestCreatedAt.Format("2006-01-02"),
				es.NewestCreatedAt.Format("2006-01-02"))))
	}

	// Content volume
	fmt.Println()
	fmt.Println(titleStyle.Render("Content volume:"))
	fmt.Printf("  %-22s %s\n", labelStyle.Render("Total characters:"),
		valueStyle.Render(formatThousands(es.TotalCharacters)))
	fmt.Printf("  %-22s %s\n", labelStyle.Render("Average per memory:"),
		valueStyle.Render(fmt.Sprintf("%d chars", es.AverageCharacters)))

	// Token saving estimate
	// Rough heuristic: 1 token ≈ 4 chars; assume each memory would otherwise
	// be re-explained ~3 times (across sessions / tools) before settling.
	estimatedTokens := es.TotalCharacters / 4
	estimatedSaved := estimatedTokens * 3
	fmt.Println()
	fmt.Println(titleStyle.Render("Estimated tokens saved (rough heuristic):"))
	fmt.Printf("  %-22s %s\n", labelStyle.Render("Stored as tokens:"),
		valueStyle.Render(fmt.Sprintf("~%s", formatThousands(estimatedTokens))))
	fmt.Printf("  %-22s %s\n", labelStyle.Render("Saved (3× re-use):"),
		accentStyle.Render(fmt.Sprintf("~%s tokens", formatThousands(estimatedSaved))))
	fmt.Println(dimStyle.Render("  (estimate — assumes you would have re-explained each memory ~3 times"))
	fmt.Println(dimStyle.Render("   without ContextSync. Not a metered measurement.)"))

	// By category
	if len(es.ByCategory) > 0 {
		fmt.Println()
		fmt.Println(titleStyle.Render("By category:"))
		printSortedCounts(es.ByCategory, labelStyle, valueStyle, es.Total)
	}

	// By project
	if len(es.ByProject) > 0 {
		fmt.Println()
		fmt.Println(titleStyle.Render("Top projects:"))
		printSortedCounts(es.ByProject, labelStyle, valueStyle, es.Total)
	}

	// Tier-aware footer
	fmt.Println()
	switch tier {
	case "pro", "team":
		fmt.Println(dimStyle.Render(fmt.Sprintf("  Tier: %s — memories never expire.", tier)))
	default:
		fmt.Println(dimStyle.Render("  Tier: free — memories expire after 14 days. `contextsync upgrade` to keep them forever."))
	}
	fmt.Println()
}

// printSortedCounts prints a name → count map sorted by count desc.
func printSortedCounts(m map[string]int, label, value lipgloss.Style, total int) {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	for _, p := range pairs {
		pct := 0
		if total > 0 {
			pct = p.v * 100 / total
		}
		fmt.Printf("  %-22s %s %s\n",
			label.Render(p.k+":"),
			value.Render(fmt.Sprintf("%d", p.v)),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(fmt.Sprintf("(%d%%)", pct)))
	}
}

// formatThousands inserts thousands separators.
func formatThousands(n int) string {
	if n < 0 {
		return "-" + formatThousands(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return formatThousands(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}

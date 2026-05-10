package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var memoriesCmd = &cobra.Command{
	Use:   "memories",
	Short: "Manage memories",
}

var memoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all memories",
	Run: func(cmd *cobra.Command, args []string) {
		repo := getMemoryRepo()
		memories := repo.List("", 50)

		if len(memories) == 0 {
			fmt.Println("No memories found.")
			return
		}

		fmt.Printf("\n  Memories (%d):\n\n", len(memories))
		for i, m := range memories {
			fmt.Printf("  %d. [%s] %s\n", i+1, m.Category, truncate(m.Content, 80))
		}
		fmt.Println()
	},
}

var memoriesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a specific memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := getMemoryRepo()
		mem, err := repo.GetByID(args[0])
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Memory not found: %s\n", args[0])
			return
		}

		fmt.Printf("\n  Memory: %s\n\n", mem.ID)
		fmt.Printf("  Category: %s\n", mem.Category)
		fmt.Printf("  Source:   %s\n", mem.Source)
		fmt.Printf("  Created:  %s\n", mem.CreatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("\n  Content:\n\n  %s\n\n", mem.Content)
	},
}

var memoriesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := getMemoryRepo()
		if err := repo.Delete(args[0]); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to delete: %v\n", err)
			return
		}
		fmt.Printf("Memory %s deleted.\n", args[0])
	},
}

func init() {
	memoriesCmd.AddCommand(memoriesListCmd)
	memoriesCmd.AddCommand(memoriesShowCmd)
	memoriesCmd.AddCommand(memoriesDeleteCmd)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

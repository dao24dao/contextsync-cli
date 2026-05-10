package cli

import (
	"fmt"

	"contextsync/internal/config"
	"contextsync/internal/rules"
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
	Short: "Sync rules to all tools",
	Run: func(cmd *cobra.Command, args []string) {
		engine := rules.NewEngine()
		if err := engine.Compile(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
		fmt.Println("Rules synced to all tools.")
	},
}

func init() {
	rulesCmd.AddCommand(rulesEditCmd)
	rulesCmd.AddCommand(rulesShowCmd)
	rulesCmd.AddCommand(rulesSyncCmd)
}

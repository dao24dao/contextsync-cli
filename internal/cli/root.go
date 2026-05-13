package cli

import (
	"fmt"
	"os"

	"contextsync/internal/config"
	"contextsync/internal/db"
	"contextsync/internal/license"
	"contextsync/internal/memory"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	version string
	commit  string
	date    string
)

var rootCmd = &cobra.Command{
	Use:   "contextsync",
	Short: "Cross-tool AI coding context hub",
	Long: `ContextSync - Unified rules and memory sync across AI coding tools.

Share context between Claude Code, Cursor, Windsurf, Gemini CLI, and more.
One set of rules, one memory store, all your AI tools.`,
}

func Execute(v, c, d string) error {
	version = v
	commit = c
	date = d
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.contextsync/config.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add commands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(activateCmd)
	rootCmd.AddCommand(deactivateCmd)
	rootCmd.AddCommand(rulesCmd)
	rootCmd.AddCommand(memoriesCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(daemonCmd)
}

func initConfig() {
	if cfgFile != "" {
		config.SetConfigFile(cfgFile)
	}
	config.Init()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ContextSync %s\n", version)
		fmt.Printf("  Commit: %s\n", commit)
		fmt.Printf("  Built:  %s\n", date)
	},
}

// Global instances
var (
	database  *db.SQLite
	validator *license.Validator
)

func initDatabase() error {
	var err error
	database, err = db.NewSQLite(config.GetDataPath())
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize license validator
	validator = license.NewValidator(config.GetServerURL())
	validator.SetDB(database)

	return nil
}

func closeDatabase() {
	if database != nil {
		database.Close()
	}
}

func ensureDatabase() {
	if database == nil {
		if err := initDatabase(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// getMemoryRepo returns a memory repository with proper Pro checker
func getMemoryRepo() *memory.Repository {
	ensureDatabase()
	return memory.NewRepository(database, memory.WithProChecker(validator.IsPro))
}

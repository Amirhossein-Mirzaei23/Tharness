package cli

import (
	"fmt"
	"os"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/config"
	"github.com/Amirhossein-Mirzaei23/Tharness/internal/session"
	"github.com/spf13/cobra"
)

// newSessionsCommand constructs `tharness sessions` with the `list` subcommand.
func newSessionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage stored sessions",
	}
	cmd.AddCommand(newSessionsListCommand())
	return cmd
}

// newSessionsListCommand constructs `tharness sessions list`, which prints stored
// session ids, newest first.
func newSessionsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored session ids",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("cli: load config: %w", err)
			}
			dataDir := firstNonEmpty(os.Getenv("THARNESS_DATA_DIR"), cfg.DataDir)
			store, err := session.Open(dataDir)
			if err != nil {
				return err
			}
			sessions, err := store.List()
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no sessions")
				return nil
			}
			for _, s := range sessions {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d lines\t%s\n", s.ID, s.LineCount, s.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}
}

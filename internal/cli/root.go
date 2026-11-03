// Package cli defines the tharness command-line interface.
package cli

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Amirhossein-Mirzaei23/Tharness/internal/tui"
	"github.com/spf13/cobra"
)

// version is the release version, stamped into `tharness --version`.
const version = "0.0.1"

// tuiStarter launches the interactive TUI. Tests override this seam to avoid
// starting a real terminal program.
var tuiStarter = startTUI

// startTUI is the default tuiStarter; it runs the bubbletea chat program
// backed by the real agent ChatRunner, styled with the configured theme.
func startTUI() error {
	runner, err := tui.NewChatRunner("")
	if err != nil {
		return err
	}
	m := tui.New(newTextarea(), runner).WithTheme(runner.Theme())
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// newTextarea returns the shared input component configuration.
func newTextarea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message…"
	ta.Focus()
	return ta
}

// NewRootCommand constructs the tharness root command. When run with -p/--prompt
// it executes a one-shot agent session; with no arguments it starts the
// interactive TUI.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "tharness",
		Short: "tharness — a single-binary coding-agent CLI harness",
		Long: `tharness — a single-binary coding-agent CLI harness.

Run interactively with no arguments, or one-shot with -p:
  tharness -p "summarize this repo"`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt, _ := cmd.Flags().GetString("prompt")
			if prompt == "" {
				return tuiStarter()
			}
			return runOneShot(cmd, prompt)
		},
	}

	root.PersistentFlags().String("config", "", "path to config file (default: ~/.tharness/config.yaml)")
	root.Flags().StringP("prompt", "p", "", "one-shot prompt: run the agent once and print the answer")
	root.Flags().String("resume", "", "resume a previous session by id (combine with -p)")

	root.AddCommand(newRunCommand())
	root.AddCommand(newSessionsCommand())
	root.AddCommand(newChatCommand())
	root.AddCommand(newSetupCommand())

	return root
}

// newChatCommand constructs the explicit `tharness chat` TUI subcommand.
func newChatCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start the interactive chat TUI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tuiStarter()
		},
	}
}

// newRunCommand constructs the explicit `tharness run` subcommand.
func newRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Run a one-shot agent session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt, _ := cmd.Flags().GetString("prompt")
			if prompt == "" {
				return cmd.Help()
			}
			return runOneShot(cmd, prompt)
		},
	}
	run.Flags().StringP("prompt", "p", "", "prompt to send to the agent (required)")
	run.Flags().String("resume", "", "resume a previous session by id")
	return run
}

// runOneShot resolves config path and output streams from cmd and delegates
// to RunOptions.Run.
func runOneShot(cmd *cobra.Command, prompt string) error {
	cfgPath, _ := cmd.Flags().GetString("config")
	resume, _ := cmd.Flags().GetString("resume")
	return RunOptions{Prompt: prompt, ConfigPath: cfgPath, Resume: resume}.Run(cmd.Context(), cmd.OutOrStdout())
}

// Execute runs the root command; called from main.
func Execute() error {
	return NewRootCommand().Execute()
}

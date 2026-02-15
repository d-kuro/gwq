package cmd

import (
	"github.com/d-kuro/gwq/internal/shell"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion and integration scripts",
	Long: `Generate shell completion scripts with optional shell integration.

When cd.launch_shell is set to false in your config, this also generates
a shell wrapper function that enables 'gwq cd' to change directory
in the current shell without launching a new shell.

  # bash (~/.bashrc)
  source <(gwq completion bash)

  # zsh (~/.zshrc)
  source <(gwq completion zsh)

  # fish (~/.config/fish/config.fish)
  gwq completion fish | source`,
}

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generate bash completion script",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true); err != nil {
			return err
		}
		if !viper.GetBool("cd.launch_shell") {
			return shell.WriteWrapper(cmd.OutOrStdout(), "bash", shell.TemplateData{CommandName: "gwq"})
		}
		return nil
	},
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate zsh completion script",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Root().GenZshCompletion(cmd.OutOrStdout()); err != nil {
			return err
		}
		if !viper.GetBool("cd.launch_shell") {
			return shell.WriteWrapper(cmd.OutOrStdout(), "zsh", shell.TemplateData{CommandName: "gwq"})
		}
		return nil
	},
}

var completionFishCmd = &cobra.Command{
	Use:   "fish",
	Short: "Generate fish completion script",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true); err != nil {
			return err
		}
		if !viper.GetBool("cd.launch_shell") {
			return shell.WriteWrapper(cmd.OutOrStdout(), "fish", shell.TemplateData{CommandName: "gwq"})
		}
		return nil
	},
}

func init() {
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)
	completionCmd.AddCommand(completionFishCmd)
	rootCmd.AddCommand(completionCmd)
}

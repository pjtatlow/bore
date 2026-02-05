package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pjtatlow/bore/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  "Validate or edit the bore configuration file.",
	}

	cmd.AddCommand(newConfigValidateCmd())
	cmd.AddCommand(newConfigEditCmd())
	cmd.AddCommand(newConfigPathCmd())

	return cmd
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		Long:  "Check the configuration file for errors.",
		RunE:  runConfigValidate,
	}
}

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration",
		Long:  "Open the configuration file in your $EDITOR.",
		RunE:  runConfigEdit,
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration path",
		Long:  "Print the path to the configuration file.",
		RunE:  runConfigPath,
	}
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Println("Configuration errors:")
		fmt.Println(err)
		return fmt.Errorf("configuration is invalid")
	}

	// Print summary
	fmt.Println("Configuration is valid")
	fmt.Printf("  Hosts: %d\n", len(cfg.Hosts))
	fmt.Printf("  Tunnels: %d\n", len(cfg.Tunnels))
	fmt.Printf("  Groups: %d\n", len(cfg.Groups))

	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	configDir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create default config if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultCfg := config.DefaultConfig()
		if err := defaultCfg.SaveTo(configPath); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		fmt.Printf("Created default config at %s\n", configPath)
	}

	// Get editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	// Open editor
	editorCmd := exec.Command(editor, configPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Validate after editing
	fmt.Println()
	return runConfigValidate(cmd, args)
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	configPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	fmt.Println(configPath)
	return nil
}

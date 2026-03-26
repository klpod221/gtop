package cmd

import (
	"fmt"
	"os"

	"gtop/internal/config"
	"gtop/internal/service"

	"github.com/spf13/cobra"
)

var (
	serviceConfigPath string
	serviceUserMode   bool
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the gtop systemd service",
	Long: `Manage the gtop systemd service.

The service respects enabled_agent and enabled_web fields in config.
At least one of them must be true to install the service.

Use '--user' to install as a user service instead of system-wide (no root required).`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gtop as a systemd service",
	Long: `Writes a systemd unit file and enables the gtop service.

Reads enabled_agent and enabled_web from config. At least one must be true.
By default installs system-wide (requires root). Use --user for user service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !serviceUserMode && os.Geteuid() != 0 {
			return fmt.Errorf("system-wide service installation requires root privileges.\nPlease run with 'sudo', or use '--user' to install for your user only.")
		}

		cfgPath := serviceConfigPath
		if cfgPath == "" {
			var err error
			cfgPath, err = config.DefaultConfigPath()
			if err != nil {
				return err
			}
			// When run via sudo, os.UserHomeDir() returns /root/, which is usually not what the user wants.
			// Let's resolve the actual user's config file if SUDO_USER is set.
			sudoUser := os.Getenv("SUDO_USER")
			if os.Geteuid() == 0 && sudoUser != "" && sudoUser != "root" {
				// We assume standard /home structure for Linux desktop users.
				cfgPath = fmt.Sprintf("/home/%s/.config/gtop/config.json", sudoUser)
			}
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if !cfg.EnabledAgent && !cfg.EnabledWeb {
			return fmt.Errorf("neither 'enabled_agent' nor 'enabled_web' is true in config.\n" +
				"Edit %s and set at least one to true before installing the service.", cfgPath)
		}

		fmt.Fprintf(os.Stdout, "Installing gtop service (agent=%v, web=%v)...\n",
			cfg.EnabledAgent, cfg.EnabledWeb)

		return service.Install(service.ServiceInstallOptions{
			UserMode: serviceUserMode,
			ConfigPath: cfgPath,
		})
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the gtop systemd service",
	Long:  `Stops, disables, and removes the gtop systemd unit file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !serviceUserMode && os.Geteuid() != 0 {
			return fmt.Errorf("system-wide service uninstallation requires root privileges.\nPlease run with 'sudo', or use '--user' to uninstall for your user only.")
		}
		return service.Uninstall(service.ServiceInstallOptions{
			UserMode: serviceUserMode,
		})
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check whether the gtop service is running",
	Long:  `Queries systemctl for the status of the gtop service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return service.Status(service.ServiceInstallOptions{
			UserMode: serviceUserMode,
		})
	},
}

func init() {
	serviceCmd.PersistentFlags().StringVar(&serviceConfigPath, "config", "", "Path to config.json (default: ~/.config/gtop/config.json)")
	serviceInstallCmd.Flags().BoolVar(&serviceUserMode, "user", false, "Install as a user service instead of system-wide")
	serviceUninstallCmd.Flags().BoolVar(&serviceUserMode, "user", false, "Uninstall user service instead of system-wide service")
	serviceStatusCmd.Flags().BoolVar(&serviceUserMode, "user", false, "Check user service status instead of system-wide")

	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)

	rootCmd.AddCommand(serviceCmd)
}

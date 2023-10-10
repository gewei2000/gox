package cmd

import (
	"fmt"
	"github.com/mitchellh/gox/pkg"
	"github.com/mitchellh/gox/pkg/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "print supported os/arch",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(
			"Supported OS/Arch combinations for %s are shown below. The \"default\"\n"+
				"boolean means that if you don't specify an OS/Arch, it will be\n"+
				"included by default. If it isn't a default OS/Arch, you must explicitly\n"+
				"specify that OS/Arch combo for Gox to use it.\n\n", pkg.GoVersion())
		for _, platform := range config.SupportedPlatforms() {
			fmt.Printf("%s\t(default: %v)\n", platform.String(), platform.Default)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

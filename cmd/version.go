package cmd

import (
	"github.com/mitchellh/gox/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version information",
	Run: func(cmd *cobra.Command, args []string) {
		print(version.AppVersion.Extended())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

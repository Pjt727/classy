package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "classy",
	Short: "classy is an application made to aggregate and serve class information from a variety of shools and terms",
	Long: `Classy can do one-off collections or act as a serivce
intermittently making collections and serving an api to observe data`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

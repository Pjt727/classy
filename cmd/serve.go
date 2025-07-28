package cmd

import (
	"log/slog"
	"os"

	logginghelpers "github.com/Pjt727/classy/data/logging-helpers"
	"github.com/Pjt727/classy/server"
	"github.com/spf13/cobra"
)

// serveapiCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Runs the api service",
	Long:  `Runs the api service`,
	Run: func(cmd *cobra.Command, args []string) {
		defaultLogger := slog.New(logginghelpers.NewHandler(os.Stdout, nil))
		slog.SetDefault(defaultLogger)
		api.Serve()
	},
}

func init() {
	appCmd.AddCommand(serveCmd)
}

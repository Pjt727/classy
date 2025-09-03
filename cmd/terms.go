package cmd

import (
	"context"
	"os"

	"log/slog"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data"
	"github.com/spf13/cobra"
)

// getSchoolTermsCmd represents the getSchoolTerms command
var termsCmd = &cobra.Command{
	Use:   "terms",
	Short: "Collects all school terms defined in orchestration",
	Long: `Collects and upserts in the db all schools for each of their terms 
as defined in orchestration`,
	Run: func(cmd *cobra.Command, args []string) {
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		logger := slog.With(
			slog.String("job", "getSchoolTerms"),
		)
		ctx := context.Background()
		dbPool, err := data.NewPool(ctx, false)
		if err != nil {
			logger.Error("Could not get database", "err", err)
			return
		}
		orchestrator := collection.GetDefaultOrchestrator(dbPool)

		logger.Info("Starting update on schools")
		orchestrator.UpsertAllSchools(ctx)
		logger.Info("Finished school update")

		logger.Info("Starting update on terms")
		orchestrator.UpsertAllTerms(ctx)
		logger.Info("Finished term update")
	},
}

func init() {
	collectCmd.AddCommand(termsCmd)
}

package cmd

import (
	"os"

	"log/slog"

	"github.com/Pjt727/classy/collection/projectpath"
	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"
)

// getSchoolTermsCmd represents the getSchoolTerms command
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Runs the up migrations",
	Long:  `Runs the up migrations and errors if there the up migrations cannot work`,
	Run: func(cmd *cobra.Command, args []string) {
		dbName := os.Getenv("DB_CONN")

		m, err := migrate.New("file://"+projectpath.Root+"/migrations", dbName)
		if err != nil {
			slog.Error("Could not set up migrations", "err", err)
			return
		}

		err = m.Up()
		if err != nil {
			slog.Error("Could not run up migrations", "err", err)
			return
		}
		slog.Info("Database has been synced with any up migraitons")
	},
}

func init() {
	appCmd.AddCommand(upCmd)
}

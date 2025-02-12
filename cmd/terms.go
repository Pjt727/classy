/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/Pjt727/classy/collection"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// getSchoolTermsCmd represents the getSchoolTerms command
var termsCmd = &cobra.Command{
	Use:   "terms",
	Short: "Collects all school terms defined in orchestration",
	Long: `Collects and upserts in the db all schools for each of their terms 
as defined in orchestration`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.TraceLevel)
		logger := log.WithFields(log.Fields{
			"job": "getSchoolTerms",
		})
		ctx := context.Background()
		orchestrator, err := collection.GetDefaultOrchestrator()
		if err != nil {
			logger.Error("Could not get orchestrator ", err)
			return
		}
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

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
var getSchoolTermsCmd = &cobra.Command{
	Use:   "getSchoolTerms",
	Short: "Gets all schools and terms defined in orchestration",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.TraceLevel)
		logger := log.WithFields(log.Fields{
			"job": "getSchoolTerms",
		})
		ctx := context.Background()
		logger.Info("Starting update on schools")
		collection.UpsertAllSchools(ctx)
		logger.Info("Finished school update")

		logger.Info("Starting update on terms")
		collection.UpsertAllTerms(ctx)
		logger.Info("Finished term update")
	},
}

func init() {
	rootCmd.AddCommand(getSchoolTermsCmd)

}

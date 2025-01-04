/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data/db"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// getSchoolCmd represents the getSchool command
var getSchoolCmd = &cobra.Command{
	Use:   "getSchool",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.TraceLevel)
		logger := log.WithFields(log.Fields{
			"job": "getSchool",
		})
		fmt.Println("getSchool called")
		schoolId, err := cmd.Flags().GetString("schoolid")
		if err != nil {
			logger.Error("invalid schoolid", err)
			return
		}
		termYear, err := cmd.Flags().GetInt("termyear")
		if err != nil {
			logger.Error("invalid termyear", err)
			return
		}
		termSeasonInput, err := cmd.Flags().GetString("termseason")
		if err != nil {
			logger.Error("invalid termyear", err)
			return
		}
		var termSeason db.SeasonEnum
		if err := termSeason.Scan(termSeasonInput); err != nil {
			logger.Error("Term season is invalid: ", err)
			return
		}
		term := db.Term{
			Year:   int32(termYear),
			Season: termSeason,
		}
		ctx := context.Background()
		logger.Infof("Starting update for school %s", schoolId)
		collection.UpdateAllSectionsOfSchool(ctx, schoolId, term)
		logger.Infof("Finished update for school %s", schoolId)
	},
}

func init() {
	rootCmd.AddCommand(getSchoolCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getSchoolCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getSchoolCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	now := time.Now()
	year := now.Year()
	month := now.Month()
	var season string
	switch month {
	// maybe change to a mapping that makes more sense for when a student would be planning for the next semester
	case time.December, time.January, time.February:
		season = string(db.SeasonEnumWinter)
	case time.March, time.April, time.May:
		season = string(db.SeasonEnumSpring)
	case time.June, time.July, time.August:
		season = string(db.SeasonEnumSummer)
	case time.September, time.October, time.November:
		season = string(db.SeasonEnumFall)
	default:
		panic("Missing month")
	}
	getSchoolCmd.Flags().String(
		"schoolid",
		"marist",
		"The school to be scraped (none for all of them)",
	)
	getSchoolCmd.Flags().String(
		"termseason",
		season,
		fmt.Sprintf(
			"The season to be scraped (%s, %s, %s, %s)",
			db.SeasonEnumWinter,
			db.SeasonEnumSpring,
			db.SeasonEnumSummer,
			db.SeasonEnumFall,
		),
	)
	getSchoolCmd.Flags().Int("termyear", year, "The year to be scraped (YYYY)")
}

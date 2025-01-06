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
var schoolCmd = &cobra.Command{
	Use:   "school",
	Short: "Collects information from a single school and term",
	Long: `Collects and upserts in the db information from a single school
and term (defaulting the current term) Updating the following data: sections, meeting times, courses, 
facualty members, and internal collection tables`,
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
	collectCmd.AddCommand(schoolCmd)

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
	schoolCmd.Flags().String(
		"schoolid",
		"marist",
		"The school to be collected (none for all of them)",
	)
	schoolCmd.Flags().String(
		"termseason",
		season,
		fmt.Sprintf(
			"The season to be collected (%s, %s, %s, %s)",
			db.SeasonEnumWinter,
			db.SeasonEnumSpring,
			db.SeasonEnumSummer,
			db.SeasonEnumFall,
		),
	)
	schoolCmd.Flags().Int("termyear", year, "The year to be collected (YYYY)")
}

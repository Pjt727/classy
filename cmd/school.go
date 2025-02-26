/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data"
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
			logger.Error("invalid termseason", err)
			return
		}
		schoolName, err := cmd.Flags().GetString("schoolname")
		if err != nil {
			logger.Error("invalid school name", err)
			return
		}
		var termSeason db.SeasonEnum
		if err := termSeason.Scan(termSeasonInput); err != nil {
			logger.Error("Term season is invalid: ", err)
			return
		}
		ctx := context.Background()
		dbPool, err := data.NewPool(ctx)
		if err != nil {
			logger.Error("Could not connect to db: ", err)
			return
		}
		orchestrator, err := collection.GetDefaultOrchestrator()
		if err != nil {
			logger.Error("Could create o: orchestrator", err)
			return
		}

		if schoolName == "" {
			school, ok := orchestrator.GetSchoolById(schoolId)
			if ok {
				schoolName = school.Name
			}
		}

		// update the terms for the school
		err = orchestrator.UpsertSchoolTerms(ctx, *logger, db.School{
			ID:   schoolId,
			Name: schoolName,
		})

		if err != nil {
			logger.Error("There was an error upserting school's terms: ", err)
			return
		}

		q := db.New(dbPool)
		termCollections, err := q.GetTermCollectionsForSchoolsSemester(ctx, db.GetTermCollectionsForSchoolsSemesterParams{
			SchoolID: schoolId,
			Year:     int32(termYear),
			Season:   termSeason,
		})
		if err != nil {
			logger.Error("There was an error getting terms: ", err)
			return
		}

		var termCollection db.TermCollection
		if len(termCollections) == 0 {
			logger.Errorf("There are no terms for %s %d", termSeason, termYear)
			return
		} else if len(termCollections) == 1 {
			termCollection = termCollections[0].TermCollection
		} else {
			for termCollection == (db.TermCollection{}) {
				fmt.Printf("There are multiple terms for %s and %d. Choose one:\n", termSeason, termYear)
				for i, getTermCollection := range termCollections {
					t := getTermCollection.TermCollection
					fmt.Printf("%d: %s %s\n", i+1, t.Name.String, t.ID)
				}
				var choice int32
				_, err = fmt.Scanln(&choice)
				choice-- // 1 based numbering
				if choice < 0 || len(termCollections) <= int(choice) {
					logger.Errorf("Invalid choice try again\n\n\n")
				} else {
					termCollection = termCollections[choice].TermCollection
				}
			}
		}

		logger.Infof("Starting update for school %s", schoolId)
		orchestrator.UpdateAllSectionsOfSchool(ctx, termCollection)
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
		"schoolname",
		"",
		"The name of the school to be collected",
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

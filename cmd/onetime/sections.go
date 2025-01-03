package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data/db"
)

func main() {
	now := time.Now()
	year := now.Year()
	month := now.Month()
	var season string
	switch month {
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
	school_id := flag.String(
		"schoolid",
		"marist",
		"The school to be scraped (none for all of them)",
	)
	termSeasonInput := flag.String(
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
	termYear := flag.Int("termyear", year, "The year to be scraped (YYYY)")
	var termSeason db.SeasonEnum
	if err := termSeason.Scan(termSeasonInput); err != nil {
		fmt.Println("Error scanning value:", err)
		return
	}
	term := db.Term{
		Year:   int32(*termYear),
		Season: termSeason,
	}
	ctx := context.Background()
	collection.UpdateAllSectionsOfSchool(ctx, *school_id, term)

}

package collection

import (
	"github.com/Pjt727/classy/internal/db"
	log "github.com/sirupsen/logrus"
)

func getTermLogger(school string, year int, term db.SeasonEnum) log.Entry {
	return *log.WithFields(log.Fields{
		"school": school,
		"year":   year,
		"term":   term,
	})
}

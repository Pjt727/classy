package collection

import (
	"github.com/Pjt727/classy/data/db"
	log "github.com/sirupsen/logrus"
)

type Service interface {
	// get the schools for this service
	ListValidSchools(logger log.Entry) []db.School

	// adds every section to database and returns the amount changed
	GetAllSections(logger log.Entry, school db.School, term db.Term) int

	// Is this term no longer getting updates and thus does not need be scraped
	IsTermDone(logger log.Entry, school db.School, term db.Term) bool

	UpdateTermsCollections(logger log.Entry, school db.School, term db.Term) []db.Ter
}

func getTermLogger(school string, term db.Term) log.Entry {
	return *log.WithFields(log.Fields{
		"school": school,
		"season": term.Season,
		"year":   term.Year,
	})
}

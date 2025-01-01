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

	// update the terms that for this service
	UpdateTermsCollections(logger log.Entry, school db.School, term db.Term) []db.TermCollection
}

func getTermLogger(school string, term db.Term) log.Entry {
	return *log.WithFields(log.Fields{
		"school": school,
		"season": term.Season,
		"year":   term.Year,
	})
}

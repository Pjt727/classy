package collection

import (
	"context"

	"github.com/Pjt727/classy/collection/services"
	"github.com/Pjt727/classy/data/db"
	log "github.com/sirupsen/logrus"
)

type Service interface {

	// get the schools for this service
	ListValidSchools(logger log.Entry, ctx context.Context, q *db.Queries) ([]db.School, error)

	// adds every section to database and returns the amount changed
	UpdateAllSections(
		logger log.Entry,
		ctx context.Context,
		q *db.Queries,
		school db.School,
		term db.Term,
	) (int, error)

	// update the terms that for this service
	GetTermCollections(
		logger log.Entry,
		ctx context.Context,
		q *db.Queries,
		school db.School,
		term db.Term,
	) ([]db.TermCollection, error)
}

var serviceEntries []Service

func init() {
	serviceEntries = []Service{services.Banner}
}

func getTermLogger(school string, term db.Term) log.Entry {
	return *log.WithFields(log.Fields{
		"school": school,
		"season": term.Season,
		"year":   term.Year,
	})
}

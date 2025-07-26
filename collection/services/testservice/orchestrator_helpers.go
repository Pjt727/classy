package testservice

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/data"
	classentry "github.com/Pjt727/classy/data/class-entry"
	"github.com/Pjt727/classy/data/db"
	dbhelpers "github.com/Pjt727/classy/data/testdb"
)

// runs the given service through an orchestrator using the first school or all schools with the first term collection
func RunServiceThroughTestOrchestrator(logger slog.Logger, service collection.Service, runAllSchools bool) error {
	err := dbhelpers.SetupTestDb()
	if err != nil {
		return err
	}

	ctx := context.Background()

	testDb, err := data.NewPool(ctx, true)
	if err != nil {
		return err
	}

	testOrchestrator, err := collection.CreateOrchestrator(
		[]collection.Service{service},
		&logger,
		testDb,
	)
	if err != nil {
		return err
	}

	schools, err := service.ListValidSchools(logger, ctx)
	if err != nil {
		return err
	}
	if len(schools) <= 0 {
		return fmt.Errorf("Services valid schools is empty")
	}
	if runAllSchools {
		for _, school := range schools {
			err := RunSchoolServiceTermClassUpdates(ctx, &logger, testOrchestrator, service, school)
			if err != nil {
				return err
			}
		}
	} else {
		err := RunSchoolServiceTermClassUpdates(ctx, &logger, testOrchestrator, service, schools[0])
		if err != nil {
			return err
		}
	}

	return nil
}

func RunSchoolServiceTermClassUpdates(
	ctx context.Context,
	logger *slog.Logger,
	testOrchestrator collection.Orchestrator,
	service collection.Service,
	school classentry.School,
) error {
	err := testOrchestrator.UpsertSchoolTermsWithService(
		ctx,
		logger,
		school,
		service.GetName(),
	)
	if err != nil {
		return err
	}
	termCollections, err := service.GetTermCollections(*logger, ctx, school)
	if err != nil {
		return err
	}
	if len(termCollections) <= 0 {
		return fmt.Errorf("Services term collections for %v is empty", school)
	}
	firstTermCollection := termCollections[0]
	err = testOrchestrator.UpdateAllSectionsOfSchoolWithService(
		ctx,
		db.TermCollection{
			ID:              firstTermCollection.ID,
			Name:            firstTermCollection.Name,
			SchoolID:        school.ID,
			Year:            firstTermCollection.Term.Year,
			Season:          firstTermCollection.Term.Season,
			StillCollecting: firstTermCollection.StillCollecting,
		},
		*logger,
		service.GetName(),
		true,
	)
	if err != nil {
		return err
	}

	return nil
}

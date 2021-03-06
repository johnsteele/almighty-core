package controller

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/almighty/almighty-core/app"
	"github.com/almighty/almighty-core/app/test"
	config "github.com/almighty/almighty-core/configuration"
	"github.com/almighty/almighty-core/gormapplication"
	"github.com/almighty/almighty-core/gormsupport/cleaner"
	"github.com/almighty/almighty-core/resource"
	"github.com/almighty/almighty-core/rest"
	"github.com/almighty/almighty-core/space"

	"github.com/goadesign/goa"
)

var trQueryTestConfiguration *config.ConfigurationData

func init() {
	var err error
	trQueryTestConfiguration, err = config.GetConfigurationData()
	if err != nil {
		panic(fmt.Errorf("Failed to setup the configuration: %s", err.Error()))
	}
}

func TestCreateTrackerQuery(t *testing.T) {
	resource.Require(t, resource.Database)
	defer cleaner.DeleteCreatedEntities(DB)()
	controller := TrackerController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}
	payload := app.CreateTrackerAlternatePayload{
		URL:  "http://api.github.com",
		Type: "github",
	}
	_, result := test.CreateTrackerCreated(t, nil, nil, &controller, &payload)
	t.Log(result.ID)
	tqController := TrackerqueryController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}

	tqpayload := getCreateTrackerQueryPayload(result.ID)

	_, tqresult := test.CreateTrackerqueryCreated(t, nil, nil, &tqController, &tqpayload)
	t.Log(tqresult)
	if tqresult.ID == "" {
		t.Error("no id")
	}
}

func TestGetTrackerQuery(t *testing.T) {
	resource.Require(t, resource.Database)
	defer cleaner.DeleteCreatedEntities(DB)()
	controller := TrackerController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}
	payload := app.CreateTrackerAlternatePayload{
		URL:  "http://api.github.com",
		Type: "github",
	}
	_, result := test.CreateTrackerCreated(t, nil, nil, &controller, &payload)

	tqController := TrackerqueryController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}

	tqpayload := getCreateTrackerQueryPayload(result.ID)

	fmt.Printf("tq payload %#v", tqpayload)
	_, tqresult := test.CreateTrackerqueryCreated(t, nil, nil, &tqController, &tqpayload)
	test.ShowTrackerqueryOK(t, nil, nil, &tqController, tqresult.ID)
	_, tqr := test.ShowTrackerqueryOK(t, nil, nil, &tqController, tqresult.ID)

	if tqr == nil {
		t.Fatalf("Tracker Query '%s' not present", tqresult.ID)
	}
	if tqr.ID != tqresult.ID {
		t.Errorf("Id should be %s, but is %s", tqresult.ID, tqr.ID)
	}
}

func TestUpdateTrackerQuery(t *testing.T) {
	resource.Require(t, resource.Database)
	defer cleaner.DeleteCreatedEntities(DB)()
	controller := TrackerController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}
	payload := app.CreateTrackerAlternatePayload{
		URL:  "http://api.github.com",
		Type: "github",
	}
	_, result := test.CreateTrackerCreated(t, nil, nil, &controller, &payload)

	tqController := TrackerqueryController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}

	tqpayload := getCreateTrackerQueryPayload(result.ID)

	_, tqresult := test.CreateTrackerqueryCreated(t, nil, nil, &tqController, &tqpayload)
	test.ShowTrackerqueryOK(t, nil, nil, &tqController, tqresult.ID)
	_, tqr := test.ShowTrackerqueryOK(t, nil, nil, &tqController, tqresult.ID)

	if tqr == nil {
		t.Fatalf("Tracker Query '%s' not present", tqresult.ID)
	}
	if tqr.ID != tqresult.ID {
		t.Errorf("Id should be %s, but is %s", tqresult.ID, tqr.ID)
	}

	reqLong := &goa.RequestData{
		Request: &http.Request{Host: "api.service.domain.org"},
	}
	spaceSelfURL := rest.AbsoluteURL(reqLong, app.SpaceHref(space.SystemSpace.String()))
	payload2 := app.UpdateTrackerQueryAlternatePayload{
		Query:     tqr.Query,
		Schedule:  tqr.Schedule,
		TrackerID: result.ID,
		Relationships: &app.TrackerQueryRelationships{
			Space: space.NewSpaceRelation(space.SystemSpace, spaceSelfURL),
		},
	}

	_, updated := test.UpdateTrackerqueryOK(t, nil, nil, &tqController, tqr.ID, &payload2)

	if updated.ID != tqresult.ID {
		t.Errorf("Id has changed from %s to %s", tqresult.ID, updated.ID)
	}
	if updated.Query != tqresult.Query {
		t.Errorf("Query has changed from %s to %s", tqresult.Query, updated.Query)
	}
	if updated.Schedule != tqresult.Schedule {
		t.Errorf("Type has changed has from %s to %s", tqresult.Schedule, updated.Schedule)
	}
}

// This test ensures that List does not return NIL items.
func TestTrackerQueryListItemsNotNil(t *testing.T) {
	resource.Require(t, resource.Database)
	defer cleaner.DeleteCreatedEntities(DB)()
	controller := TrackerController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}
	payload := app.CreateTrackerAlternatePayload{
		URL:  "http://api.github.com",
		Type: "github",
	}
	_, result := test.CreateTrackerCreated(t, nil, nil, &controller, &payload)
	t.Log(result.ID)
	tqController := TrackerqueryController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}

	tqpayload := getCreateTrackerQueryPayload(result.ID)

	test.CreateTrackerqueryCreated(t, nil, nil, &tqController, &tqpayload)
	test.CreateTrackerqueryCreated(t, nil, nil, &tqController, &tqpayload)

	_, list := test.ListTrackerqueryOK(t, nil, nil, &tqController)
	for _, tq := range list {
		if tq == nil {
			t.Error("Returned Tracker Query found nil")
		}
	}
}

// This test ensures that ID returned by Show is valid.
// refer : https://github.com/almighty/almighty-core/issues/189
func TestCreateTrackerQueryValidId(t *testing.T) {
	resource.Require(t, resource.Database)
	defer cleaner.DeleteCreatedEntities(DB)()
	controller := TrackerController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}
	payload := app.CreateTrackerAlternatePayload{
		URL:  "http://api.github.com",
		Type: "github",
	}
	_, result := test.CreateTrackerCreated(t, nil, nil, &controller, &payload)
	t.Log(result.ID)
	tqController := TrackerqueryController{Controller: nil, db: gormapplication.NewGormDB(DB), scheduler: RwiScheduler, configuration: configuration}

	tqpayload := getCreateTrackerQueryPayload(result.ID)

	_, trackerquery := test.CreateTrackerqueryCreated(t, nil, nil, &tqController, &tqpayload)
	_, created := test.ShowTrackerqueryOK(t, nil, nil, &tqController, trackerquery.ID)
	if created != nil && created.ID != trackerquery.ID {
		t.Error("Failed because fetched Tracker query not same as requested. Found: ", trackerquery.ID, " Expected, ", created.ID)
	}
}

func getCreateTrackerQueryPayload(trackerID string) app.CreateTrackerQueryAlternatePayload {
	reqLong := &goa.RequestData{
		Request: &http.Request{Host: "api.service.domain.org"},
	}
	spaceSelfURL := rest.AbsoluteURL(reqLong, app.SpaceHref(space.SystemSpace.String()))
	return app.CreateTrackerQueryAlternatePayload{
		Query:     "is:open is:issue user:arquillian author:aslakknutsen",
		Schedule:  "15 * * * * *",
		TrackerID: trackerID,
		Relationships: &app.TrackerQueryRelationships{
			Space: space.NewSpaceRelation(space.SystemSpace, spaceSelfURL),
		},
	}
}

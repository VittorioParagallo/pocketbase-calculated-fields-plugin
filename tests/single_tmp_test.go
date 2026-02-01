package tests

import (
	"net/http"

	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

func TestComputedFieldsCreateFxCalculations2(t *testing.T) {
	//t.Parallel()
	autApp, _ := tests.NewTestApp("../tests/pb_data")
	superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}
	scenarios := []tests.ApiScenario{

		{
			Name:   "Calcolo con funzioni aggregate su valori da record",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "gregaterecords1",
  "formula": "sum([yysba8o7a6773c3, c03u7o5plc4hucf, f73xufl8v7lyapj])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_aggregate_records",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"gregaterecords1"`,
				`"formula":"sum([yysba8o7a6773c3, c03u7o5plc4hucf, f73xufl8v7lyapj])"`,
				`"value":29`, // 5 + 6 + 18
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

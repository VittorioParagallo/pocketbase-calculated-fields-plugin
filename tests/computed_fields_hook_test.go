package tests

import (
	"encoding/json"
	"fmt"
	"myapp/hooks"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestComputedFieldsCreateFxCalculations(t *testing.T) {
	//t.Parallel()
	autApp, _ := tests.NewTestApp("../pb_data")

superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}
	
	scenarios := []tests.ApiScenario{

		{
			Name:   "Calcolo formula costante",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "testcoll_abc123_constant",
  "formula": "42 + 8",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_testcoll_abc123_constant",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"testcoll_abc123_constant"`,
				`"formula":"42 + 8"`,
				`"value":50`,
			},
		},

		{
			Name:   "Calcolo formula da campo esistente",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_test_from_db_dependent",
  "formula": "booking_queue_queue_b00000002_act + 2",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_test_from_db_dependent",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_test_from_db_dependent"`,
				`"formula":"booking_queue_queue_b00000002_act + 2"`,
				`"value":2`,
			},
		},
		{
			Name:   "Errore: self-reference esplicita su se stesso",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_self_ref_node_value",
  "formula": "booking_queue_self_ref_node_value + 1",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_self_ref_node_value",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1002"`,
			},
		},

		{
			Name:   "Errore: formula malformata (sintassi non valida)",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_syntax_error_node_value",
  "formula": "1 + * 2",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_syntax_error_node_value",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1004"`,
			},
		},

		{
			Name:   "Errore: riferimento a campo o record inesistente",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_missing_ref_test_value",
  "formula": "booking_queue_nonexistent_record_act + 10",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_missing_ref_test_value",
  "owner_field": "cf"
}`),
			Headers:         superAuthHeader,
			TestAppFactory:  setupTestApp,
			ExpectedStatus:  400,
			ExpectedContent: []string{`"code":"1007"`},
		},

		{
			Name:   "Errore: somma tra numero e stringa",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_b00000002_invalidsum",
  "formula": "booking_queue_queue_b00000002_act + \"abc\"",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_b00000002_invalidsum",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"value":"#VALUE!"`,
				`"error":"Tipo non compatibile nell'operazione"`,
			},
		},
		{
			Name:   "Errore: divisione per zero",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_test_from_db_divzero",
  "formula": "10 / 0",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_test_from_db_divzero",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"value":"#DIV/0!"`,
				`"error":"Divisione per zero o risultato infinito"`,
			},
		},
		{
			Name:   "Errore: formula con caratteri speciali o unicode non validi",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_test_from_db_unicode",
  "formula": "booking_queue_queue_b00000002_act + 💥",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_test_from_db_unicode",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1004"`, // errore di parsing
			},
		},

		{
			Name:   "Dipendenza incrociata valida tra due calcolati",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_c00000000_cross_dep_test",
  "formula": "booking_queue_queue_b00000002_min + booking_queue_queue_c00000000_max",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_c00000000_cross_dep_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_c00000000_cross_dep_test"`,
				`"formula":"booking_queue_queue_b00000002_min + booking_queue_queue_c00000000_max"`,
				`"value":24`,
			},
		},
		{
			Name:   "Dipendenza incrociata su tre livelli",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_triple_dep_test",
  "formula": "booking_queue_queue_a00000001_min + booking_queue_queue_c00000000_max",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_triple_dep_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_triple_dep_test"`,
				`"formula":"booking_queue_queue_a00000001_min + booking_queue_queue_c00000000_max"`,
				`"value":23`,
			},
		},

		{
			Name:   "Errore: somma con campo null",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_null_dep_test",
  "formula": "1 + foo_bar_baz",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_null_dep_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"error":"Valore non disponibile (null) in operazione"`,
				`"value":"#N/A"`,
			},
		},

		{
			Name:   "Errore: uso di funzione non supportata",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_unsupported_func",
  "formula": "exec(\"rm -rf /\")",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_unsupported_func",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"value":"#NAME?"`, // errore di valutazione formula
			},
		},

		{
			Name:   "Errore: funzione con argomenti non validi",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_invalid_args",
  "formula": "sum()",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_invalid_args",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1004"`, // errore in fase di parsing per argomenti non validi
			},
		},
		{
			Name:   "Errore: formula con caratteri non ASCII o simboli non validi",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_invalid_chars",
  "formula": "booking_queue_queue_a00000001_min + 🚀",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_invalid_chars",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1004"`, // parsing error atteso per carattere non valido
			},
		},

		{
			Name:   "Errore: formula che tenta operazione aritmetica su campo array",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_array_misuse",
  "formula": "booking_queue_v7tex6i3v4w6hs0_linked_bookings + 1",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_array_misuse",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"error":"Tipo non compatibile nell'operazione"`, // tipo errato: somma tra array e numero
				`"value":"#VALUE!"`,
			},
		},
		{
			Name:   "Calcolo con funzione len() su campo linked_bookings",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_len_test",
  "formula": "len(booking_queue_v7tex6i3v4w6hs0_linked_bookings)",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_len_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_len_test"`,
				`"formula":"len(booking_queue_v7tex6i3v4w6hs0_linked_bookings)"`,
				`"value":3`, // perché nel tuo dump ci sono 3 elementi
			},
		},

		{
			Name:   "Calcolo con funzione any() su campo linked_bookings",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_contains_test",
  "formula": "\"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_contains_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_contains_test"`,
				`"formula":"\"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings"`,
				`"value":true`,
			},
		},
		{
			Name:   "Calcolo con if { ... } else { ... } su campo linked_bookings",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_if_test",
  "formula": "if \"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings { 100 } else { 0 }",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_if_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_if_test"`,
				`"formula":"if \"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings { 100 } else { 0 }"`,
				`"value":100`,
			},
		},
		{
			Name:   "Calcolo con operatore ternario su campo linked_bookings",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_ternary_test",
  "formula": "\"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings ? 100 : 0",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_ternary_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_ternary_test"`,
				`"formula":"\"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings ? 100 : 0"`,
				`"value":100`,
			},
		},
		{
			Name:   "Calcolo con funzioni min, max e sum su array numerico",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_aggregate_funcs",
  "formula": "sum([1, 2, 3]) + min([5, 2, 9]) + max([4, 8, 1])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_aggregate_funcs",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_aggregate_funcs"`,
				`"formula":"sum([1, 2, 3]) + min([5, 2, 9]) + max([4, 8, 1])"`,
				`"value":16`,
			},
		},

		{
			Name:   "Calcolo con funzioni aggregate su valori da record",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_aggregate_records",
  "formula": "sum([booking_queue_queue_a00000001_min, booking_queue_queue_b00000002_min, booking_queue_queue_c00000000_max])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_aggregate_records",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_aggregate_records"`,
				`"formula":"sum([booking_queue_queue_a00000001_min, booking_queue_queue_b00000002_min, booking_queue_queue_c00000000_max])"`,
				`"value":29`, // 5 + 6 + 18
			},
		},
		{
			Name:   "Calcolo con max() tra due record",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_test_max",
  "formula": "max(booking_queue_queue_a00000001_min, booking_queue_queue_b00000002_min)",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_test_max",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_test_max"`,
				`"value":6`,
			},
		},
		{
			Name:   "Calcolo con sum(len(), costante)",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_sum_len_constant",
  "formula": "sum([len(booking_queue_v7tex6i3v4w6hs0_linked_bookings), 7])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_sum_len_constant",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_sum_len_constant"`,
				`"formula":"sum([len(booking_queue_v7tex6i3v4w6hs0_linked_bookings), 7])"`,
				`"value":10`, // perché linked_bookings ha 3 elementi
			},
		},
		{
			Name:   "Calcolo con sum([max(), costante])",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_sum_max_test",
  "formula": "sum([max(3, 7), 5])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_sum_max_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_sum_max_test"`,
				`"formula":"sum([max(3, 7), 5])"`,
				`"value":12`,
			},
		},

		{
			Name:   "Calcolo con sum([min(), campo esistente])",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_sum_min_test",
  "formula": "sum([min(2, 4), booking_queue_queue_b00000002_act])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_sum_min_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_sum_min_test"`,
				`"formula":"sum([min(2, 4), booking_queue_queue_b00000002_act])"`,
				`"value":2`, // 2 (min) + 1 (act value from queue_b00000002)
			},
		},
		{
			Name:   "Calcolo con sum([if(condizione){...}else{...}, costante])",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_sum_if_block_test",
  "formula": "sum([if booking_queue_queue_a00000001_min > 3 { 10 } else { 0 }, 5])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_sum_if_block_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_sum_if_block_test"`,
				`"formula":"sum([if booking_queue_queue_a00000001_min \u003e 3 { 10 } else { 0 }, 5])"`, `"value":15`, // queue_a00000001.min è 5 > 3 → if produce 10 → 10 + 5 = 15
			},
		},

		{
			Name:   "Calcolo con sum e formule annidate (len, max, if, costante)",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_d00000001_nested_formula_test",
  "formula": "sum([len(booking_queue_v7tex6i3v4w6hs0_linked_bookings),max(booking_queue_queue_a00000001_min,booking_queue_queue_b00000002_min),10,if booking_queue_queue_c00000000_max > 10 { 100 } else { 0 }])",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_d00000001_nested_formula_test",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_d00000001_nested_formula_test"`,
				`"value":119`, // 3 (len) + 6 (max) + 10 + 100
			},
		},
		{
			Name:   "Calcolo concatenazione stringhe da più record",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_concat1234_min",
  "formula": "string(booking_queue_queue_a00000001_min) + \"‑\" + string(booking_queue_queue_b00000002_min)",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_concat1234_min",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_concat1234_min"`,
				`"value":"5‑6"`,
			},
		},
		{
			Name:   "Calcolo data + 3 giorni",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
  "id": "booking_queue_queue_date_plus3_date",
  "formula": "date(booking_queue_queue_date_test_date) + duration(\"72h\")",
  "owner_collection": "cf_owner_pool",
  "owner_row": "cf_owner_booking_queue_queue_date_plus3_date",
  "owner_field": "cf"
}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_date_plus3_date"`,
				`"formula":"date(booking_queue_queue_date_test_date) + duration(\"72h\")"`,
				`"value":"2025-12-28T15:30:00Z"`, // 25 Dic 2025 + 3 giorni
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

// -------------------------------------------------------------------

func TestComputedFieldsUpdateFxCalculations(t *testing.T) {
	//t.Parallel()
	autApp, _ := tests.NewTestApp("../pb_data")
	
superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	scenarios := []tests.ApiScenario{

		{
			Name:   "Update formula valida su record esistente booking_queue_queue_zb3456789_min",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zb3456789_min",
			Body: strings.NewReader(`{
		"formula": "booking_queue_queue_zc4567890_min + 10"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_zb3456789_min"`,
				`"formula":"booking_queue_queue_zc4567890_min + 10"`,
				`"value":14`, // queue_zc4567890.min = 4
			},
		}, {
			Name:   "Errore: Update con formula malformata",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zb3456789_min",
			Body: strings.NewReader(`{
		"formula": "1 + * 2"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1004"`,
			},
		},
		{
			Name:   "Update e propagazione a campo dipendente",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zb3456789_min",
			Body: strings.NewReader(`{
		"formula": "2"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_zb3456789_min"`,
				`"formula":"2"`,
				`"value":2`,
			},
		},
		{
			Name:   "Update nodo base con propagazione su catena profonda",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_min",
			Body: strings.NewReader(`{
		"formula": "6"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_a00000001_min"`,
				`"formula":"6"`,
				`"value":6`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				checkFormulaUpdate(t, app,
					"booking_queue_queue_b00000002_min",
					"booking_queue_queue_a00000001_min + 1",
					"7",
					"")
				checkFormulaUpdate(t, app,
					"booking_queue_queue_c00000000_min",
					"booking_queue_queue_b00000002_min+ 2",
					"9",
					"")
			},
		}, {
			Name:   "Update formula rimuovendo un riferimento e verifica depends_on",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_c00000000_min",
			Body: strings.NewReader(`{
        "formula": "booking_queue_queue_b00000002_min + 4"
    }`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_c00000000_min"`,
				`"formula":"booking_queue_queue_b00000002_min + 4"`,
				`"value":10`,
				`"depends_on":["booking_queue_queue_b00000002_min"]`,
			},
		},
		{
			Name:   "Update con aggiunta nuova dipendenza",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_root00001_min",
			Body: strings.NewReader(`{
		"formula": "booking_queue_queue_a00000001_min + 1"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_root00001_min"`,
				`"formula":"booking_queue_queue_a00000001_min + 1"`,
				`"value":6`,
				`"depends_on":["booking_queue_queue_a00000001_min"]`,
			},
		},
		{
			Name:           "Errore su ciclo indiretto tra record",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_min",
			Body:           strings.NewReader(`{"formula": "booking_queue_queue_c00000000_min + 1"}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1003"`,
			},
		},

		{
			Name:           "Errore su funzione con tipo errato (sum su int e non con array come argomento)",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_zc4567890_min",
			Body:           strings.NewReader(`{"formula": "sum(5)"}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1004"`,
			},
		}, {
			Name:   "Errore su formula con identificatore inesistente",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zc4567890_min",
			Body: strings.NewReader(`{
		"formula": "not_existing_record.min + 1"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1007"`,
			},
		}, {
			Name:   "Errore su formula con dipendenza diretta da se stesso",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zc4567890_min",
			Body: strings.NewReader(`{
		"formula": "booking_queue_queue_zc4567890_min + 1"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{

				`"code":"1002"`,
			},
		}, {
			Name:   "Update rimuove tutte le dipendenze (formula statica)",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zb3456789_min",
			Body: strings.NewReader(`{
		"formula": "42"
	}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_zb3456789_min"`,
				`"formula":"42"`,
				`"depends_on":[]`,
				`"value":42`,
			},
		},
		{
			Name:           "Fix formula clears previous error and updates value",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_error_reset_test_min",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			Body:           strings.NewReader(`{"formula": "5 + 1"}`),
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_error_reset_test_min"`,
				`"formula":"5 + 1"`,
				`"depends_on":[]`,
				`"value":6`,
				`"error":""`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				checkFormulaUpdate(t, app,
					"booking_queue_error_reset_test_min",
					"5 + 1",
					"6",
					"")
			},
		},
		{
			Name:           "Manual #REF! formula is preserved and error is reset",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_b89101213_min",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			Body:           strings.NewReader(`{"formula": "#REF! + 1"}`),
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_b89101213_min"`,
				`"formula":"#REF! + 1"`,
				`"value":"#REF!"`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				record, err := app.FindRecordById("calculated_fields", "booking_queue_queue_b89101213_min")
				if err != nil {
					t.Fatalf("Errore nel recupero del record aggiornato: %v", err)
				}

				if record.GetString("formula") != "#REF! + 1" {
					t.Errorf("Formula errata. Attesa: \"#REF! + 1\", Ottenuta: %s", record.GetString("formula"))

				}

				if record.GetString("value") != "\"#REF!\"" {
					t.Errorf("Valore errato. Atteso: \"#REF!\", Ottenuto: %s", record.GetString("value"))
				}

				if record.GetString("error") != "Formula contains reference to missing node (#REF!)" {
					t.Errorf("Errore errato. Atteso vuoto, Ottenuto: %s", record.GetString("error"))
				}
			},
		},

		{
			Name:           "Update formula fixes previous syntax error (#REF! state)",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_error_reset_test_min",
			Body:           strings.NewReader(`{"formula": "5 + 1"}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_error_reset_test_min"`,
				`"formula":"5 + 1"`,
				`"value":6`,
				`"error":""`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				checkFormulaUpdate(t, app,
					"booking_queue_error_reset_test_min",
					"5 + 1",
					"6",
					"",
				)
			},
		},
		{
			Name:           "Update formula fixes division by zero error",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/rec_divzero_err",
			Body:           strings.NewReader(`{"formula": "10 / 2"}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"rec_divzero_err"`,
				`"formula":"10 / 2"`,
				`"value":5`,
				`"error":""`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				checkFormulaUpdate(t, app,
					"rec_divzero_err",
					"10 / 2",
					"5",
					"",
				)
			},
		},
		{
			Name:           "Update formula fixes invalid type operation (#VALUE!)",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/inval_types_sum",
			Body:           strings.NewReader(`{"formula": "1 + 2"}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"inval_types_sum"`,
				`"formula":"1 + 2"`,
				`"value":3`,
				`"error":""`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				checkFormulaUpdate(t, app,
					"inval_types_sum",
					"1 + 2",
					"3",
					"",
				)
			},
		},
		{
			Name:           "Update not prsisted if #REF! error not resolved",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/bad_refer_to_fx",
			Body:           strings.NewReader(`{"formula": "missing_ref + 3"}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1007"`,
				`"message":"Formula evaluation error: referenced variable not found: [missing_ref]."`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				checkFormulaUpdate(t, app,
					"bad_refer_to_fx",
					"missing_ref + 2",
					"\"#REF!\"",
					"Reference to deleted node",
				)
			},
		},

		{
			Name:           "Solving rec_divzero_err (A) propagates to correct dep_from_divzero_rec value",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/rec_divzero_err",
			Body:           strings.NewReader(`{"formula":"10 / 2"}`),
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"formula":"10 / 2"`,
				`"value":5`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				checkFormulaUpdate(t, app,
					"dep_from_divzero_rec",
					"4 + rec_divzero_err",
					"9",
					"")
			},
		},
		{
			Name:           "Update A propaga ad A, B, C e D correttamente con D che dipende da A e da C",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_min",
			Body:           strings.NewReader(`{"formula": "8"}`), // esempio: cambiamo A a 8
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_queue_a00000001_min"`,
				`"formula":"8"`,
				`"value":8`, // A diventa 8
				`"id":"booking_queue_queue_b00000002_min"`,
				`"value":9`, // B = A + 1 => 9
				`"id":"booking_queue_queue_c00000000_min"`,
				`"value":11`, // C = B + 2 => 11
				`"id":"booking_queue_queue_d00000001_min"`,
				`"value":26`, // D = A + C_max (?) → se C_max è per esempio 11, D = 8 + 11 = 19
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// qui puoi fare checkFormulaUpdate per ciascun nodo: A, B, C, D
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestComputedFieldsDeleteFxUpdatesDependents(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}
	scenarios := []tests.ApiScenario{
		{
			Name:           "tries to delete without auth",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_b00000002_min",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 204,
		},
		{
			Name:           "Delete node with dependents triggers formula replacement",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_b00000002_min",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 204,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// Verifica aggiornamenti su dipendente
				checkFormulaUpdate(t, app, "booking_queue_queue_c00000000_min", "#REF! + 2", "\"#REF!\"", "Formula contains reference to missing node (#REF!)")

				// Verifica che il record eliminato non esista più
				_, err := app.FindRecordById("calculated_fields", "booking_queue_queue_b00000002_min")
				if err == nil {
					t.Fatalf("Expected record booking_queue_queue_b00000002_min to be deleted, but it still exists.")
				}
			},
		},
		{
			Name:           "Delete base node updates chain of dependents",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_min",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 204,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// Dipendente diretto: formula modificata
				checkFormulaUpdate(t, app,
					"booking_queue_queue_b00000002_min",
					"#REF! + 1",
					"\"#REF!\"",
					"Formula contains reference to missing node (#REF!)")

				// Dipendente transitivo: formula invariata, ma valore aggiornato
				checkFormulaUpdate(t, app,
					"booking_queue_queue_c00000000_min",
					"booking_queue_queue_b00000002_min+ 2",
					"\"#REF!\"",
					"Reference to deleted node")

				// Verifica che il nodo eliminato non esista più
				_, err := app.FindRecordById("calculated_fields", "booking_queue_queue_a00000001_min")
				if err == nil {
					t.Fatalf("Expected record booking_queue_queue_a00000001_min to be deleted, but it still exists")
				}
			},
		},

		{
			Name:           "Delete node with single dependent updates that dependent",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_b00000002_min",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 204,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// Dipendente diretto
				checkFormulaUpdate(t, app,
					"booking_queue_queue_c00000000_min",
					"#REF! + 2",
					"\"#REF!\"",
					"Formula contains reference to missing node (#REF!)")

				// Verifica che il nodo eliminato non esista più
				_, err := app.FindRecordById("calculated_fields", "booking_queue_queue_b00000002_min")
				if err == nil {
					t.Fatalf("Expected record booking_queue_queue_b00000002_min to be deleted, but it still exists")
				}
			},
		},
		{
			Name:           "Delete base node in deep chain updates all dependents correctly",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_min",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 204,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// Diretto
				checkFormulaUpdate(t, app,
					"booking_queue_queue_b00000002_min",
					"#REF! + 1",
					"\"#REF!\"",
					"Formula contains reference to missing node (#REF!)")

				// Transitivo
				checkFormulaUpdate(t, app,
					"booking_queue_queue_c00000000_min",
					"booking_queue_queue_b00000002_min + 2", // formula resta invariata
					"\"#REF!\"",
					"Reference to deleted node")

				// Verifica eliminazione del nodo originale
				_, err := app.FindRecordById("calculated_fields", "booking_queue_queue_a00000001_min")
				if err == nil {
					t.Fatalf("Expected record booking_queue_queue_a00000001_min to be deleted, but it still exists")
				}
			},
		},

		{
			Name:           "Delete base node in 4‑level dependency chain updates all dependents correctly",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_max",
			Headers:        superAuthHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 204,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// 1° livello diretto (dipende da queue_a00000001.max)
				checkFormulaUpdate(t, app,
					"booking_queue_queue_b00000002_max",
					"#REF! + 5",
					"\"#REF!\"",
					"Formula contains reference to missing node (#REF!)")

				// 2° livello transitivo (dipende da b00000002.max)
				checkFormulaUpdate(t, app,
					"booking_queue_queue_c00000000_max",
					"booking_queue_queue_b00000002_max + 3",
					"\"#REF!\"",
					"Reference to deleted node")

				// 3° livello transitivo (dipende da c00000000.max)
				checkFormulaUpdate(t, app,
					"booking_queue_queue_d00000001_max",
					"booking_queue_queue_c00000000_max + 7",
					"\"#REF!\"",
					"Reference to deleted node")

				// Verifica che il nodo base sia cancellato
				_, err := app.FindRecordById("calculated_fields", "booking_queue_queue_a00000001_max")
				if err == nil {
					t.Fatalf("Expected record booking_queue_queue_a00000001_max to be deleted, but it still exists")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func checkFormulaUpdate(t testing.TB, app *tests.TestApp, id, expectedFormula, expectedValue, expectedError string) {
	t.Helper()

	record, err := app.FindRecordById("calculated_fields", id)
	if err != nil {
		t.Fatalf("checkFormulaUpdate: Errore nel recupero del record %s: %v", id, err)
	}

	if strings.ReplaceAll(record.GetString("formula"), " ", "") != strings.ReplaceAll(expectedFormula, " ", "") {
		t.Errorf("Formula errata per %s. Attesa: %s, Ottenuta: %s", id, expectedFormula, record.GetString("formula"))
	}

	if record.GetString("value") != expectedValue {
		t.Errorf("Valore errato per %s. Atteso: %s, Ottenuto: %s", id, expectedValue, record.GetString("value"))
	}

	if record.GetString("error") != expectedError {
		t.Errorf("Errore errato per %s. Atteso: %s, Ottenuto: %s", id, expectedError, record.GetString("error"))
	}
}

// -------------------------------------------------------------------

func setupTestApp(t testing.TB) *tests.TestApp {

	testApp, err := tests.NewTestApp("../pb_data")
	if err != nil {
		t.Fatal(err)
	}
	// reset globale hook prima di bindare
	hooks.BindCalculatedFieldsHooks(testApp)
	return testApp
}

func getAuthToken(app *tests.TestApp, collection, username string) string {

	//defer app.Cleanup()

	record, _ := app.FindFirstRecordByData(collection, "username", username)

	token, _ := record.NewAuthToken()

	return token
}

func TestResolveDepsAndTxSave(t *testing.T) {
	formulas := []struct {
		formula string
		want    []string
	}{
		{"foo + bar", []string{"foo", "bar"}},
		{"booking_queue1_rec2_max+7", []string{"booking_queue1_rec2_max"}},
		{"len(my_var) + other_var", []string{"my_var", "other_var"}},
		{"exec(\"rm -rf /\")", []string{}},
		{"exec(\"rm -rf /\") + foo", []string{"foo"}},
		{"#REF! + foo", []string{"foo"}},
		{"len(trim(foo)) + bar", []string{"foo", "bar"}},
		{"foo.bar + baz.qux", []string{"foo", "baz"}},
		{"foo > 0 && bar < 10", []string{"foo", "bar"}},
		{"foo[0] + bar[1:2]", []string{"foo", "bar"}},
		{"foo > 0 ? bar : baz", []string{"foo", "bar", "baz"}},
		{`{"a": foo, "b": bar}`, []string{"foo", "bar"}},
		{`"hello" + 2`, []string{}},
		{`[1, 2, 3]`, []string{}},
		{`len + foo`, []string{"foo", "len"}},
		{`true ? foo : bar`, []string{"foo", "bar"}},
		{"foo.bar.baz + qux", []string{"foo", "qux"}},
		{"foo.bar(len(baz)) + qux", []string{"baz", "qux"}},
		{`"value is \\(foo)" + bar`, []string{"bar"}},
		{"foo + ", []string{"foo"}},
		{"len + 1", []string{"len"}},
		{"#DIV/0! + foo", []string{"foo"}},
		{"#VALUE! + bar", []string{"bar"}},
		{"#NUM! + baz", []string{"baz"}},
		{"#NAME? + x", []string{"x"}},
		{"#NULL! + y", []string{"y"}},
		{"#N/A + z", []string{"z"}},
	}

	for _, tt := range formulas {
		app := setupTestApp(t)
		defer app.Cleanup()

		calculated_fields, _ := app.FindCollectionByNameOrId("calculated_fields")

		// Crea un record minimale nella collection “calculated_fields”
		rec := core.NewRecord(calculated_fields)
		rec.Set("id", "abcd123456789ef")
		rec.Set("formula", tt.formula)

		rec.Set("owner_collection", "cf_owner_pool")
		rec.Set("owner_row", "cf_owner_resolve_deps")
		rec.Set("owner_field", "cf")
		// Chiama la funzione da testare
		_, err := hooks.ResolveDepsAndTxSave(app, rec)
		if err != nil {
			t.Errorf("Formula %q: ResolveDepsAndTxSave error: %v", tt.formula, err)
			continue
		}

		got := rec.GetStringSlice("depends_on")
		sort.Strings(got)
		sort.Strings(tt.want)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Formula %q: depends_on = %v; want %v", tt.formula, got, tt.want)
		}
	}
}

func TestCF_TouchOwnerUpdated_WhenFormulaChanges(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	cfId := "booking_queue_vhtyej8kobau44t_min"

	var (
		ownerCol string
		ownerId  string
		before   string // string ok per confronto "changed"
	)

	(&tests.ApiScenario{
		Name:           "patch formula touches owner.updated",
		Method:         http.MethodPatch,
		URL:            "/api/collections/calculated_fields/records/" + cfId,
		Body:           strings.NewReader(`{"formula":"4"}`),
		Headers:        superAuthHeader,
		TestAppFactory: setupTestApp,
		ExpectedStatus: 200,
		ExpectedContent: []string{
			`"formula":"4"`,
		},

		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			t.Helper()

			cf, err := app.FindRecordById("calculated_fields", cfId)
			if err != nil {
				t.Fatalf("cannot find calculated_field: %v", err)
			}

			ownerCol = cf.GetString("owner_collection")
			ownerId = cf.GetString("owner_row")
			if ownerCol == "" || ownerId == "" {
				t.Fatalf("invalid CF owner ref: owner_collection=%q owner_row=%q", ownerCol, ownerId)
			}

			owner, err := app.FindRecordById(ownerCol, ownerId)
			if err != nil {
				t.Fatalf("cannot find owner %s/%s: %v", ownerCol, ownerId, err)
			}

			before = owner.GetDateTime("updated").String()
		},

		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			t.Helper()

			owner, err := app.FindRecordById(ownerCol, ownerId)
			if err != nil {
				t.Fatalf("cannot reload owner %s/%s: %v", ownerCol, ownerId, err)
			}

			after := owner.GetDateTime("updated").String()

			if after == before {
				t.Fatalf("expected %s/%s.updated to change after CF patch; before=%s after=%s",
					ownerCol, ownerId, before, after)
			}
		},
	}).Test(t)
}

func TestCF_TouchOwnerUpdated_WhenFormulaUnchanged_DoesNotTouchOwner(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	// CF reale nel dump (tu lo stai già usando)
	cfId := "booking_queue_vhtyej8kobau44t_min"
	ownerCol := "booking_queue"
	ownerId := "vhtyej8kobau44t" // owner reale

	var beforeOwnerUpdated string
	var sameFormula string

	sc := &tests.ApiScenario{
		Name:           "PATCH same formula must NOT touch owner.updated",
		Method:         http.MethodPatch,
		URL:            "/api/collections/calculated_fields/records/" + cfId,
		Headers:        superAuthHeader,
		TestAppFactory: setupTestApp,
		ExpectedStatus: 200,
		  ExpectedContent: []string{
    `"id":"` + cfId + `"`,
  },
	}

	sc.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
		t.Helper()

		// leggi formula attuale
		cf, err := app.FindRecordById("calculated_fields", cfId)
		if err != nil {
			t.Fatalf("cannot find calculated_fields/%s: %v", cfId, err)
		}
		sameFormula = cf.GetString("formula")
		if sameFormula == "" {
			t.Fatalf("test invalid: calculated_fields/%s has empty formula", cfId)
		}

		// leggi owner.updated prima
		owner, err := app.FindRecordById(ownerCol, ownerId)
		if err != nil {
			t.Fatalf("cannot find owner %s/%s: %v", ownerCol, ownerId, err)
		}
		beforeOwnerUpdated = owner.GetDateTime("updated").String()

		// PATCH con la stessa formula (JSON escape safe)
		payload := fmt.Sprintf(`{"formula":%q}`, sameFormula)
		sc.Body = strings.NewReader(payload)
	}

	sc.AfterTestFunc = func(t testing.TB, app *tests.TestApp, _ *http.Response) {
		t.Helper()

		owner, err := app.FindRecordById(ownerCol, ownerId)
		if err != nil {
			t.Fatalf("cannot reload owner %s/%s: %v", ownerCol, ownerId, err)
		}
		afterOwnerUpdated := owner.GetDateTime("updated").String()

		if afterOwnerUpdated != beforeOwnerUpdated {
			t.Fatalf(
				"owner.updated CHANGED even though formula was unchanged.\nowner=%s/%s\nbefore=%s\nafter =%s\nformula=%q\n",
				ownerCol, ownerId,
				beforeOwnerUpdated, afterOwnerUpdated,
				sameFormula,
			)
		}
	}

	sc.Test(t)
}
func TestCF_ParentValueEmptyString_IsHandledAsNull_NotCrash(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	parentId := "ut_parent_empty_value"
	childId := "ut_child_dep_on_empty"

	parentOwnerRow := "cf_owner_" + parentId
	childOwnerRow := "cf_owner_" + childId

	(&tests.ApiScenario{
		Name:           `parent value=="" must not crash child eval (treat as null)`,
		Method:         http.MethodPost,
		URL:            "/api/collections/calculated_fields/records",
		Headers:        superAuthHeader,
		TestAppFactory: setupTestApp,

		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			t.Helper()

			// 1) Seed owners in cf_owner_pool (required by your "owner must exist" guard)
			ownerCol, err := app.FindCollectionByNameOrId("cf_owner_pool")
			if err != nil {
				t.Fatalf("cannot find owner collection cf_owner_pool: %v", err)
			}

			seedOwner := func(id string) {
				if _, err := app.FindRecordById("cf_owner_pool", id); err == nil {
					return
				}
				r := core.NewRecord(ownerCol)
				r.Set("id", id)
				if err := app.UnsafeWithoutHooks().Save(r); err != nil {
					t.Fatalf("failed to seed owner cf_owner_pool/%s: %v", id, err)
				}
			}

			seedOwner(parentOwnerRow)
			seedOwner(childOwnerRow)

			// 2) Seed parent CF "sporco": value == "" bypassando hook
			cfCol, err := app.FindCollectionByNameOrId("calculated_fields")
			if err != nil {
				t.Fatalf("cannot find calculated_fields: %v", err)
			}

			parent := core.NewRecord(cfCol)
			parent.Set("id", parentId)
			parent.Set("formula", "2") // formula ok
			parent.Set("value", "")    // 🔴 caso problematico
			parent.Set("error", "")

			parent.Set("owner_collection", "cf_owner_pool")
			parent.Set("owner_row", parentOwnerRow)
			parent.Set("owner_field", "cf")

			if err := app.UnsafeWithoutHooks().Save(parent); err != nil {
				t.Fatalf("failed to seed parent calculated_field: %v", err)
			}
		},

		Body: strings.NewReader(`{
			"id": "` + childId + `",
			"formula": "` + parentId + ` + 1",
			"owner_collection": "cf_owner_pool",
			"owner_row": "` + childOwnerRow + `",
			"owner_field": "cf"
		}`),

		// Se oggi il codice fa json.Unmarshal("") e crasha/errore, qui non sarà 200.
		ExpectedStatus: 200,
		ExpectedContent: []string{
  `"id":"ut_child_dep_on_empty"`,
  `"value":"#VALUE!"`,
  `"error":"Tipo non compatibile nell'operazione"`,
},
	}).Test(t)
}
func TestResolveDeps_MissingParentIds_MustBeDetected(t *testing.T) {
	app := setupTestApp(t)
	defer app.Cleanup()

	cfCol, _ := app.FindCollectionByNameOrId("calculated_fields")

	// usa un id che sai esistere nel dump come parent "OK"
	okParent := "booking_queue_queue_a00000001_min"
	missingParent := "this_parent_does_not_exist"

	rec := core.NewRecord(cfCol)
	rec.Set("id", "ut_missing_parent_check")
	rec.Set("formula", okParent+" + "+missingParent+" + 1")
	rec.Set("owner_collection", "cf_owner_pool")
	rec.Set("owner_row", "cf_owner_missing_parent_check")
	rec.Set("owner_field", "cf")

	_, err := hooks.ResolveDepsAndTxSave(app, rec)

	// ✅ FIX atteso: err != nil e contiene info sui missing parents
	// 🔴 OGGI: spesso ottieni errori in fase di eval (“referenced variable not found”)
	if err == nil {
		t.Fatalf("expected error for missing parent %q, got nil", missingParent)
	}

	// questo check lo adatti a come vuoi formattare l'errore nel codice
	if !strings.Contains(err.Error(), "Formula evaluation error: referenced variable not found") && !strings.Contains(err.Error(), missingParent) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCF_ChangingParentFormulaButSameResult_DoesNotTouchChildUpdated(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	parentID := "booking_queue_queue_a00000001_min"
	childID := "booking_queue_queue_b00000002_min"

	var childUpdatedBefore string

	(&tests.ApiScenario{
		Name:           "PATCH parent formula (same result) must NOT update child.updated",
		Method:         http.MethodPatch,
		URL:            "/api/collections/calculated_fields/records/" + parentID,
		Headers:        superAuthHeader,
		TestAppFactory: setupTestApp,

		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			t.Helper()

			child, err := app.FindRecordById("calculated_fields", childID)
			if err != nil {
				t.Fatalf("cannot find child CF %s: %v", childID, err)
			}

			childUpdatedBefore = child.GetDateTime("updated").String()

			// sanity
			if strings.TrimSpace(childUpdatedBefore) == "" {
				t.Fatalf("child.updated is empty; test db not consistent")
			}
		},

		Body: strings.NewReader(`{"formula":"2 + 3"}`),

		ExpectedStatus: 200,
    ExpectedContent: []string{
        `"id":"` + parentID + `"`,
        `"formula":"2 + 3"`,
        `"value":5`, // opzionale ma utile: il parent resta 5
    },
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			t.Helper()

			child, err := app.FindRecordById("calculated_fields", childID)
			if err != nil {
				t.Fatalf("cannot reload child CF %s: %v", childID, err)
			}

			childUpdatedAfter := child.GetDateTime("updated").String()

			// Se isDirty è sbagliato, spesso qui cambia perché salva comunque B.
			if childUpdatedAfter != childUpdatedBefore {
				t.Fatalf("child.updated changed but should NOT.\nchild=%s\nbefore=%s\nafter =%s\n",
					childID, childUpdatedBefore, childUpdatedAfter)
			}

			// (opzionale) value invariato
			if child.GetString("value") != "6" {
				t.Fatalf("unexpected child.value. want=6 got=%s", child.GetString("value"))
			}
		},
	}).Test(t)
} 

func TestCalculatedFields_TouchOwnerUpdated_FailsIfOwnerMissing(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	authHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	ownerCol := "ut_owner_missing"
	ownerId := "ut_owner_DOES_NOT_EXIST"
	cfId := ownerCol + "_" + ownerId + "_cf"

	factory := setupTestApp

	(&tests.ApiScenario{
		Name:    "Patch formula fails with 1008 when owner missing",
		Method:  http.MethodPatch,
		URL:     "/api/collections/calculated_fields/records/" + cfId,
		Headers: authHeader,
		Body:    strings.NewReader(`{ "formula": "2" }`),

		TestAppFactory: factory,

		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			t.Helper()

			cfCol, err := app.FindCollectionByNameOrId("calculated_fields")
			if err != nil {
				t.Fatalf("cannot find calculated_fields collection: %v", err)
			}

			rec := core.NewRecord(cfCol)
			rec.Set("id", cfId)
			rec.Set("formula", "1")
			rec.Set("owner_collection", ownerCol)
			rec.Set("owner_row", ownerId)
			rec.Set("owner_field", "cf")

			// Seed bypassing hooks, otherwise create would already fail
			if err := app.UnsafeWithoutHooks().Save(rec); err != nil {
				t.Fatalf("failed to seed calculated_field: %v", err)
			}
		},

		ExpectedStatus: 400,
		ExpectedContent: []string{
			`"code":"1008"`,
			`Invalid owner reference: record ` + ownerCol + `/` + ownerId + ` not found.`,
		},
	}).Test(t)
}

func TestCalculatedFields_GenericOwnerCascadeDelete_UsesExistingBookingQueue(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	authHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	ownerCol := "booking_queue"
	ownerId := "vhtyej8kobau44t" // esiste nel tuo test_pb_data  [oai_citation:2‡bq.json](sediment://file_00000000868471f4ae294ccbf8b95cab)

	// Leggo prima i CF ids dal record reale
	app0 := setupTestApp(t)
	owner, err := app0.FindRecordById(ownerCol, ownerId)
	if err != nil {
		app0.Cleanup()
		t.Fatalf("cannot find owner %s/%s in test db: %v", ownerCol, ownerId, err)
	}

	actID := owner.GetString("act_fx")
	minID := owner.GetString("min_fx")
	maxID := owner.GetString("max_fx")
	app0.Cleanup()

	if actID == "" || minID == "" || maxID == "" {
		t.Fatalf("owner %s/%s has empty relation(s): act=%q min=%q max=%q", ownerCol, ownerId, actID, minID, maxID)
	}

	(&tests.ApiScenario{
		Name:           "delete owner triggers calculated_fields cascade delete",
		Method:         http.MethodDelete,
		URL:            "/api/collections/" + ownerCol + "/records/" + ownerId,
		Headers:        authHeader,
		TestAppFactory: setupTestApp,
		ExpectedStatus: 204,
		ExpectedContent: []string{
			"", // empty body
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			t.Helper()

			// owner must be gone
			if _, err := app.FindRecordById(ownerCol, ownerId); err == nil {
				t.Fatalf("expected owner %s/%s to be deleted, but it still exists", ownerCol, ownerId)
			}

			// calculated_fields must be gone (these IDs are real in your dump)  [oai_citation:3‡cf.json](sediment://file_000000000f7872468f1977a058db2d32)
			for _, cfId := range []string{actID, minID, maxID} {
				if strings.TrimSpace(cfId) == "" {
					continue
				}
				if _, err := app.FindRecordById("calculated_fields", cfId); err == nil {
					t.Fatalf("expected calculated_field %q to be deleted (cascade), but it still exists", cfId)
				}
			}
		},
	}).Test(t)
}

func TestCalculatedFields_Create_DoesNotFail_WhenOwnerPoolHasNoUpdatedField(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	 superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}
	
	ownerColName := "cf_owner_pool"
ownerId := "cfownnoupd00002" 
cfId := "utcfnoupd000002"  

	(&tests.ApiScenario{
		Name:           "create calculated_field with cf_owner_pool owner without updated field must not fail",
		Method:         http.MethodPost,
		URL:            "/api/collections/calculated_fields/records",
		Headers:        superAuthHeader,
		TestAppFactory: setupTestApp,

		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			t.Helper()

			ownerCol, err := app.FindCollectionByNameOrId(ownerColName)
			if err != nil {
				t.Fatalf("cannot find owner collection %q: %v", ownerColName, err)
			}

			// Assert: cf_owner_pool NON deve avere il field "updated"
			for _, f := range ownerCol.Fields {
				if f.GetName() == "updated" {
					t.Fatalf("test invalid: %s unexpectedly has an 'updated' field; this test must cover owners without it", ownerColName)
				}
			}

			// Seed: crea un owner reale in cf_owner_pool
			ownerRec := core.NewRecord(ownerCol)
			ownerRec.Set("id", ownerId)

			// Se in cf_owner_pool hai campi required (es. relation "cf"), qui NON li valorizziamo:
			// l'owner deve poter esistere "vuoto" e poi ci pensa il CF a puntargli contro.
			if err := app.UnsafeWithoutHooks().Save(ownerRec); err != nil {
				t.Fatalf("failed to seed owner record %s/%s: %v", ownerColName, ownerId, err)
			}
		},

		Body: strings.NewReader(`{
			"id": "` + cfId + `",
			"formula": "42 + 8",
			"owner_collection": "` + ownerColName + `",
			"owner_row": "` + ownerId + `",
			"owner_field": "cf"
		}`),

		ExpectedStatus: 200,
		ExpectedContent: []string{
			`"id":"` + cfId + `"`,
			`"value":50`,
		},
	}).Test(t)
}

func TestCalculatedFields_AutoCreate_OnOwnerCreate_BookingQueue(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	 superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}
	

	ownerCol := "booking_queue"
	ownerId := "utqueue000000001"  // 15 chars
	

	(&tests.ApiScenario{
		Name:           "create booking_queue auto-creates calculated_fields act/min/max",
		Method:         http.MethodPost,
		URL:            "/api/collections/" + ownerCol + "/records",
		Headers:        superAuthHeader,
		TestAppFactory: setupTestApp,

	
		Body: strings.NewReader(`{
			"id": "` + ownerId + `",
			"queue_name": "UT Queue",
			"booking_status": "booked"
		}`),

		ExpectedStatus: 200,
		ExpectedContent: []string{
			`"id":"` + ownerId + `"`,
			`"queue_name":"UT Queue"`,
		},

		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			t.Helper()

			owner, err := app.FindRecordById(ownerCol, ownerId)
			if err != nil {
				t.Fatalf("cannot find created owner %s/%s: %v", ownerCol, ownerId, err)
			}

			actID := owner.GetString("act_fx")
			minID := owner.GetString("min_fx")
			maxID := owner.GetString("max_fx")

			if actID == "" || minID == "" || maxID == "" {
				t.Fatalf("expected owner %s/%s to have act_fx/min_fx/max_fx set, got act=%q min=%q max=%q", ownerCol, ownerId, actID, minID, maxID)
			}

			// Verify the calculated_fields records exist and point back to the owner
			for _, tc := range []struct {
				field string
				cfId  string
			}{
				{"act_fx", actID},
				{"min_fx", minID},
				{"max_fx", maxID},
			} {
				cf, err := app.FindRecordById("calculated_fields", tc.cfId)
				if err != nil {
					t.Fatalf("expected calculated_fields/%s to exist (owner field %s), but not found: %v", tc.cfId, tc.field, err)
				}

				if cf.GetString("owner_collection") != ownerCol || cf.GetString("owner_row") != ownerId {
					t.Fatalf("calculated_field %s has wrong owner reference: owner_collection=%q owner_row=%q (want %q/%q)",
						tc.cfId,
						cf.GetString("owner_collection"),
						cf.GetString("owner_row"),
						ownerCol, ownerId,
					)
				}

				if cf.GetString("owner_field") != tc.field {
					t.Fatalf("calculated_field %s has wrong owner_field=%q (want %q)", tc.cfId, cf.GetString("owner_field"), tc.field)
				}
			}
		},
	}).Test(t)
}

func TestCalculatedFields_AutoCreate_AntiHijack_OnOwnerCreate_BookingQueue(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	 superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}
	

	ownerCol := "booking_queue"
	owner1Id := "utqueue000000001"
	owner2Id := "utqueue000000002"
	courseId := "utcourse00000002"

	scenario := &tests.ApiScenario{}
	*scenario = tests.ApiScenario{
		Name:           "anti-hijack: creating booking_queue with foreign calculated_field reference must fail",
		Method:         http.MethodPost,
		URL:            "/api/collections/" + ownerCol + "/records",
		Headers:        superAuthHeader,
		TestAppFactory: setupTestApp,

		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			t.Helper()

		

			// Create owner1 normally -> auto-create should generate CFs
			bqCol, err := app.FindCollectionByNameOrId(ownerCol)
			if err != nil {
				t.Fatalf("cannot find %s collection: %v", ownerCol, err)
			}
			owner1 := core.NewRecord(bqCol)
			owner1.Set("id", owner1Id)
			owner1.Set("queue_name", "UT Queue 1")
			owner1.Set("booking_status", "booked")
			owner1.Set("course", courseId)
			if err := app.Save(owner1); err != nil {
				t.Fatalf("failed to seed owner1 %s/%s: %v", ownerCol, owner1Id, err)
			}

			// Grab a CF from owner1 to hijack (e.g. act_fx)
			owner1db, err := app.FindRecordById(ownerCol, owner1Id)
			if err != nil {
				t.Fatalf("cannot reload owner1 %s/%s: %v", ownerCol, owner1Id, err)
			}
			hijackCfId := strings.TrimSpace(owner1db.GetString("act_fx"))
			if hijackCfId == "" {
				t.Fatalf("test invalid: owner1 %s/%s has empty act_fx", ownerCol, owner1Id)
			}

			// IMPORTANT: set Body here (after hijackCfId exists)
			scenario.Body = strings.NewReader(`{
				"id": "` + owner2Id + `",
				"queue_name": "UT Queue 2",
				"booking_status": "booked",
				"act_fx": "` + hijackCfId + `"
			}`)
		},

		ExpectedStatus: 400,
		ExpectedContent: []string{
			`"code":"1011"`,
		},

		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			t.Helper()
			if _, err := app.FindRecordById(ownerCol, owner2Id); err == nil {
				t.Fatalf("expected owner2 %s/%s to NOT be created after hijack attempt", ownerCol, owner2Id)
			}
		},
	}

	scenario.Test(t)
}
func TestCalculatedFields_Update_RequiresOwnerUpdatePermission(t *testing.T) {
	ownerColName := "ut_owner"
	ownerId := "utowner00000001" // 1

	allowedAdminId := "utadmallow00001" //
	deniedAdminId := "utadmdenyd00001"  //

	allowedUsername := "ut_allow"
	deniedUsername := "ut_deny"

	seedAll := func(t testing.TB, app *tests.TestApp) (cfId string, allowedToken string, deniedToken string, superToken string) {
		t.Helper()

		// ---------- (A) collection owner ----------
		ownerCol, err := app.FindCollectionByNameOrId(ownerColName)
		if err != nil {
			ownerCol = core.NewBaseCollection(ownerColName)

			ownerCol.Fields.Add(&core.TextField{
				Name:     "allowed_admin",
				Required: true,
			})

			cfCol, err := app.FindCollectionByNameOrId("calculated_fields")
			if err != nil {
				t.Fatalf("cannot find calculated_fields collection: %v", err)
			}

			ownerCol.Fields.Add(&core.RelationField{
				Name:         "cf",
				CollectionId: cfCol.Id,
				MinSelect:    0,
				MaxSelect:    1,
				Required:     false,
			})

			ownerCol.UpdateRule = types.Pointer(
				`@request.auth.collectionName = "administrators" && @request.auth.id = allowed_admin`,
			)
			ownerCol.CreateRule = nil
			ownerCol.ViewRule = types.Pointer(`@request.auth.id != ""`)
			ownerCol.ListRule = types.Pointer(`@request.auth.id != ""`)

			if err := app.Save(ownerCol); err != nil {
				t.Fatalf("failed to create owner collection %q: %v", ownerColName, err)
			}
		}

		// ---------- (B) seed admins (serve login, quindi password+tokenKey unici) ----------
		adminCol, err := app.FindCollectionByNameOrId("administrators")
		if err != nil {
			t.Fatalf("cannot find administrators collection: %v", err)
		}

		seedAdmin := func(id, username string) {
			if _, err := app.FindRecordById("administrators", id); err == nil {
				return
			}
			rec := core.NewRecord(adminCol)
			rec.Set("id", id)
			rec.Set("username", username)
			rec.Set("email", username+"@example.com")
			rec.Set("verified", true)
			rec.Set("tokenKey", security.RandomString(48))
			rec.SetPassword("testtest")

			if err := app.UnsafeWithoutHooks().Save(rec); err != nil {
				t.Fatalf("failed to seed administrators/%s: %v", id, err)
			}
		}

		seedAdmin(allowedAdminId, allowedUsername)
		seedAdmin(deniedAdminId, deniedUsername)

		// ---------- (C) seed owner record (HOOK crea automaticamente il CF su campo "cf") ----------
		// NB: se ti serve ID esattamente 15, assicurati che ownerId lo sia davvero.
		ownerRec := core.NewRecord(ownerCol)
		ownerRec.Set("id", ownerId)
		ownerRec.Set("allowed_admin", allowedAdminId)

		if err := app.Save(ownerRec); err != nil {
			t.Fatalf("failed to seed owner %s/%s: %v", ownerColName, ownerId, err)
		}

		// ricarica owner e prendi CF auto-creato
		ownerDb, err := app.FindRecordById(ownerColName, ownerId)
		if err != nil {
			t.Fatalf("cannot reload owner %s/%s: %v", ownerColName, ownerId, err)
		}
		ids := ownerDb.GetStringSlice("cf")
		if len(ids) == 0 || strings.TrimSpace(ids[0]) == "" {
			t.Fatalf("expected owner %s/%s to have cf set by auto-create hook", ownerColName, ownerId)
		}
		cfId = ids[0]

		// ---------- tokens ----------
		allowedToken = getAuthToken(app, "administrators", allowedUsername)
		deniedToken = getAuthToken(app, "administrators", deniedUsername)
		superToken = getSuperuserToken(t.(*testing.T), app) // ok: qui t è TB, cast perché è il test principale

		return
	}

	t.Run("allowed admin can patch CF", func(t *testing.T) {
		sc := &tests.ApiScenario{}
		*sc = tests.ApiScenario{
			Name:           "allowed admin patches calculated_field ok",
			Method:         http.MethodPatch,
			TestAppFactory: setupTestApp,
			Body:           strings.NewReader(`{"formula":"2"}`),
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"formula":"2"`,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
				cfId, allowedToken, _, _ := seedAll(t, app)
				sc.URL = "/api/collections/calculated_fields/records/" + cfId
				sc.Headers = map[string]string{"Authorization": allowedToken}
				sc.ExpectedContent = append(sc.ExpectedContent, `"id":"`+cfId+`"`)
			},
		}
		sc.Test(t)
	})

	t.Run("denied admin cannot patch CF (hook checks owner updateRule)", func(t *testing.T) {
		sc := &tests.ApiScenario{}
		*sc = tests.ApiScenario{
			Name:           "denied admin patches calculated_field forbidden",
			Method:         http.MethodPatch,
			TestAppFactory: setupTestApp,
			Body:           strings.NewReader(`{"formula":"3"}`),
			ExpectedStatus: 403,
			ExpectedContent: []string{
  `"message":"Forbidden updating calculated_fields/`,
  `user administrators/` + deniedAdminId,
  `has no update access to owner ` + ownerColName + `/` + ownerId,
},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
				cfId, _, deniedToken, _ := seedAll(t, app)
				sc.URL = "/api/collections/calculated_fields/records/" + cfId
				sc.Headers = map[string]string{"Authorization": deniedToken}
				sc.ExpectedContent = append(sc.ExpectedContent, `calculated_fields/`+cfId)
			},
		}
		sc.Test(t)
	})

	t.Run("superuser can patch CF", func(t *testing.T) {
		sc := &tests.ApiScenario{}
		*sc = tests.ApiScenario{
			Name:           "superuser patches calculated_field ok",
			Method:         http.MethodPatch,
			TestAppFactory: setupTestApp,
			Body:           strings.NewReader(`{"formula":"4"}`),
			ExpectedStatus: 200,
			  ExpectedContent: []string{
    `"formula":"4"`,
  },
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
				cfId, _, _, superToken := seedAll(t, app)
				sc.URL = "/api/collections/calculated_fields/records/" + cfId
				sc.Headers = map[string]string{"Authorization": superToken}
			},
		}
		sc.Test(t)
	})
}




func TestCalculatedFields_ServerSideSave_RecalculatesValue(t *testing.T) {
	// Questo test si aspetta il "vecchio comportamento":
	// un app.Save(record) (NON request) su calculated_fields deve triggerare ricalcolo/value update.
	//
	// Con i hook spostati su OnRecordUpdateRequest/OnRecordCreateRequest, dovrebbe FALLIRE,
	// perché app.Save(...) non passa dalla request pipeline.

	app := setupTestApp(t)
	defer app.Cleanup()

	// usa un CF che sai esistere nel tuo test_pb_data
	cfId := "booking_queue_queue_zb3456789_min"

	cf, err := app.FindRecordById("calculated_fields", cfId)
	if err != nil {
		t.Fatalf("cannot find calculated_fields/%s: %v", cfId, err)
	}

	oldFormula := cf.GetString("formula")
	oldValue := cf.GetString("value")

	// cambia formula in modo deterministico: costante, nessuna dipendenza
	cf.Set("formula", "100")

	// 🔴 server-side save (NON request)
	if err := app.Save(cf); err != nil {
		t.Fatalf("server-side app.Save(calculated_fields/%s) failed: %v", cfId, err)
	}

	// ricarica da db
	after, err := app.FindRecordById("calculated_fields", cfId)
	if err != nil {
		t.Fatalf("cannot reload calculated_fields/%s: %v", cfId, err)
	}

	newFormula := after.GetString("formula")
	newValue := after.GetString("value")


	// la formula deve essere aggiornata (questa dovrebbe PASSARE)
	if newValue != "100" || newFormula !="100"{
		t.Fatalf("expected formula to be persisted as %q, got %q (old=%q, new=%q)", "100", newValue, oldFormula, newFormula)
	}

	// ✅ aspettativa "vecchio mondo": anche value deve aggiornarsi a 100
	// (Se oggi sei solo su request hooks, qui tipicamente rimane il vecchio value e il test FALLISCE)
	if got := after.GetString("value"); got != "100" {
		t.Fatalf(
			"EXPECTED recalculation on server-side Save did not happen.\ncalculated_fields/%s\noldValue=%s\nnewValue=%s\n(oldFormula=%q)\n",
			cfId, oldValue, got, oldFormula,
		)
	}

	// extra: sanity check
	if oldValue == "100" {
		t.Fatalf("test invalid: oldValue was already 100, pick another CF id")
	}
}

func TestCalculatedFields_AutoCreate_ServerSideSave_ShouldEvaluateCF(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	defer autApp.Cleanup()

	superAuthHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	ownerColName := "ut_owner_eval" // ok
ownerId := "utoeval00000001" 

	// app factory con hook
	factory := func(t testing.TB) *tests.TestApp {
		app, err := tests.NewTestApp("../pb_data")
		if err != nil {
			t.Fatal(err)
		}
		hooks.BindCalculatedFieldsHooks(app)
		return app
	}

	(&tests.ApiScenario{
		Name:           "create owner triggers auto-create CF and CF is evaluated even if created via txApp.Save()",
		Method:         http.MethodPost,
		URL:            "/api/collections/" + ownerColName + "/records",
		Headers:        superAuthHeader,
		TestAppFactory: factory,

		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			t.Helper()

			// 1) crea owner collection con relation single-select verso calculated_fields
			if _, err := app.FindCollectionByNameOrId(ownerColName); err != nil {
				ownerCol := core.NewBaseCollection(ownerColName)

				cfCol, err := app.FindCollectionByNameOrId("calculated_fields")
				if err != nil {
					t.Fatalf("cannot find calculated_fields: %v", err)
				}

				ownerCol.Fields.Add(&core.RelationField{
					Name:         "cf",
					CollectionId: cfCol.Id,
					MinSelect:    0,
					MaxSelect:    1,
					Required:     false,
				})

				// regole permissive per il test
				ownerCol.CreateRule = nil
				ownerCol.UpdateRule = nil
				ownerCol.ViewRule = types.Pointer(`@request.auth.id != ""`)
				ownerCol.ListRule = types.Pointer(`@request.auth.id != ""`)

				if err := app.Save(ownerCol); err != nil {
					t.Fatalf("failed to create owner collection %q: %v", ownerColName, err)
				}
			}
		},

		Body: strings.NewReader(`{
			"id":"` + ownerId + `"
		}`),

		ExpectedStatus: 200,
		ExpectedContent: []string{
			`"id":"` + ownerId + `"`,
		},

		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			t.Helper()

			// 2) ricarica owner e prendi il CF id auto-creato
			owner, err := app.FindRecordById(ownerColName, ownerId)
			if err != nil {
				t.Fatalf("cannot reload owner %s/%s: %v", ownerColName, ownerId, err)
			}

			ids := owner.GetStringSlice("cf")
			if len(ids) == 0 || strings.TrimSpace(ids[0]) == "" {
				t.Fatalf("expected owner %s/%s to have cf set by auto-create hook", ownerColName, ownerId)
			}
			cfId := ids[0]

			// 3) CF esiste?
			cf, err := app.FindRecordById("calculated_fields", cfId)
			if err != nil {
				t.Fatalf("expected calculated_fields/%s to exist, but not found: %v", cfId, err)
			}

			// 4) ✅ aspettativa: il CF creato via txApp.Save(newCF) deve essere anche valutato
			// default formula = "0" -> value deve essere 0 e error vuoto
			raw := cf.GetString("value")
			var v any
			if raw != "" {
				_ = json.Unmarshal([]byte(raw), &v)
			}

			// value numerico 0 (o string "0" se il field è diverso, tolleriamo entrambi)
			isZero := false
			switch vv := v.(type) {
			case float64:
				isZero = (vv == 0)
			case string:
				isZero = (vv == "0")
			default:
				// se raw non era JSON valido o era vuoto -> sicuramente non valutato come ci aspettiamo
				isZero = false
			}

			if !isZero {
				t.Fatalf("CF auto-created was NOT evaluated.\ncalculated_fields/%s\nformula=%q\nvalue(raw)=%q\nparsed=%#v\nerror=%q\n\nThis usually fails when CF calc hook is bound only to *Request* events and the CF is created via txApp.Save() inside owner-create hook.",
					cfId,
					cf.GetString("formula"),
					raw,
					v,
					cf.GetString("error"),
				)
			}

			if cf.GetString("error") != "" {
				t.Fatalf("CF auto-created has unexpected error.\ncalculated_fields/%s\nerror=%q\nvalue=%q\nformula=%q",
					cfId, cf.GetString("error"), cf.GetString("value"), cf.GetString("formula"),
				)
			}
		},
	}).Test(t)
}

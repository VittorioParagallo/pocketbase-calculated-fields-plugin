package tests

import (
	"myapp/hooks"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestComputedFieldsCreateFxCalculations(t *testing.T) {
	//t.Parallel()
	autApp, _ := tests.NewTestApp("./test_pb_data")
	authHeader := map[string]string{"Authorization": getAuthToken(autApp, "administrators", "testuserpbx.com")}

	scenarios := []tests.ApiScenario{

		{
			Name:   "Calcolo formula costante",
			Method: http.MethodPost,
			URL:    "/api/collections/calculated_fields/records",
			Body: strings.NewReader(`{
					"id": "testcoll_abc123_constant",
					"formula": "42 + 8"
					}`),
			Headers:        authHeader,
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
					"formula": "booking_queue_queue_b00000002_act + 2"
				}`),
			Headers:        authHeader,
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
														"formula": "booking_queue_self_ref_node_value + 1"
													}`),
			Headers:        authHeader,
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
														"formula": "1 + * 2"
													}`),
			Headers:        authHeader,
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
											"formula": "booking_queue_nonexistent_record_act + 10"
										}`),
			Headers:         authHeader,
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
						"formula": "booking_queue_queue_b00000002_act + \"abc\""
					}`),
			Headers:        authHeader,
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
							"formula": "10 / 0"
						}`),
			Headers:        authHeader,
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
								"formula": "booking_queue_queue_b00000002_act + 💥"
							}`),
			Headers:        authHeader,
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
				"formula": "booking_queue_queue_b00000002_min + booking_queue_queue_c00000000_max"
							}`),
			Headers:        authHeader,
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
				"formula": "booking_queue_queue_a00000001_min + booking_queue_queue_c00000000_max"
			}`),
			Headers:        authHeader,
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
				"formula": "1 + foo_bar_baz"
			}`),
			Headers:        authHeader,
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
								"formula": "exec(\"rm -rf /\")"
							}`),
			Headers:        authHeader,
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
								"formula": "sum()"
							}`),
			Headers:        authHeader,
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
								"formula": "booking_queue_queue_a00000001_min + 🚀"
							}`),
			Headers:        authHeader,
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
								"formula": "booking_queue_v7tex6i3v4w6hs0_linked_bookings + 1"
							}`),
			Headers:        authHeader,
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
								"formula": "len(booking_queue_v7tex6i3v4w6hs0_linked_bookings)"
							}`),
			Headers:        authHeader,
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
								"formula": "\"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings"
							}`),
			Headers:        authHeader,
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
								"formula": "if \"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings { 100 } else { 0 }"
							}`),
			Headers:        authHeader,
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
								"formula": "\"booking00000001\" in booking_queue_v7tex6i3v4w6hs0_linked_bookings ? 100 : 0"
							}`),
			Headers:        authHeader,
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
								"formula": "sum([1, 2, 3]) + min([5, 2, 9]) + max([4, 8, 1])"
							}`),
			Headers:        authHeader,
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
								"formula": "sum([booking_queue_queue_a00000001_min, booking_queue_queue_b00000002_min, booking_queue_queue_c00000000_max])"
							}`),
			Headers:        authHeader,
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
								"formula": "max(booking_queue_queue_a00000001_min, booking_queue_queue_b00000002_min)"
							}`),
			Headers:        authHeader,
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
								"formula": "sum([len(booking_queue_v7tex6i3v4w6hs0_linked_bookings), 7])"
							}`),
			Headers:        authHeader,
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
					"formula": "sum([max(3, 7), 5])"
				}`),
			Headers:        authHeader,
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
					"formula": "sum([min(2, 4), booking_queue_queue_b00000002_act])"
				}`),
			Headers:        authHeader,
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
		"formula": "sum([if booking_queue_queue_a00000001_min > 3 { 10 } else { 0 }, 5])"
	}`),
			Headers:        authHeader,
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
			Body: strings.NewReader(`{"id": "booking_queue_queue_d00000001_nested_formula_test","formula": "sum([len(booking_queue_v7tex6i3v4w6hs0_linked_bookings),max(booking_queue_queue_a00000001_min,booking_queue_queue_b00000002_min),10,if booking_queue_queue_c00000000_max > 10 { 100 } else { 0 }])"
				}`),
			Headers:        authHeader,
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
        "formula": "string(booking_queue_queue_a00000001_min) + \"‑\" + string(booking_queue_queue_b00000002_min)"
    }`),
			Headers:        authHeader,
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
        "formula": "date(booking_queue_queue_date_test_date) + duration(\"72h\")"
    }`),
			Headers:        authHeader,
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
	autApp, _ := tests.NewTestApp("./test_pb_data")
	authHeader := map[string]string{"Authorization": getAuthToken(autApp, "administrators", "testuserpbx.com")}

	scenarios := []tests.ApiScenario{

		{
			Name:   "Update formula valida su record esistente booking_queue_queue_zb3456789_min",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zb3456789_min",
			Body: strings.NewReader(`{
		"formula": "booking_queue_queue_zc4567890_min + 10"
	}`),
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Name:   "Errore su ciclo indiretto tra record",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_min",
			Body: strings.NewReader(`{"formula": "booking_queue_queue_c00000000_min + 1"}`),
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1003"`,
			},
		}, {
			Name:   "Errore su funzione con tipo errato (sum su int e non con array come argomento)",
			Method: http.MethodPatch,
			URL:    "/api/collections/calculated_fields/records/booking_queue_queue_zc4567890_min",
			Body: strings.NewReader(`{"formula": "sum(5)"}`),
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
    Name:   "Update A propaga ad A, B, C e D correttamente con D che dipende da A e da C",
    Method: http.MethodPatch,
    URL:    "/api/collections/calculated_fields/records/booking_queue_queue_a00000001_min",
    Body:   strings.NewReader(`{"formula": "8"}`), // esempio: cambiamo A a 8
    Headers:        authHeader,
    TestAppFactory: setupTestApp,
    ExpectedStatus: 200,
    ExpectedContent: []string{
        `"id":"booking_queue_queue_a00000001_min"`,
        `"formula":"8"`,
        `"value":8`,         // A diventa 8
        `"id":"booking_queue_queue_b00000002_min"`,
        `"value":9`,         // B = A + 1 => 9
        `"id":"booking_queue_queue_c00000000_min"`,
        `"value":11`,        // C = B + 2 => 11
        `"id":"booking_queue_queue_d00000001_min"`,
        `"value":26`,         // D = A + C_max (?) → se C_max è per esempio 11, D = 8 + 11 = 19
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
	autApp, _ := tests.NewTestApp("./test_pb_data")
	authHeader := map[string]string{"Authorization": getAuthToken(autApp, "administrators", "testuserpbx.com")}

	scenarios := []tests.ApiScenario{
		{
			Name:           "tries to delete without auth",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_b00000002_min",
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 204,
		},
		{
			Name:           "Delete node with dependents triggers formula replacement",
			Method:         http.MethodDelete,
			URL:            "/api/collections/calculated_fields/records/booking_queue_queue_b00000002_min",
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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
			Headers:        authHeader,
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

	testApp, err := tests.NewTestApp("./test_pb_data")
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

func TestComputedFieldsUpdatesSourceCollection(t *testing.T) {
	autApp, _ := tests.NewTestApp("./test_pb_data")
	authHeader := map[string]string{"Authorization": getAuthToken(autApp, "administrators", "testuserpbx.com")}
	startTimeBeforePatch := time.Now()
	scenarios := []tests.ApiScenario{
		{
			Name:           "Updating calculated field referenced from booking_queue triggers update in referenced  record defined in update_target field",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_vhtyej8kobau44t_min",
			Body:           strings.NewReader(`{"formula": "4"}`),
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"booking_queue_vhtyej8kobau44t_min"`,
				`"formula":"4"`,
				`"value":4`, // queue_zc4567890.min = 4
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				updatedRecord, err := app.FindRecordById("booking_queue", "ywyty5ygy3kmmze")
				if err != nil {
					t.Fatalf("Failed to find updated booking_queue record: %v", err)
				}
				updatedAt := updatedRecord.GetDateTime("updated").Time()
				if !updatedAt.After(startTimeBeforePatch) {
					t.Errorf("Expected 'updated' timestamp to be refreshed after %v, but got %v", startTimeBeforePatch, updatedRecord.GetDateTime("updated"))
				}
			},
		},
		{
			Name:           "No update in booking_queue when formula value is unchanged",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/booking_queue_vhtyej8kobau44t_min",
			Body:           strings.NewReader(`{"formula": "3"}`), // stessa formula già salvata
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"formula":"3"`,
				`"value":3`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				record, err := app.FindRecordById("booking_queue", "ywyty5ygy3kmmze")
				if err != nil {
					t.Fatalf("Failed to find booking_queue record: %v", err)
				}
				updatedAt := record.GetDateTime("updated").Time()
				if updatedAt.After(startTimeBeforePatch) {
					t.Errorf("Expected 'updated' field NOT to change, but it was refreshed at %v", updatedAt)
				}
			},
		},
		{
			Name:           "Update_target points to non-existent collection record field",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/fake_target_ref",
			Body:           strings.NewReader(`{"formula": "10"}`),
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1008"`,
				`"message":"Invalid update_target: record fakecollection/fakerecord not found."`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// Nessun panic o errore, log silenzioso nel backend
			},
		},
		{
			Name:           "Update_target points to missing record in existing collection",
			Method:         http.MethodPatch,
			URL:            "/api/collections/calculated_fields/records/missing_record_ref",
			Body:           strings.NewReader(`{"formula": "2+7"}`),
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"code":"1008"`,
				`"message":"Invalid update_target: record booking_queue/abcdef1290wegty not found."`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

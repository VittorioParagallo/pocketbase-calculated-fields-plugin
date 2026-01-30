package tests

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestCalculatedFieldsSchemaGuard_BlocksMultiSelectRelation(t *testing.T) {
	autApp, _ := tests.NewTestApp("../pb_data")
	authHeader := map[string]string{"Authorization": getSuperuserToken(t, autApp)}

	// NB: qui uso direttamente l'ID della collection _calculated_fields che hai nel dump:
	// "id": "pbc_2828438558"
	// Se in futuro cambia, lo rendiamo dinamico.
	const calculatedFieldsColId = "pbc_2828438558"

	scenarios := []tests.ApiScenario{
		{
			Name:   "Schema guard: blocca relation verso _calculated_fields con maxSelect > 1",
			Method: http.MethodPost,
			URL:    "/api/collections",
			Body: strings.NewReader(`{
				"name": "bad_owner_collection",
				"type": "base",
				"fields": [
					{
						"type": "text",
						"name": "title"
					},
					{
						"type": "relation",
						"name": "cf",
						"collectionId": "` + calculatedFieldsColId + `",
						"cascadeDelete": false,
						"minSelect": 0,
						"maxSelect": 2
					}
				]
			}`),
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 400,

			ExpectedContent: []string{
				`"status":400`,
				`Invalid schema: relation to _calculated_fields must be single-select (maxSelect=1)`,
			},
		},
		{
			Name:   "Schema guard: consente relation verso _calculated_fields con maxSelect = 1",
			Method: http.MethodPost,
			URL:    "/api/collections",
			Body: strings.NewReader(`{
    "name": "good_owner_collection",
    "type": "base",
    "fields": [
      { "type": "text", "name": "title" },
      {
        "type": "relation",
        "name": "cf",
        "collectionId": "` + calculatedFieldsColId + `",
        "cascadeDelete": false,
        "minSelect": 0,
        "maxSelect": 1
      }
    ]
  }`),
			Headers:        authHeader,
			TestAppFactory: setupTestApp,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"name":"good_owner_collection"`,
				`"maxSelect":1`,
				`"collectionId":"` + calculatedFieldsColId + `"`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}

}

func getSuperuserToken(t testing.TB, app *tests.TestApp) string {
	t.Helper()

	col, err := app.FindCollectionByNameOrId("_superusers")
	if err != nil {
		t.Fatalf("cannot find _superusers collection: %v", err)
	}

	// crea un superuser "ad hoc" per questo test
	rec := core.NewRecord(col)
	rec.Set("email", "admin@admin.com")
	rec.Set("password", "adminadmin")
	rec.Set("passwordConfirm", "adminadmin")

	if err := app.Save(rec); err != nil {
		// se già esiste, non è un problema: prova a leggerlo
		existing, ferr := app.FindFirstRecordByData("_superusers", "email", "admin@admin.com")
		if ferr != nil {
			t.Fatalf("failed to create or find superuser: createErr=%v findErr=%v", err, ferr)
		}
		rec = existing
	}

	token, err := rec.NewAuthToken()
	if err != nil {
		t.Fatalf("failed to create superuser auth token: %v", err)
	}
	return token
}
func TestCalculatedFieldsSchemaGuard_BlocksPatchToMultiSelectRelation_DirectSave(t *testing.T) {
	app := setupTestApp(t)
	defer app.Cleanup()

	cf, err := app.FindCollectionByNameOrId("_calculated_fields")
	if err != nil {
		t.Fatalf("cannot find _calculated_fields: %v", err)
	}

	// 1) create collection with maxSelect=1
	col := core.NewBaseCollection("good_owner_collection_patch_test")
	col.Fields.Add(&core.TextField{
		Name: "title",
	})
	rel := &core.RelationField{
		Name:          "cf",
		CollectionId:  cf.Id,
		MaxSelect:     1,
		MinSelect:     0,
		CascadeDelete: false,
	}
	col.Fields.Add(rel)

	if err := app.Save(col); err != nil {
		t.Fatalf("expected create OK, got err: %v", err)
	}

	// 2) patch (simulate) maxSelect -> 2 and save again
	rel.MaxSelect = 2
	if err := app.Save(col); err == nil {
		t.Fatalf("expected schema guard error on save with maxSelect=2, got nil")
	}
}

func TestCalculatedFieldsSchemaGuard_BlocksUpdateToMaxSelect2(t *testing.T) {
	app := setupTestApp(t)
	defer app.Cleanup()

	cf, err := app.FindCollectionByNameOrId("_calculated_fields")
	if err != nil {
		t.Fatalf("cannot find _calculated_fields: %v", err)
	}

	// 1) CREATE ok (maxSelect=1)
	col := core.NewBaseCollection("owner_collection_update_maxselect_test")
	col.Fields.Add(&core.TextField{Name: "title"})
	rel := &core.RelationField{
		Name:          "cf",
		CollectionId:  cf.Id,
		MinSelect:     0,
		MaxSelect:     1,
		CascadeDelete: false,
	}
	col.Fields.Add(rel)

	if err := app.Save(col); err != nil {
		t.Fatalf("expected create OK, got err: %v", err)
	}

	// 2) UPDATE (maxSelect=2) -> MUST fail
	rel.MaxSelect = 2
	err = app.Save(col)
	if err == nil {
		t.Fatalf("expected schema guard error when updating maxSelect to 2, got nil")
	}

	// (opzionale) check messaggio
	if !strings.Contains(err.Error(), "relation to _calculated_fields must be single-select") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func ensureCalculatedFieldsViewRule(t testing.TB, app *tests.TestApp) {
	t.Helper()

	cfCol, err := app.FindCollectionByNameOrId("_calculated_fields")
	if err != nil {
		t.Fatalf("cannot find _calculated_fields collection: %v", err)
	}

	// add field allowed_view_admin if missing
	has := false
	for _, f := range cfCol.Fields {
		if f.GetName() == "allowed_view_admin" {
			has = true
			break
		}
	}
	if !has {
		cfCol.Fields.Add(&core.TextField{
			Name:     "allowed_view_admin",
			Required: false,
		})
	}

	// VIEW: superusers always ok; admins only if id matches allowed_view_admin
	cfCol.ViewRule = types.Pointer(
		`@request.auth.collectionName = "_superusers" || ` +
			`(@request.auth.collectionName = "administrators" && @request.auth.id = allowed_view_admin)`,
	)

	// (opzionale) list uguale alla view, così non rompi altre cose se ti serve
	cfCol.ListRule = cfCol.ViewRule

	if err := app.Save(cfCol); err != nil {
		t.Fatalf("failed to update _calculated_fields schema/rules: %v", err)
	}
}

func seedAdmin(t testing.TB, app *tests.TestApp, id, username string) {
	t.Helper()

	adminCol, err := app.FindCollectionByNameOrId("administrators")
	if err != nil {
		t.Fatalf("cannot find administrators collection: %v", err)
	}

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

// crea owner collection ad hoc con updateRule che abilita solo allowed_admin
func ensureOwnerCollectionForUpdateGuard(t testing.TB, app *tests.TestApp, ownerColName string) {
	t.Helper()

	if _, err := app.FindCollectionByNameOrId(ownerColName); err == nil {
		return
	}

	ownerCol := core.NewBaseCollection(ownerColName)
	ownerCol.Fields.Add(&core.TextField{
		Name:     "allowed_admin",
		Required: true,
	})

	cfCol, err := app.FindCollectionByNameOrId("_calculated_fields")
	if err != nil {
		t.Fatalf("cannot find _calculated_fields collection: %v", err)
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

// crea CF con owner e allowed_view_admin e formula
func createCF(t testing.TB, app *tests.TestApp, id, formula, ownerCol, ownerRow, ownerField, allowedViewAdmin string) *core.Record {
	t.Helper()

	cfCol, err := app.FindCollectionByNameOrId("_calculated_fields")
	if err != nil {
		t.Fatalf("cannot find _calculated_fields: %v", err)
	}

	rec := core.NewRecord(cfCol)
	rec.Set("id", id)
	rec.Set("formula", formula)
	rec.Set("owner_collection", ownerCol)
	rec.Set("owner_row", ownerRow)
	rec.Set("owner_field", ownerField)
	rec.Set("allowed_view_admin", allowedViewAdmin)

	// Save normale così popola depends_on (tramite i tuoi hook OnCalculatedFieldsCreateUpdate)
	if err := app.Save(rec); err != nil {
		t.Fatalf("failed to create _calculated_fields/%s: %v", id, err)
	}
	return rec
}

// PASS: updater ha UPDATE su owner + VIEW su deps closure => PATCH ok
func TestCalculatedFields_UpdateGuard_RequiresViewOnTransitiveDeps_AllowsWhenViewable(t *testing.T) {
	superScenario := &tests.ApiScenario{
		Name:           "setup and patch OK",
		Method:         http.MethodPatch,
		TestAppFactory: setupTestApp,
		ExpectedStatus: 200,
	}

	superScenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
		t.Helper()

		ensureCalculatedFieldsViewRule(t, app)
		updaterId := "utadmallow00001" // 15
		otherId := "utadmother00001"

		seedAdmin(t, app, updaterId, "ut_allow")
		seedAdmin(t, app, otherId, "ut_other")

		ownerCol := "ut_owner_vw1"
		ensureOwnerCollectionForUpdateGuard(t, app, ownerCol)

		ownerId := "utownervwok0001"
		ownerRec := core.NewRecord(mustFindCol(t, app, ownerCol))
		ownerRec.Set("id", ownerId)
		ownerRec.Set("allowed_admin", updaterId)
		if err := app.Save(ownerRec); err != nil {
			t.Fatalf("failed to seed owner: %v", err)
		}

		// chain: A depends on B; B depends on C
		cID := "cfcvwok00000001" // 15
		bID := "cfbvwok00000001" // 15
		aID := "cfavwok00000001"

		c := createCF(t, app, cID, "2", ownerCol, ownerId, "cf_c", updaterId)
		_ = c
		b := createCF(t, app, bID, fmt.Sprintf("%s + 1", cID), ownerCol, ownerId, "cf_b", updaterId)
		_ = b
		a := createCF(t, app, aID, fmt.Sprintf("%s + 1", bID), ownerCol, ownerId, "cf_a", updaterId)
		_ = a

		token := getAuthToken(app, "administrators", "ut_allow")

		superScenario.URL = "/api/collections/_calculated_fields/records/" + aID
		superScenario.Headers = map[string]string{"Authorization": token}
		superScenario.Body = strings.NewReader(fmt.Sprintf(`{"formula":"%s + 5"}`, bID))
		superScenario.ExpectedContent = []string{`"id":"` + aID + `"`}
	}

	superScenario.Test(t)
}

// FAIL: dipendenza diretta non viewable => PATCH 403
func TestCalculatedFields_UpdateGuard_FailsWhenDirectDepNotViewable(t *testing.T) {
	sc := &tests.ApiScenario{
		Name:           "direct dep not viewable -> forbidden",
		Method:         http.MethodPatch,
		TestAppFactory: setupTestApp,
		ExpectedStatus: 403,
		ExpectedContent: []string{
			`"message":"Forbidden`,
		},
	}

	sc.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
		t.Helper()

		ensureCalculatedFieldsViewRule(t, app)

		updaterId := "utadm_allow_002"
		otherId := "utadm_other_002"
		seedAdmin(t, app, updaterId, "ut_allow2")
		seedAdmin(t, app, otherId, "ut_other2")

		ownerCol := "ut_owner_vw2"
		ensureOwnerCollectionForUpdateGuard(t, app, ownerCol)

		ownerId := "utownvwno200002"
		ownerRec := core.NewRecord(mustFindCol(t, app, ownerCol))
		ownerRec.Set("id", ownerId)
		ownerRec.Set("allowed_admin", updaterId)
		if err := app.Save(ownerRec); err != nil {
			t.Fatalf("failed to seed owner: %v", err)
		}

		// A references B, but B is NOT viewable by updater (allowed_view_admin != updaterId)
		bID := "cfbvwno20000002" // 15
		aID := "cfavwno20000002"
		createCF(t, app, bID, "2", ownerCol, ownerId, "cf_b", otherId)
		createCF(t, app, aID, fmt.Sprintf("%s + 1", bID), ownerCol, ownerId, "cf_a", updaterId)

		token := getAuthToken(app, "administrators", "ut_allow2")
		sc.URL = "/api/collections/_calculated_fields/records/" + aID
		sc.Headers = map[string]string{"Authorization": token}
		sc.Body = strings.NewReader(fmt.Sprintf(`{"formula":"%s + 5"}`, bID))
	}

	sc.Test(t)
}

// FAIL: dipendenza profonda (nipote) non viewable => PATCH 403 (verifica chiusura transitiva)
func TestCalculatedFields_UpdateGuard_FailsWhenTransitiveDepNotViewable(t *testing.T) {
	sc := &tests.ApiScenario{
		Name:           "transitive dep not viewable -> forbidden",
		Method:         http.MethodPatch,
		TestAppFactory: setupTestApp,
		ExpectedStatus: 403,
		ExpectedContent: []string{
			`"message":"Forbidden`,
		},
	}

	sc.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
		t.Helper()

		ensureCalculatedFieldsViewRule(t, app)

		updaterId := "utadm_allow_03"
		otherId := "utadm_other_03"
		seedAdmin(t, app, updaterId, "ut_allow3")
		seedAdmin(t, app, otherId, "ut_other3")

		ownerCol := "ut_owner_vw3"
		ensureOwnerCollectionForUpdateGuard(t, app, ownerCol)

		ownerId := "utownvwno300003"
		ownerRec := core.NewRecord(mustFindCol(t, app, ownerCol))
		ownerRec.Set("id", ownerId)
		ownerRec.Set("allowed_admin", updaterId)
		if err := app.Save(ownerRec); err != nil {
			t.Fatalf("failed to seed owner: %v", err)
		}

		// chain: A -> B -> C
		// updater can view B, but NOT C
		cID := "cfcvwno30000003" // 15
		bID := "cfbvwno30000003" // 15
		aID := "cfavwno30000003" // 15
		createCF(t, app, cID, "2", ownerCol, ownerId, "cf_c", otherId)
		createCF(t, app, bID, fmt.Sprintf("%s + 1", cID), ownerCol, ownerId, "cf_b", updaterId)
		createCF(t, app, aID, fmt.Sprintf("%s + 1", bID), ownerCol, ownerId, "cf_a", updaterId)
		token := getAuthToken(app, "administrators", "ut_allow3")
		sc.URL = "/api/collections/_calculated_fields/records/" + aID
		sc.Headers = map[string]string{"Authorization": token}
		sc.Body = strings.NewReader(fmt.Sprintf(`{"formula":"%s + 5"}`, bID))
	}

	sc.Test(t)
}

// ------------------------------------------------------------

func mustFindCol(t testing.TB, app *tests.TestApp, name string) *core.Collection {
	t.Helper()
	col, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatalf("cannot find collection %q: %v", name, err)
	}
	return col
}

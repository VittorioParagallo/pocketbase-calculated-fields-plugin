package hooks

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func CalculatedFieldsOwnersSchemaGuards(e *core.CollectionEvent) error {
	// NB: usa e.App, non app, per restare nel contesto dell’evento
	cf, err := e.App.FindCollectionByNameOrId("calculated_fields")
	if err != nil || cf == nil {
		// prima install o calculated_fields non ancora presente
		return e.Next()
	}
	cfId := cf.Id

	col := e.Collection
	if col.Id == cfId || col.Name == "calculated_fields" {
		return e.Next()
	}

	for _, f := range col.Fields {
		rf, ok := f.(*core.RelationField)
		if !ok {
			continue
		}

		if rf.CollectionId == cfId && rf.MaxSelect != 1 {
			return apis.NewBadRequestError(
				"Invalid schema: relation to calculated_fields must be single-select (maxSelect=1)",
				validation.Errors{
					rf.Name: validation.NewError(
						"calculated_fields_relation_maxSelect",
						"Relation fields pointing to calculated_fields must have maxSelect=1",
					),
				},
			)
		}
	}

	return e.Next()
}

func OnOwnerCreate_AutoCreateCalculatedFields(e *core.RecordEvent) error {
	// evita loop: quando stai creando un calculated_field, NON creare altro
	if e.Record != nil && e.Record.Collection() != nil && e.Record.Collection().Name == "calculated_fields" {
		return e.Next()
	}

	ownerCol := e.Record.Collection()
	if ownerCol == nil {
		return e.Next()
	}

	originalApp := e.App
	return originalApp.RunInTransaction(func(txApp core.App) error {
		e.App = txApp

		// 0) salva owner (così ha Id definitivo)
		if err := e.Next(); err != nil {
			return err
		}

		// 1) se non esiste calculated_fields, fine
		cfCol, err := txApp.FindCollectionByNameOrId("calculated_fields")
		if err != nil || cfCol == nil {
			return nil
		}
		cfColId := cfCol.Id

		// 2) per ogni relation field dell'owner che punta a calculated_fields
		for _, f := range ownerCol.Fields {
			rel, ok := f.(*core.RelationField)
			if !ok {
				continue
			}
			if rel.CollectionId != cfColId {
				continue
			}

			fieldName := rel.Name

			// PB salva relation anche single-select come []string
			ids := e.Record.GetStringSlice(fieldName)
			hasVal := len(ids) > 0 && strings.TrimSpace(ids[0]) != ""

			if hasVal {
				// ✅ anti-hijack: deve esistere e deve appartenere a QUESTO owner/field
				cfID := ids[0]
				cfRec, err := txApp.FindRecordById("calculated_fields", cfID)
				if err != nil {
					return apis.NewBadRequestError("Invalid calculated_field reference", validation.Errors{
						fieldName: validation.NewError("1011",
							fmt.Sprintf("calculated_field %q referenced by %s/%s not found", cfID, ownerCol.Name, e.Record.Id)),
					})
				}

				if cfRec.GetString("owner_collection") != ownerCol.Name ||
					cfRec.GetString("owner_row") != e.Record.Id ||
					cfRec.GetString("owner_field") != fieldName {
					return apis.NewBadRequestError("Calculated field hijack attempt", validation.Errors{
						fieldName: validation.NewError("1011",
							fmt.Sprintf("calculated_field %q does not belong to %s/%s.%s", cfID, ownerCol.Name, e.Record.Id, fieldName)),
					})
				}

				continue
			}

			// 3) non valorizzato -> crea CF owner-aware e collega
			newCF := core.NewRecord(cfCol)

			newCF.Set("formula", "0")
			newCF.Set("owner_collection", ownerCol.Name)
			newCF.Set("owner_row", e.Record.Id)
			newCF.Set("owner_field", fieldName)

			// Save "normale" -> farà scattare i tuoi hook di CF (validazioni, eval, ecc.)
			if err := txApp.Save(newCF); err != nil {
				return err
			}

			// collega il CF appena creato al campo relation dell'owner
			e.Record.Set(fieldName, []string{newCF.Id})

			// salva l'owner SENZA HOOKS per non rientrare in OnOwnerCreate_* (loop)
			if err := txApp.UnsafeWithoutHooks().Save(e.Record); err != nil {
				return err
			}
		}

		return nil
	})
}

// BindCalculatedFieldsGenericCascadeDelete:
// - intercetta la DELETE di QUALSIASI record (tutte le collection)
// - se quel record ha campi relation verso la collection calculated_fields,
//   cancella prima i record calculated_fields referenziati (triggerando OnCalculatedFieldsDelete)
// - poi procede con la delete dell'owner.
//
// Assunzione plugin:
// - le relation verso calculated_fields sono SINGLE select (maxSelect=1)
// - se un owner referenzia un CF, quel CF deve esistere (integrità demandata a PocketBase)

func OnOwnerDelete_AutoDeleteCalculatedFields(e *core.RecordEvent) error {
	// evita loop: quando stai cancellando un calculated_field, NON fare cascade su se stesso
	if e.Record != nil && e.Record.Collection() != nil && e.Record.Collection().Name == "calculated_fields" {
		return e.Next()
	}

	ownerCol := e.Record.Collection()
	if ownerCol == nil {
		return e.Next()
	}

	originalApp := e.App
	return originalApp.RunInTransaction(func(txApp core.App) error {
		e.App = txApp

		// se non esiste calculated_fields, non c'è nulla da fare
		cfCol, err := txApp.FindCollectionByNameOrId("calculated_fields")
		if err != nil {
			return e.Next()
		}
		cfColId := cfCol.Id

		seen := map[string]struct{}{}
		cfIDs := make([]string, 0, 4)

		// 1) trova tutte le relation dell'owner che puntano a calculated_fields
		for _, f := range ownerCol.Fields {
			rel, ok := f.(*core.RelationField)
			if !ok {
				continue
			}
			if rel.CollectionId != cfColId {
				continue
			}

			// SINGLE select: PB salva comunque come []string
			ids := e.Record.GetStringSlice(rel.Name)
			if len(ids) == 0 || ids[0] == "" {
				continue
			}

			id := ids[0]
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			cfIDs = append(cfIDs, id)
		}

		// 2) cancella i CF referenziati (deve triggerare OnCalculatedFieldsDelete)
		for _, id := range cfIDs {
			cfRec, err := txApp.FindRecordById("calculated_fields", id)
			if err != nil {
				return apis.NewBadRequestError("Cascade delete failed", validation.Errors{
					"calculated_fields": validation.NewError("1008",
						fmt.Sprintf(
							"Cascade delete: calculated_field %q referenced by %s/%s not found.",
							id, ownerCol.Name, e.Record.Id,
						),
					),
				})
			}

			if err := txApp.Delete(cfRec); err != nil {
				return err
			}
		}

		// 3) continua con la delete dell'owner
		return e.Next()
	})
}

func CalculatedFieldsUpdateRequestGuard(e *core.RecordRequestEvent) error {
	// superuser bypass
	if e.Auth != nil && e.Auth.Collection() != nil && e.Auth.Collection().Name == "_superusers" {
		return e.Next()
	}

	cf := e.Record
	ownerCol := cf.GetString("owner_collection")
	ownerRow := cf.GetString("owner_row")

	if ownerCol == "" || ownerRow == "" {
		return apis.NewBadRequestError("Missing owner reference", validation.Errors{
			"owner": validation.NewError("1008", "owner_collection/owner_row are required"),
		})
	}

	ownerRec, err := e.App.FindRecordById(ownerCol, ownerRow)
	if err != nil {
		return apis.NewBadRequestError("Owner record not found", validation.Errors{
			cf.Id: validation.NewError(
				"1008",
				fmt.Sprintf("Invalid owner reference: record %s/%s not found.", ownerCol, ownerRow),
			),
		})
	}

	requestInfo, requestInfoErr := e.RequestInfo()
	if requestInfoErr != nil {
		return apis.NewInternalServerError(
			fmt.Sprintf(
				"Failed to retrieve request info while updating calculated_field %s (owner=%s/%s)",
				cf.Id, ownerCol, ownerRow,
			),
			requestInfoErr,
		)
	}

	// 1) UPDATE permission sull’owner
	canUpdate, ruleErr := e.App.CanAccessRecord(ownerRec, requestInfo, ownerRec.Collection().UpdateRule)
	if !canUpdate {
		authCol := ""
		authId := ""
		if e.Auth != nil {
			authId = e.Auth.Id
			if e.Auth.Collection() != nil {
				authCol = e.Auth.Collection().Name
			}
		}

		e.App.Logger().Warn("calculated_fields update forbidden: cannot update owner",
			"cfId", cf.Id,
			"ownerCollection", ownerCol,
			"ownerRow", ownerRow,
			"authCollection", authCol,
			"authId", authId,
			"ruleErr", fmt.Sprintf("%v", ruleErr),
		)

		return e.ForbiddenError(
			fmt.Sprintf(
				"Forbidden updating calculated_fields/%s: user %s/%s has no update access to owner %s/%s",
				cf.Id, authCol, authId, ownerCol, ownerRow,
			),
			ruleErr,
		)
	}

	// 2) VIEW permission sulla chiusura transitiva delle dipendenze della NUOVA formula
	//
	// NB: qui stai guardando “cosa userai per calcolare” -> quindi estrai gli identificatori
	// dalla formula nuova (quella in request) e fai BFS via depends_on.
	formula := cf.GetString("formula")
	startIds, err := extractIdentifiersFromFormula(formula)
	if err != nil {
		return apis.NewBadRequestError("Invalid formula", validation.Errors{
			"formula": validation.NewError("1004", fmt.Sprintf("Failed to parse formula identifiers: %v", err)),
		})
	}
	if len(startIds) > 0 {
		startRecs, err := e.App.FindRecordsByIds("calculated_fields", startIds)
		if err != nil {
			return apis.NewBadRequestError("Formula dependency error", validation.Errors{
				"formula": validation.NewError("1007", fmt.Sprintf("Failed to load referenced calculated_fields: %v", err)),
			})
		}

		// FindRecordsByIds può non dare errore ma restituire meno record -> trattalo come missing ref
		if len(startRecs) != len(startIds) {
			found := map[string]struct{}{}
			for _, r := range startRecs {
				if r != nil {
					found[r.Id] = struct{}{}
				}
			}
			missing := make([]string, 0, 4)
			for _, id := range startIds {
				if _, ok := found[id]; !ok {
					missing = append(missing, id)
				}
			}

			return apis.NewBadRequestError("Formula evaluation error: referenced variable not found", validation.Errors{
				"formula": validation.NewError("1007", fmt.Sprintf("Variable not found in dependency graph: %v", missing)),
			})
		}

		if err := assertDepsViewableTransitive(e.App, requestInfo, startRecs, e.Auth); err != nil {
			return err
		}
	}

	return e.Next()
}

func assertDepsViewableTransitive(
	app core.App,
	requestInfo *core.RequestInfo,
	frontier []*core.Record, // già popolata dal chiamante
	auth *core.Record,
) error {
	if len(frontier) == 0 {
		return nil
	}

	visited := make(map[string]struct{}, 128)

	authCol, authId := "", ""
	if auth != nil {
		authId = auth.Id
		if auth.Collection() != nil {
			authCol = auth.Collection().Name
		}
	}

	// queue di record (non ids)
	queue := make([]*core.Record, 0, len(frontier))
	for _, r := range frontier {
		if r == nil || strings.TrimSpace(r.Id) == "" {
			continue
		}
		if _, ok := visited[r.Id]; ok {
			continue
		}
		visited[r.Id] = struct{}{}
		queue = append(queue, r)
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		// 1) VIEW check sul nodo corrente
		rule := cur.Collection().ViewRule
		if rule != nil {
			ok, rerr := app.CanAccessRecord(cur, requestInfo, rule)
			if rerr != nil {
				return apis.NewInternalServerError(
					fmt.Sprintf("Failed to evaluate view access for calculated_fields/%s", cur.Id),
					rerr,
				)
			}
			if !ok {
				return apis.NewForbiddenError(
					fmt.Sprintf(
						"Forbidden: user %s/%s has no view access to dependency calculated_fields/%s",
						authCol, authId, cur.Id,
					),
					nil,
				)
			}
		}

		// 2) espandi parents via depends_on
		parentIds := cur.GetStringSlice("depends_on")
		if len(parentIds) == 0 {
			continue
		}

		parents, err := app.FindRecordsByIds("calculated_fields", parentIds)
		if err != nil {
			return apis.NewBadRequestError(
				"Formula dependency error: referenced record not found",
				validation.Errors{
					"formula": validation.NewError("1007", fmt.Sprintf("Failed to load depends_on records for %s", cur.Id)),
				},
			)
		}

		for _, p := range parents {
			if p == nil {
				continue
			}
			if _, ok := visited[p.Id]; ok {
				continue
			}
			visited[p.Id] = struct{}{}
			queue = append(queue, p)
		}
	}

	return nil
}

func CalculatedFieldsViewRequestGuard(e *core.RecordRequestEvent) error {
	// superuser bypass
	if e.Auth != nil && e.Auth.Collection() != nil && e.Auth.Collection().Name == "_superusers" {
		return e.Next()
	}

	cf := e.Record
	ownerCol := cf.GetString("owner_collection")
	ownerRow := cf.GetString("owner_row")

	if ownerCol == "" || ownerRow == "" {
		return e.ForbiddenError("Forbidden viewing calculated_field: missing owner reference", nil)
	}

	ownerRec, err := e.App.FindRecordById(ownerCol, ownerRow)
	if err != nil {
		return e.ForbiddenError(
			fmt.Sprintf("Forbidden viewing calculated_fields/%s: owner %s/%s not found", cf.Id, ownerCol, ownerRow),
			err,
		)
	}

	reqInfo, reqErr := e.RequestInfo()
	if reqErr != nil {
		return apis.NewInternalServerError(
			fmt.Sprintf("Failed to retrieve request info while viewing calculated_fields/%s (owner=%s/%s)", cf.Id, ownerCol, ownerRow),
			reqErr,
		)
	}

	// 1) l'utente deve poter vedere l'owner del CF
	ok, ruleErr := e.App.CanAccessRecord(ownerRec, reqInfo, ownerRec.Collection().ViewRule)
	if !ok {
		authCol, authId := "", ""
		if e.Auth != nil {
			authId = e.Auth.Id
			if e.Auth.Collection() != nil {
				authCol = e.Auth.Collection().Name
			}
		}

		e.App.Logger().Warn("calculated_fields view forbidden: cannot view owner",
			"cfId", cf.Id,
			"ownerCollection", ownerCol,
			"ownerRow", ownerRow,
			"authCollection", authCol,
			"authId", authId,
			"ruleErr", fmt.Sprintf("%v", ruleErr),
		)

		return e.ForbiddenError(
			fmt.Sprintf("Forbidden viewing calculated_fields/%s: user %s/%s has no view access to owner %s/%s",
				cf.Id, authCol, authId, ownerCol, ownerRow,
			),
			ruleErr,
		)
	}

	// 2) se l'owner è viewable, allora il record è viewable,
	//    ma mascheriamo value/error se una dipendenza (transitiva) non è viewable via owner
	masked, blockedAt, maskErr := maskIfDepsNotViewable(e.App, reqInfo, cf)
	if maskErr != nil {
		return apis.NewInternalServerError(
			fmt.Sprintf("Failed to evaluate dependency access while viewing calculated_fields/%s", cf.Id),
			maskErr,
		)
	}
	if masked {
		// NB: value è JSON-encoded string nel tuo schema
		cf.Set("value", "\"#AUTH!\"")
		cf.Set("error", fmt.Sprintf("Not authorized to read one or more dependencies (first blocked: %s)", blockedAt))
	}

	return e.Next()
}

func CalculatedFieldsListRequestGuard(e *core.RecordsListRequestEvent) error {
	// superuser bypass
	if e.Auth != nil && e.Auth.Collection() != nil && e.Auth.Collection().Name == "_superusers" {
		return e.Next()
	}

	reqInfo, reqErr := e.RequestInfo()
	if reqErr != nil {
		return apis.NewInternalServerError(
			"Failed to retrieve request info while listing calculated_fields",
			reqErr,
		)
	}

	// esegui query base
	if err := e.Next(); err != nil {
		return err
	}

	filtered := make([]*core.Record, 0, len(e.Records))

	for _, cf := range e.Records {
		ownerCol := cf.GetString("owner_collection")
		ownerRow := cf.GetString("owner_row")
		if ownerCol == "" || ownerRow == "" {
			continue
		}

		ownerRec, err := e.App.FindRecordById(ownerCol, ownerRow)
		if err != nil {
			continue
		}

		// 🔐 HARD GATE: serve VIEW sull’owner diretto
		ok, _ := e.App.CanAccessRecord(ownerRec, reqInfo, ownerRec.Collection().ViewRule)
		if !ok {
			continue
		}

		// 🛡️ MASK se una dipendenza non è viewable
		masked, blockedAt, err := maskIfDepsNotViewable(e.App, reqInfo, cf)
		if err != nil {
			return err
		}

		if masked {
		cf.Set("value", "\"#AUTH!\"")
			cf.Set(
				"error",
				fmt.Sprintf(
					"Not authorized to read one or more dependencies (blocked at %s)",
					blockedAt,
				),
			)
		}

		filtered = append(filtered, cf)
	}

	e.Records = filtered
	return nil
}

func maskIfDepsNotViewable(
	app core.App,
	reqInfo *core.RequestInfo,
	root *core.Record,
) (masked bool, blockedAt string, err error) {

	queue := []*core.Record{root}
	visited := map[string]struct{}{root.Id: {}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		// espandi depends_on
		errs := app.ExpandRecord(cur, []string{"depends_on"}, nil)
		if len(errs) != 0 {
			return false, "", fmt.Errorf("failed to expand depends_on: %v", errs)
		}

		for _, dep := range cur.ExpandedAll("depends_on") {
			if _, seen := visited[dep.Id]; seen {
				continue
			}
			visited[dep.Id] = struct{}{}

			ownerCol := dep.GetString("owner_collection")
			ownerRow := dep.GetString("owner_row")
			if ownerCol == "" || ownerRow == "" {
				return true, dep.Id, nil
			}

			ownerRec, err := app.FindRecordById(ownerCol, ownerRow)
			if err != nil {
				return true, dep.Id, nil
			}

			ok, _ := app.CanAccessRecord(ownerRec, reqInfo, ownerRec.Collection().ViewRule)
			if !ok {
				return true, dep.Id, nil
			}

			queue = append(queue, dep)
		}
	}

	return false, "", nil
}

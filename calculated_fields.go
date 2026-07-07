package calculatedfields

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/file"
	"github.com/ganigeorgiev/fexpr"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// se chiamato senza parametri registra tutto altrimenti le funzioni indicate nei parametri
func BindCalculatedFieldsHooks(app core.App) error {
	app.OnCollectionValidate().BindFunc(CalculatedFieldsOwnersSchemaGuards)
	app.OnRecordViewRequest("calculated_fields").BindFunc(CalculatedFieldsViewRequestGuard)
	app.OnRecordsListRequest("calculated_fields").BindFunc(CalculatedFieldsListRequestGuard) // o l’equivalente nella tua versione

	app.OnRecordCreate().BindFunc(OnOwnerCreate_AutoCreateCalculatedFields)
	app.OnRecordDelete().BindFunc(OnOwnerDelete_AutoDeleteCalculatedFields)

	app.OnRecordCreate("calculated_fields").BindFunc(OnCalculatedFieldsCreateUpdate)
	// user può fare update sul record solo se può fare update sull'owner
	app.OnRecordUpdateRequest("calculated_fields").BindFunc(CalculatedFieldsUpdateRequestGuard)
	app.OnRecordUpdate("calculated_fields").BindFunc(OnCalculatedFieldsCreateUpdate)
	app.OnRecordDelete("calculated_fields").BindFunc(OnCalculatedFieldsDelete)
	return nil
}

func OnCalculatedFieldsCreateUpdate(e *core.RecordEvent) error {
	e.App.Logger().Debug("called OnCalculatedFieldsCreateUpdate")
	originalApp := e.App
	txErr := originalApp.RunInTransaction(func(txApp core.App) error {
		e.App = txApp
		//salviamo prima il nuovo record e poi calcoliamo le formule in una transaction
		//se fallisce fa il rollback di tutto
		if err := e.Next(); err != nil {
			return err
		}

		orig := e.Record.Original()
		origFormula := orig.GetString("formula")
		newFormula := e.Record.GetString("formula")

		if newFormula == origFormula {
			// niente ricalcolo, niente touch owner
			return nil
		}

		if orig.GetString("owner_collection") != "" || orig.GetString("owner_row") != "" || orig.GetString("owner_field") != "" {
			if e.Record.GetString("owner_collection") != orig.GetString("owner_collection") ||
				e.Record.GetString("owner_row") != orig.GetString("owner_row") ||
				e.Record.GetString("owner_field") != orig.GetString("owner_field") {
				return apis.NewBadRequestError("Cannot change calculated_field owner", validation.Errors{
					"owner": validation.NewError("1010", "owner_collection/owner_row/owner_field are immutable once set"),
				})
			}
		}

		// 1️⃣ Normalizza formule e aggiorna referenced_queues e salva nella transazione
		var err error
		var init_env map[string]any
		if init_env, err = ResolveDepsAndTxSave(txApp, e.Record); err == nil {
			err = evaluateFormulaGraph(txApp, e.Record, init_env)
		}
		return err
	})
	e.App = originalApp
	return txErr
}

func OnCalculatedFieldsDelete(e *core.RecordEvent) error {
	deletedRecord := e.Record
	originalApp := e.App
	txErr := originalApp.RunInTransaction(func(txApp core.App) error {
		e.App = txApp
		// Espandi i figli diretti del nodo eliminato
		if err := expandFormulaDependencies(txApp, deletedRecord); err != nil {
			return err
		}

		//transitiveDepQueues := []*core.Record{}
		visited := map[string]struct{}{}

		//cicla i nodi direttamente dipendenti, aggiorna i campi e crea transitiveDepQueues
		for _, direct := range e.Record.ExpandedAll("calculated_fields_via_depends_on") {
			visited[direct.Id] = struct{}{}
			formula := direct.GetString("formula")
			updatedFormula := strings.ReplaceAll(formula, deletedRecord.Id, "#REF!")
			direct.Set("formula", updatedFormula)

			// Rimuovi il riferimento dal depends_on
			newDepends := []string{}
			for _, dep := range direct.GetStringSlice("depends_on") {
				if dep != deletedRecord.Id {
					newDepends = append(newDepends, dep)
				}
			}
			if err := applyResultAndSave(txApp, direct, "#REF!", "Reference to deleted node", map[string]any{}, newDepends); err != nil {
				return err
			}
			if err := evaluateFormulaGraph(txApp, direct, map[string]any{}); err != nil {
				return err
			}
		}
		return e.Next()
	})

	e.App = originalApp
	return txErr
}

var formulaReservedRegex = regexp.MustCompile(`(?i)\b(true|false|null|if|else|in)\b|#(NAME\?|REF!|VALUE!|NUM!|DIV/0!|N/A|NULL!)`)

// risolve le dipendenze fa un salvataggio intermedio e restituisce l'env per il calcolo
func ResolveDepsAndTxSave(app core.App, rec *core.Record) (map[string]any, error) {
	// 1️⃣ Estrazione delle variabili dalla formula
	formula := formulaReservedRegex.ReplaceAllString(rec.GetString("formula"), "0")
	identifiers, err := extractIdentifiersFromFormula(formula)
	if err != nil {
		return nil, err
	}

	//se non ci sono identificatori non serve proseguire e si può restituire la mappa vuota
	if len(identifiers) == 0 {
		return map[string]any{}, nil
	}

	parentIds := make([]string, 0, len(identifiers))
	for _, id := range identifiers {
		if id == rec.Id {
			return nil, apis.NewBadRequestError(
				"Formula dependency error: self-reference",
				validation.Errors{
					rec.Id: validation.NewError("1002", fmt.Sprintf("Self-reference detected on %s with formula %s", rec.Id, formula)),
				},
			)
		}
		parentIds = append(parentIds, id)
	}

	// 2️⃣ Verifica che i record referenziati esistano
	env_init_list, err := app.FindRecordsByIds("calculated_fields", parentIds)
	if err != nil {
		return nil, apis.NewBadRequestError(
			fmt.Sprintf("Failed to find referenced records %v", parentIds),
			validation.Errors{
				rec.Id: validation.NewError("1005", fmt.Sprintf("Reference in formula %s not found", rec.Id))},
		)
	}
	if len(env_init_list) != len(parentIds) {

		found := make(map[string]bool, len(env_init_list))
		for _, r := range env_init_list {
			found[r.Id] = true
		}

		var missing []string
		for _, pid := range parentIds {
			if !found[pid] {
				missing = append(missing, pid)
			}
		}

		return nil, apis.NewBadRequestError(
			fmt.Sprintf("Formula evaluation error: referenced variable not found: %v", missing),
			validation.Errors{
				"formula": validation.NewError(
					"1007",
					fmt.Sprintf("Variable not found in DAG during evaluation: %v", missing),
				),
			},
		)
	}

	// 3️⃣ Salva le dipendenze aggiornate
	rec.Set("depends_on", parentIds)
	if err := app.UnsafeWithoutHooks().Save(rec); err != nil {
		return map[string]any{}, fmt.Errorf("failed to save updated record: %w", err)
	}

	env_init := map[string]any{}
	if _, err = populateEnvAndCheckRef(env_init, env_init_list); err != nil {
		return nil, err
	}

	return env_init, nil
}

func applyResultAndSave(txApp core.App, node *core.Record, value any, errMsg string, env map[string]any, newDepends []string) error {
	b, err := json.Marshal(value)
	if err != nil {
		return apis.NewBadRequestError("Failed to serialize calculated value", validation.Errors{
			node.Id: validation.NewError("1012", fmt.Sprintf("Invalid computed value for %s: %v", node.Id, err)),
		})
	}

	jsonValue := string(b)

	node.Set("value", jsonValue)
	node.Set("error", errMsg)
	if newDepends != nil {
		node.Set("depends_on", newDepends)
	}
	env[node.Id] = value

	if err := txApp.UnsafeWithoutHooks().Save(node); err != nil {
		return fmt.Errorf("errore salvataggio queue %s: %v", node.Id, err)
	}
	//---UPDATE OWNER UPDATED FIELD IF PRESENT
	// --- TOUCH OWNER.updated (deterministic, strict)
	ownerCol := node.GetString("owner_collection")
	ownerRow := node.GetString("owner_row")

	// se non c'è owner, non tocchiamo nulla (ma puoi decidere di renderlo errore)
	if ownerCol == "" || ownerRow == "" {
		return nil
	}

	ownerRec, err := txApp.FindRecordById(ownerCol, ownerRow)
	if err != nil {
		return apis.NewBadRequestError("owner record not found", validation.Errors{
			node.Id: validation.NewError("1008",
				fmt.Sprintf("Invalid owner reference: record %s/%s not found.", ownerCol, ownerRow)),
		})
	}

	// Aggiornamento deterministico
	ownerRec.Set("updated", types.NowDateTime())

	if err := txApp.Save(ownerRec); err != nil {
		return apis.NewBadRequestError("Failed to update owner 'updated' field", validation.Errors{
			node.Id: validation.NewError("1008",
				fmt.Sprintf("Failed to touch owner %s/%s.updated: %v", ownerCol, ownerRow, err)),
		})
	}

	return nil
}

func extractIdentifiersFromFormula(formula string) ([]string, error) {
	formula = formulaReservedRegex.ReplaceAllString(formula, "0")
	scanner := fexpr.NewScanner([]byte(formula))
	identifiersMap := map[string]struct{}{}
	tokens := []fexpr.Token{}

	for token, _ := scanner.Scan(); token.Type != fexpr.TokenEOF; token, _ = scanner.Scan() {
		tokens = append(tokens, token)
	}

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.Type == fexpr.TokenIdentifier {
			id := strings.SplitN(t.Literal, ".", 2)[0]
			identifiersMap[id] = struct{}{}
		}
		if nested, ok := t.Meta.([]fexpr.Token); ok {
			tokens = append(tokens, nested...)
		}
	}

	result := make([]string, 0, len(identifiersMap))
	for id := range identifiersMap {
		result = append(result, id)
	}

	return result, nil
}

func evaluateFormulaGraph(txApp core.App, node *core.Record, env map[string]any) error {
	//make sure the rec is expanded
	if err := expandFormulaDependencies(txApp, node); err != nil {
		return err
	}
	//calcola il nodo imputato
	rootResult, rootErrMsg, rootEvalErr := evalFormula(txApp, node, env)
	if rootEvalErr != nil {
		return rootEvalErr
	}

	if !isDirty(node, rootResult, rootErrMsg) {
		return nil
	}
	if err := applyResultAndSave(txApp, node, rootResult, rootErrMsg, env, nil); err != nil {
		return err
	}
	// ---------
	children := node.ExpandedAll("calculated_fields_via_depends_on")
	//------------
	for i := 0; i < len(children); i++ {
		child := children[i]
		if child.Id == node.Id {
			return apis.NewBadRequestError(
				fmt.Sprintf("Formula dependency error: circular reference found (%s → %s)", child.Id, node.Id),
				validation.Errors{
					child.Id: validation.NewError("1003", fmt.Sprintf("Detected circular dependency between %s and %s", child.Id, node.Id)),
				},
			)
		}

		if err := expandFormulaDependencies(txApp, child); err != nil {
			return err
		}
		//espando i dipendenti
		dependOnRecords := child.ExpandedAll("depends_on")
		var childResult any
		var childEvalError string
		var childErr error
		hasRefErrVal, err := populateEnvAndCheckRef(env, dependOnRecords)
		if err != nil {
			return err
		}

		if hasRefErrVal {
			childResult = "#REF!"
			childEvalError = "Reference to deleted node"
		} else {
			if childResult, childEvalError, childErr = evalFormula(txApp, child, env); childErr != nil {
				return childErr
			}
		}

		if isDirty(child, childResult, childEvalError) {
			if err := applyResultAndSave(txApp, child, childResult, childEvalError, env, nil); err != nil {
				return err
			}
		}

		// 🔹 3️⃣ Espansione BFS verso i figli
		grandChildren := child.ExpandedAll("calculated_fields_via_depends_on")
		for _, grandChild := range grandChildren {

			children = append(children, grandChild)
		}
	}
	return nil
}
func populateEnvAndCheckRef(env map[string]any, records []*core.Record) (hasRef bool, err error) {
	for _, rec := range records {
		var v any
		if err = json.Unmarshal([]byte(rec.GetString("value")), &v); err != nil {
			return false, fmt.Errorf("invalid JSON in value of %s: %v", rec.Id, err)
		}
		env[rec.Id] = v
		if s, ok := v.(string); ok && s == "#REF!" {
			return true, nil
		}
	}
	return false, nil
}

func evalFormula(txApp core.App, node *core.Record, env map[string]any) (any, string, error) {
	formula := node.GetString("formula")

	if strings.Contains(formula, "#REF!") {
		return "#REF!", "Formula contains reference to missing node (#REF!)", nil
	}
	//compila in modo da evidenziare errori di sintassi
	program, err := expr.Compile(node.GetString("formula"))
	if err != nil {
		return "", "", apis.NewBadRequestError(
			fmt.Sprintf("Syntax error in formula %s", node.Id),
			validation.Errors{
				node.Id: validation.NewError("1004", fmt.Sprintf("Syntax error in formula: %v", err))})
	}
	// 🔹 Esegui la formula
	result, err := expr.Run(program, env)
	if err != nil {
		return translateFormulaError(txApp, node, err)
	}
	//gestire il risultato infinito ad esempio divisione per zero
	if f, ok := result.(float64); ok {
		// Divisione per zero → #DIV/0!
		if math.IsInf(f, 0) {
			return "#DIV/0!", "Divisione per zero o risultato infinito", nil
		}
		// Risultato numerico non valido → #NUM!
		if math.IsNaN(f) {
			return "#NUM!", "Risultato numerico non valido (NaN)", nil
		}
	}
	return result, "", nil
}

/*
╔════════════╦══════════════════════════════════════════╗
║ Excel Code ║             Description                  ║
╠════════════╬══════════════════════════════════════════╣
║ #NAME?     ║ Funzione non definita o non supportata    ║
║ #REF!      ║ Variabile o riferimento non trovato      ║
║ #VALUE!    ║ Tipo di argomento errato                 ║
║ #NUM!      ║ Numero fuori range o valore non numerico ║
║ #DIV/0!    ║ Divisione per zero                       ║
║ #N/A       ║ alore non disponibile o nullo            ║
║ #NULL!     ║ Intersezione non valida tra intervalli   ║
╚════════════╩══════════════════════════════════════════╝
*/
func translateFormulaError(txApp core.App, node *core.Record, err error) (any, string, error) {
	ferr, ok := err.(*file.Error)
	if !ok {
		return "", "", apis.NewBadRequestError(
			fmt.Sprintf("Failed to evaluate formula for %s", node.Id),
			validation.Errors{
				node.Id: validation.NewError("1006", fmt.Sprintf("Evaluation error in formula %s: %v", node.Id, err))})
	}

	switch {
	case strings.Contains(ferr.Message, "invalid operation: cannot call nil"):
		return "#NAME?", "Funzione non riconosciuta o non definita", nil

	case strings.Contains(ferr.Message, "invalid operation") && strings.Contains(ferr.Message, "<nil>"):
		if _, dep_err := ResolveDepsAndTxSave(txApp, node); dep_err != nil {
			return "", "", dep_err
		}
		return "#N/A", "Valore non disponibile (null) in operazione", nil

	case strings.Contains(ferr.Message, "invalid operation"):
		return "#VALUE!", "Tipo non compatibile nell'operazione", nil

	default:
		return "#VALUE!", ferr.Message, nil
	}
}

func expandFormulaDependencies(app core.App, node *core.Record) error {
	errs := app.ExpandRecord(node, []string{"calculated_fields_via_depends_on", "depends_on"}, nil)
	if len(errs) != 0 {
		return fmt.Errorf("failed to expand referenced queues: %v", errs)
	}
	return nil
}

func isDirty(node *core.Record, value any, errMsg string) bool {
	b, err := json.Marshal(value)
	if err != nil {
		// se non serializza, consideriamolo dirty così almeno scrivi l’errore a db
		return true
	}
	return string(b) != node.Original().GetString("value") ||
		errMsg != node.Original().GetString("error")
}

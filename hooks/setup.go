package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func EnsureCalculatedFieldsSystemSchema(app core.App) error {
	const name = "_calculated_fields"

	// 1) Find or create empty collection
	col, _ := app.FindCollectionByNameOrId(name)
	if col == nil {
		col = core.NewBaseCollection(name)

		// minimal base props (then we will set proper ones below too)
		col.Name = name
		col.Type = core.CollectionTypeBase
		col.System = true

		if err := app.Save(col); err != nil {
			return fmt.Errorf("cannot create %s collection: %w", name, err)
		}
	}

	// 2) Enforce base props (system collection)
	// NOTE: "System=true" makes it locked/internal. It's still queryable by API
	// if you explicitly allow it via rules/handlers, which you are doing in tests.
	col.System = true
	col.Type = core.CollectionTypeBase
	col.Name = name

	// Rules (same spirit as your dump)
	col.ListRule = types.Pointer(`@request.auth.id != ""`)
	col.ViewRule = types.Pointer(`@request.auth.id != ""`)
	col.CreateRule = nil
	col.UpdateRule = types.Pointer(`@request.auth.id != ""`)
	col.DeleteRule = nil

	// 3) Fields: get-or-create minimal, then set properties in-place (no remove!)
	//    This preserves field IDs -> preserves stored data.

	// formula (TextField, required)
	{
		f := col.Fields.GetByName("formula")
		if f == nil {
			col.Fields.Add(&core.TextField{Name: "formula"})
			f = col.Fields.GetByName("formula")
		}
		tf, ok := f.(*core.TextField)
		if !ok {
			return fmt.Errorf("field 'formula' exists but is not TextField (got %T)", f)
		}
		tf.Name = "formula"
		tf.Required = true
	}

	// value (JSONField)
	{
		f := col.Fields.GetByName("value")
		if f == nil {
			col.Fields.Add(&core.JSONField{Name: "value"})
			f = col.Fields.GetByName("value")
		}
		jf, ok := f.(*core.JSONField)
		if !ok {
			return fmt.Errorf("field 'value' exists but is not JSONField (got %T)", f)
		}
		jf.Name = "value"
		// jf.Required default false is fine
	}

	// error (TextField)
	{
		f := col.Fields.GetByName("error")
		if f == nil {
			col.Fields.Add(&core.TextField{Name: "error"})
			f = col.Fields.GetByName("error")
		}
		tf, ok := f.(*core.TextField)
		if !ok {
			return fmt.Errorf("field 'error' exists but is not TextField (got %T)", f)
		}
		tf.Name = "error"
		tf.Required = false
	}

	// owner_collection (TextField, required)
	{
		f := col.Fields.GetByName("owner_collection")
		if f == nil {
			col.Fields.Add(&core.TextField{Name: "owner_collection"})
			f = col.Fields.GetByName("owner_collection")
		}
		tf, ok := f.(*core.TextField)
		if !ok {
			return fmt.Errorf("field 'owner_collection' exists but is not TextField (got %T)", f)
		}
		tf.Name = "owner_collection"
		tf.Required = true
	}

	// owner_row (TextField, required)
	{
		f := col.Fields.GetByName("owner_row")
		if f == nil {
			col.Fields.Add(&core.TextField{Name: "owner_row"})
			f = col.Fields.GetByName("owner_row")
		}
		tf, ok := f.(*core.TextField)
		if !ok {
			return fmt.Errorf("field 'owner_row' exists but is not TextField (got %T)", f)
		}
		tf.Name = "owner_row"
		tf.Required = true
	}

	// owner_field (TextField, required)
	{
		f := col.Fields.GetByName("owner_field")
		if f == nil {
			col.Fields.Add(&core.TextField{Name: "owner_field"})
			f = col.Fields.GetByName("owner_field")
		}
		tf, ok := f.(*core.TextField)
		if !ok {
			return fmt.Errorf("field 'owner_field' exists but is not TextField (got %T)", f)
		}
		tf.Name = "owner_field"
		tf.Required = true
	}

	// depends_on (RelationField to self)
	{
		f := col.Fields.GetByName("depends_on")
		if f == nil {
			col.Fields.Add(&core.RelationField{Name: "depends_on"})
			f = col.Fields.GetByName("depends_on")
		}
		rf, ok := f.(*core.RelationField)
		if !ok {
			return fmt.Errorf("field 'depends_on' exists but is not RelationField (got %T)", f)
		}
		rf.Name = "depends_on"
		rf.CollectionId = col.Id // self relation MUST reference final col.Id
		rf.MaxSelect = 999
		rf.MinSelect = 0
		rf.Required = false
		rf.CascadeDelete = false
	}

	// 4) Indexes: reset + apply known set (safe to overwrite)
	//    NOTE: table name equals collection name for base collections.
	//    If PocketBase ever changes table naming, you'll need to adjust.
	col.Indexes = types.JSONArray[string]{
		"CREATE UNIQUE INDEX IF NOT EXISTS `idx_cf_owner_triplet_unique` ON `_calculated_fields` (\n  `owner_collection`,\n  `owner_row`,\n  `owner_field`\n)",
		"CREATE INDEX IF NOT EXISTS `idx_cf_owner_row` ON `_calculated_fields` (`owner_row`)",
		"CREATE INDEX IF NOT EXISTS `idx_cf_owner_collection` ON `_calculated_fields` (`owner_collection`)",
		"CREATE INDEX IF NOT EXISTS `idx_cf_owner_field` ON `_calculated_fields` (`owner_field`)",
	}

	// 5) Save final schema
	if err := app.Save(col); err != nil {
		return fmt.Errorf("cannot save %s schema: %w", name, err)
	}

	return nil
}
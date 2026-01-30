package calculatedfields

import (
	"calculatedfields/hooks"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbuilds/xpb"
)

type Plugin struct {}

func init() {
	xpb.Register(&Plugin{})
}

// Name implements xpb.Plugin.
func (p *Plugin) Name() string {
	return "calculatedfields"
}

// This variable will automatically be set at build time by xpb
var version string

// Version implements xpb.Plugin.
func (p *Plugin) Version() string {
	return version
}

// Description implements xpb.Plugin.
func (p *Plugin) Description() string {
		return "Excel-style calculated fields for PocketBase: reactive formulas with dependency graph, auto-create per-owner fields, and owner touch updates."

}

// Init implements xpb.Plugin.
func (p *Plugin) Init(app core.App) error {
	// 1) Fail-fast / enforce schema
	if err := hooks.EnsureCalculatedFieldsSystemSchema(app); err != nil {
		// log utile + error di ritorno
		app.Logger().Error("calculatedfields: schema ensure failed", "err", err)
		return fmt.Errorf("calculatedfields: schema ensure failed: %w", err)
	}

	// 2) Register hooks
	if err := hooks.BindCalculatedFieldsHooks(app); err != nil {
		app.Logger().Error("calculatedfields: bind hooks failed", "err", err)
		return fmt.Errorf("calculatedfields: bind hooks failed: %w", err)
	}

	return nil
}


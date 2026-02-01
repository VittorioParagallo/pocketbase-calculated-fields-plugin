package calculatedfields

import (
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
	// 1) Ensure schema when DB is ready
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		// IMPORTANT: execute PB bootstrap first so DB/DAO are ready,
		// then ensure our schema.
		if err := e.Next(); err != nil {
			return err
		}
		if err := EnsureCalculatedFieldsSystemSchema(app); err != nil {
			return fmt.Errorf("calculatedfields: schema ensure failed: %w", err)
		}
		return nil
	})

	// 2) Register hooks (ok to bind now; they will run later)
	if err := BindCalculatedFieldsHooks(app); err != nil {
		return fmt.Errorf("calculatedfields: bind hooks failed: %w", err)
	}

	return nil
}


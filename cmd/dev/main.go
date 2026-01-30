package calculatedfields

import (
	"github.com/pocketbase/pocketbase"
	"log"
	"calculatedfields/hooks"
)

func main() {
	app := pocketbase.New()

	if err := hooks.EnsureCalculatedFieldsSystemSchema(app); err != nil {
		log.Fatal(err)
	}

	if err := hooks.BindCalculatedFieldsHooks(app); err != nil {
		log.Fatal(err)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

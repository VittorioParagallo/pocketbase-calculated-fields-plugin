package main

import (
	"log"
	"myapp/hooks"
	"github.com/pocketbase/pocketbase"
)

func main() {
	app := pocketbase.New()
    hooks.BindCalculatedFieldsHooks(app)
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}

}

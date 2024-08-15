package main

import (
	"log"
	"os"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)




func main() {
    app := pocketbase.New()

    // serves static files from the provided public dir (if exists)
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))
        return nil
    })

	app.OnModelAfterCreate("vpn_configs").Add(func(e *core.ModelEvent) error {
		app.Dao().DB().NewQuery(`UPDATE vpn_configs SET outlineConnection={:outlineLink} WHERE id={:id}`).
			Bind(dbx.Params{ "id": e.Model.GetId(), "outlineLink": "test"}).Execute()
		return nil
	})

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
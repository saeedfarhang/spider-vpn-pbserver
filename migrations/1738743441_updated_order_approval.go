package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_3505586701")
		if err != nil {
			return err
		}

		// update collection data
		if err := json.Unmarshal([]byte(`{
			"createRule": "",
			"listRule": "",
			"updateRule": null,
			"viewRule": ""
		}`), &collection); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_3505586701")
		if err != nil {
			return err
		}

		// update collection data
		if err := json.Unmarshal([]byte(`{
			"createRule": "is_approved = false && is_fraud = false",
			"listRule": "@request.auth.is_admin = true && is_approved = false && is_fraud = false",
			"updateRule": "@request.auth.is_admin = true && is_approved = false && is_fraud = false",
			"viewRule": "@request.auth.is_admin = true && is_approved = false && is_fraud = false"
		}`), &collection); err != nil {
			return err
		}

		return app.Save(collection)
	})
}

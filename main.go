package main

import (
	"log"
	"os"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)




func main() {
    app := pocketbase.New()

    // serves static files from the provided public dir (if exists)
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))
        return nil
    })


	app.OnModelAfterCreate("orders").Add(func( e *core.ModelEvent) error {
		app.Dao().DB().NewQuery(`UPDATE orders SET status="INCOMPLETE" WHERE id={:id}`).
			Bind(dbx.Params{ "id": e.Model.GetId()}).Execute()
			
		orderId := e.Model.GetId()
		order := e.Model.(*models.Record)
		// Fetch the related plan's ID
		planId := order.GetString("plan")
		gatewayId := order.GetString("payment_gateway")

		// Fetch related pricing records for the plan
		pricings := []*models.Record{}
		err := app.Dao().DB().NewQuery(`
			SELECT pricing.* 
			FROM pricing 
			JOIN plans_pricing ON plans_pricing.pricing = pricing.id 
			WHERE plans_pricing.plan = {:planId} AND plans_pricing.gateway = {:gatewayId}
		`).Bind(dbx.Params{"planId": planId, "gatewayId": gatewayId}).All(&pricings)
		if err != nil {
			return err
		}

		// Create payments for each related pricing
		for _, pricing := range pricings {
			pricing, err := app.Dao().FindRecordById("pricing", pricing.Id)
			if err != nil {
				return err
			}
			
			payments , err := app.Dao().FindCollectionByNameOrId("payments")
			payment := models.NewRecord(payments)
			if err != nil {
				return err
			}
			
			payment.Set("user", order.GetString("user"))
			log.Println(pricing.Get("id"), pricing.GetFloat("price"),order.GetString("user"),pricing.GetString("currency"))
			payment.Set("order", orderId)
			payment.Set("amount", pricing.GetFloat("price"))
			payment.Set("currency", pricing.GetString("currency"))
			payment.Set("status", "UNPAID")

			// Insert the payment into the database
			if err := app.Dao().Save(payment); err != nil {
				return err
			}
		}
		return nil
	})

	// create outline vpn config base on selected plan
	app.OnModelAfterCreate("vpn_configs").Add(func(e *core.ModelEvent) error {
		app.Dao().DB().NewQuery(`UPDATE vpn_configs SET outlineConnection={:outlineLink} WHERE id={:id}`).
			Bind(dbx.Params{ "id": e.Model.GetId(), "outlineLink": "test"}).Execute()
		return nil
	})

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
	
}
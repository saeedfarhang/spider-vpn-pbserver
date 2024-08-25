package main

import (
	"log"
	"os"
	outlineApi "spider-vpn/wrappers/outline/api"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)


type AccessKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Password  string `json:"password"`
	Port      int    `json:"port"`
	Method    string `json:"method"`
	AccessUrl string `json:"accessUrl"`
	DataLimit struct {
		Bytes int64 `json:"bytes"`
	} `json:"dataLimit,omitempty"`
}


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

	app.OnModelAfterCreate("order_approval").Add(func( e *core.ModelEvent) error {
		order_approval := e.Model.(*models.Record)
		orderId := order_approval.GetString("order")
		app.Dao().DB().NewQuery(`UPDATE orders SET status="WAIT_FOR_APPROVE", order_approval={:order_approval_id} WHERE id={:order_id}`).
			Bind(dbx.Params{ "order_id": orderId, "order_approval_id": order_approval.Id}).Execute()
		return nil
	})

	app.OnModelAfterUpdate("orders").Add(func( e *core.ModelEvent) error {
		a, err := outlineApi.ListAccessKeys()
		log.Print(a)
		if err != nil{
			return err
		}
		return nil
	})

	app.OnModelAfterUpdate("order_approval").Add(func( e *core.ModelEvent) error {
		order_approval := e.Model.(*models.Record)
		orderId := order_approval.GetString("order")
		payment, err := app.Dao().FindFirstRecordByData("payments", "order", orderId)
		if err != nil {
			return err
		}
		is_approved := order_approval.GetBool(("is_approved"))
		if (is_approved){
			log.Print(is_approved, payment)
			app.Dao().DB().NewQuery(`UPDATE orders SET status="COMPLETE" WHERE id={:order_id};
								UPDATE payments SET status="PAID" WHERE id = {:payment_id}`).
			Bind(dbx.Params{ "order_id": orderId, "payment_id": payment.Id}).Execute()
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

// func OutlineApiCall(method string, url string, result any)(any, error){
// 	tr := &http.Transport{
// 		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
// 	}
// 	client := &http.Client{Transport: tr}

// 	req, err := http.NewRequest(method, url, nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}

// 	defer resp.Body.Close()

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, err
// 	}
// 	err = json.Unmarshal(body, &result)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// func ListAccessKeys() ([]AccessKey, error) {
// 	var result struct {
// 		AccessKeys []AccessKey `json:"accessKeys"`
// 	}
// 	resp, err := OutlineApiCall("GET","https://backup.heycodinguy.site:3843/tVhaFf05N6k8tHKXk3UZ-w/access-keys/", &result)
// 	if err != nil {
// 		fmt.Println("Error:", err)
// 		return nil, err
// 	}
// 	accessKeysResult, ok := resp.(*struct {
// 		AccessKeys []AccessKey `json:"accessKeys"`
// 	})
// 	if !ok {
// 		fmt.Println("Error: Type assertion failed")
// 		return nil, err
// 	}

// 	return accessKeysResult.AccessKeys, nil
// }
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	env "spider-vpn/config"
	helpers "spider-vpn/helpers"
	outlineApi "spider-vpn/wrappers/outline/api"
	tgbot "spider-vpn/wrappers/tg-bot"
	"strings"
	"time"

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
	tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")
    app := pocketbase.New()
    // serves static files from the provided public dir (if exists)
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))
        return nil
    })


	app.OnModelAfterCreate("orders").Add(func( e *core.ModelEvent) error {
		orderId := e.Model.GetId()
		order := e.Model.(*models.Record)
		// Fetch the related plan's ID
		planId := order.GetString("plan")
		gatewayId := order.GetString("payment_gateway")
		gateway, err := app.Dao().FindRecordById("payment_gateway", gatewayId)
		if err !=nil{
			return err
		}
		app.Dao().DB().NewQuery(`UPDATE orders SET status="INCOMPLETE" WHERE id={:id}`).
			Bind(dbx.Params{ "id": e.Model.GetId()}).Execute()
			
		// Fetch related pricing records for the plan
		pricings := []*models.Record{}
		err = app.Dao().DB().NewQuery(`
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
			payment.Set("order", orderId)
			payment.Set("amount", pricing.GetFloat("price"))
			payment.Set("currency", pricing.GetString("currency"))
			if err := app.Dao().Save(payment); err != nil {
				return err
			}
			
			if gateway.GetString("type") == "FREE"{
				user, err := app.Dao().FindRecordById("users", order.GetString("user"))
				if err != nil {
					return err
				}
				if user.GetBool("first_test_done"){
					_, err = tgbot.SendVpnConfig(tgbotWebhookServer, user.Username(), "Nil")
					if err != nil{
						return err
					}
					return fmt.Errorf("duplicate test account")
				}
				payment.Set("status", "PAID")
				if err := app.Dao().Save(payment); err != nil {
					return err
				}
				order.Set("status", "COMPLETE")
				if err := app.Dao().Save(order); err != nil {
					return err
				}
				user.Set("first_test_done", true)
				if err := app.Dao().Save(user); err != nil {
					return err
				}
				
			}else{
				payment.Set("status", "UNPAID")
				if err := app.Dao().Save(payment); err != nil {
					return err
				}
			}
		}
		return nil
	})

	app.OnModelAfterCreate("order_approval").Add(func( e *core.ModelEvent) error {
		order_approval := e.Model.(*models.Record)
		orderId := order_approval.GetString("order")
		order, err := app.Dao().FindRecordById("order", orderId)
		if err!= nil {
			return nil
		}
		order.Set("status", "WAIT_FOR_APPROVE")
		order.Set("order_approval", order_approval.Id)
		if err := app.Dao().SaveRecord(order); err != nil {
			return err
		}
		return nil
	})

	app.OnModelAfterUpdate("orders").Add(func(e *core.ModelEvent) error {
		order, ok := e.Model.(*models.Record)
		if !ok {
			log.Print("Error: Could not cast model to *models.Record")
			return fmt.Errorf("model casting error")
		}
	
		if order.GetString("status") != "COMPLETE" || order.GetString("vpn_config") != "" {
			return nil
		}
	
		planId := order.GetString("plan")
		plan, err := app.Dao().FindFirstRecordByData("plans", "id", planId)
		if err != nil {
			log.Print("Error retrieving plan: ", err)
			return err
		}
		if plan == nil {
			log.Print("Error: Plan is nil")
			return fmt.Errorf("plan not found")
		}
	
		serverIds := plan.GetStringSlice("servers")
		if len(serverIds) == 0 {
			log.Print("Error: No servers associated with the plan")
			return fmt.Errorf("no servers associated with the plan")
		}
	
		enableStatusExpr := dbx.HashExp{"enable": true,}
	
		// Create a raw query with placeholders for each ID
		placeholders := make([]string, len(serverIds))
		params := dbx.Params{}
		for i, id := range serverIds {
			placeholders[i] = fmt.Sprintf("{:id%d}", i)
			params[fmt.Sprintf("id%d", i)] = id
		}
	
		// Join the placeholders into a single string
		idsExpr := dbx.NewExp(fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ", ")), params)
	
		// Add an ordering expression to sort by capacity in descending order
	
		// Combine all expressions and limit the result to 1
		query := app.Dao().RecordQuery("servers").
        AndWhere(idsExpr).
        AndWhere(enableStatusExpr).
        OrderBy("capacity DESC").
        Limit(1)
		servers := []*models.Record{}
		if err := query.All(&servers); err != nil {
			return err
		}

		if len(servers) == 0 {
			log.Print("Error: No servers found")
			return fmt.Errorf("no servers found")
		}
	
		// Handle the result
		server := servers[0]
		if server == nil {
			log.Print("Error: Server is nil")
			return fmt.Errorf("server not found")
		}
	
		log.Print("server connection: ", server.GetString("hostname"))
		if server.GetString("type") == "OUTLINE" {
			apiUrl := server.GetString("management_api_url")
			vpnConfigsCollection, err := app.Dao().FindCollectionByNameOrId("vpn_configs")
			if err !=nil {
				return nil
			}
			vpnConfig := models.NewRecord(vpnConfigsCollection)
			if err := app.Dao().SaveRecord(vpnConfig); err != nil {
				return err
			}
			accessKeyConfig, err := outlineApi.CreateAccessKey(apiUrl, vpnConfig.Id, int64(plan.GetInt("usage_limit_gb")))
			if err != nil {
				return nil
			}
			// create new vpn config 
			

			startDate := time.Now()
			endDate:= helpers.AddDays(plan.GetInt("date_limit"), startDate)

			jsonAccessKeyConfig, err := json.Marshal(accessKeyConfig)
			if err != nil {
				return nil
			}
			vpnConfig.Set("plan", planId)
			vpnConfig.Set("user", order.GetString("user"))
			vpnConfig.Set("start_date", startDate)
			vpnConfig.Set("end_date", endDate)
			vpnConfig.Set("type", "OUTLINE")
			vpnConfig.Set("usage_in_gb", plan.GetInt("usage_limit_gb"))
			vpnConfig.Set("server", server.Id)
			vpnConfig.Set("connection_data", string(jsonAccessKeyConfig))
			if err := app.Dao().SaveRecord(vpnConfig); err != nil {
				return err
			}		
			order.Set("vpn_config", vpnConfig.GetId())
			if err := app.Dao().SaveRecord(order); err != nil {
				return err
			}
			tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")

			user, err := app.Dao().FindRecordById("users", order.GetString("user"))
			if err != nil{
				return err
			}
			_, err = tgbot.SendVpnConfig(tgbotWebhookServer, user.Username(), order.Id)
			if err != nil {
				return err
			}
		}
	
		return nil
	})
	
	app.OnModelBeforeDelete("vpn_configs").Add(func(e *core.ModelEvent) error {
		vpnConfig, ok := e.Model.(*models.Record)
		server, err := app.Dao().FindRecordById("servers", vpnConfig.GetString("server"))
		if err != nil{
			return fmt.Errorf("model casting error")
		}
		if !ok {
			log.Print("Error: Could not cast model to *models.Record")
			return fmt.Errorf("model casting error")
		}
		if vpnConfig.GetString("type") == "OUTLINE"{
			fmt.Println(vpnConfig.GetString("connection_data"))
			connectionDataStr := vpnConfig.GetString("connection_data")

			var connectionDataStruct outlineApi.AccessKey
			err := json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
			if err != nil {
				return fmt.Errorf("failed to unmarshal connection_data: %w", err)
			}
			fmt.Println(connectionDataStruct.ID)
			err=outlineApi.DeleteAccessKey(server.GetString("management_api_url"), connectionDataStruct.ID)
			if err != nil {
				return nil
			}
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
			fmt.Println("is_approved",orderId, payment)
			order, err := app.Dao().FindRecordById("orders", orderId)
			if err!= nil {
				return err
			}
			order.Set("status", "COMPLETE")
			payment.Set("status", "PAID")
			if err := app.Dao().SaveRecord(order); err != nil {
				return err
			}
			if err := app.Dao().SaveRecord(payment); err != nil {
				return err
			}
		}
		return nil
	})

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
	
}

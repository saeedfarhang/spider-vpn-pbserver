package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	env "spider-vpn/config"
	"spider-vpn/helpers/queries"
	outlineApi "spider-vpn/wrappers/outline/api"
	tgbot "spider-vpn/wrappers/tg-bot"
	webhooks "spider-vpn/wrappers/tg-bot"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
	"github.com/robfig/cron/v3"
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
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.GET("/*", apis.Static(os.DirFS("./pb_public"), false))

		registry := template.NewRegistry()

		e.Router.GET("/pricing/{name}", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")

			html, err := registry.LoadFiles(
				"views/layout.html",
				"views/pricing/index.html",
			).Render(map[string]any{
				"name": name,
			})

			if err != nil {
				// or redirect to a dedicated 404 HTML page
				return apis.NewNotFoundError("", err)
			}
			return e.HTML(http.StatusOK, html)
		})
		e.Router.GET("/ssconf/{conf_id}", func(e *core.RequestEvent) error {
			conf_id := e.Request.PathValue("conf_id")
			vpnConfig, err := app.FindRecordById("vpn_configs", conf_id)
			if err != nil {
				return err
			}
			connectionDataStr := vpnConfig.GetString("connection_data")

			var connectionDataStruct outlineApi.AccessKey
			err = json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
			if err != nil {
				return err
			}

			// Log the AccessUrl for debugging
			fmt.Printf("conf: %v\n", connectionDataStruct.AccessUrl)

			// Prepare CSV data
			csvData := [][]string{
				{connectionDataStruct.AccessUrl},
				// Add other fields if necessary
			}

			// Set headers for CSV download
			e.Response.Header().Set(echo.HeaderContentDisposition, "attachment; filename=config.csv")
			e.Response.Header().Set(echo.HeaderContentType, "text/csv")

			// Write CSV content to the response
			writer := csv.NewWriter(e.Response)
			err = writer.WriteAll(csvData)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate CSV")
			}
			writer.Flush()

			return e.Next()
		})
		scheduler := cron.New()

		scheduler.AddFunc("30 * * * *", func() {
			err := queries.SyncVpnConfigsRemainUsage(app)
			if err != nil {
				fmt.Printf("Failed: %v", err)
				return
			}
			queries.HandleConfigsExpiry(app)
			log.Printf("add function to cronjob. each 1min")
		})
		scheduler.AddFunc("*/1 * * * *", func() {
			serverStatuses, err := queries.CheckActiveServersHealth(app)
			if err != nil {
				fmt.Printf("Failed: %v", err)
				return
			}
			tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")
			tgAdminUsers, err := app.FindRecordsByFilter("users", "is_admin=true", "id", 100, 0)
			if err != nil {
				fmt.Printf("Failed: %v", err)
				return
			}
			webhooks.SendServersHealthToAdmins(tgbotWebhookServer, serverStatuses, tgAdminUsers)

			log.Printf("add function to cronjob. each 1min, %v", serverStatuses)
		})

		scheduler.Start()
		return e.Next()
	})

	app.OnRecordAfterCreateSuccess("orders").BindFunc(func(e *core.RecordEvent) error {
		orderId := e.Record.Id
		order := e.Record
		// Fetch the related plan's ID
		planId := order.GetString("plan")
		gatewayId := order.GetString("payment_gateway")
		gateway, err := app.FindRecordById("payment_gateway", gatewayId)
		if err != nil {
			return err
		}
		app.DB().NewQuery(`UPDATE orders SET status="INCOMPLETE" WHERE id={:id}`).
			Bind(dbx.Params{"id": e.Record.Id}).Execute()

		// Fetch related pricing records for the plan
		pricings := []*core.Record{}
		err = app.DB().NewQuery(`
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
			pricing, err := app.FindRecordById("pricing", pricing.Id)
			if err != nil {
				return err
			}

			payments, err := app.FindCollectionByNameOrId("payments")
			payment := core.NewRecord(payments)
			if err != nil {
				return err
			}

			payment.Set("user", order.GetString("user"))
			payment.Set("order", orderId)
			payment.Set("amount", pricing.GetFloat("price"))
			payment.Set("currency", pricing.GetString("currency"))
			if err := app.Save(payment); err != nil {
				return err
			}

			if gateway.GetString("type") == "FREE" {
				user, err := app.FindRecordById("users", order.GetString("user"))
				if err != nil {
					return err
				}
				if user.GetBool("first_test_done") {
					_, err = tgbot.SendVpnConfig(tgbotWebhookServer, user.GetString("username"), "Nil")
					if err != nil {
						return err
					}
					return fmt.Errorf("duplicate test account")
				}
				payment.Set("status", "PAID")
				if err := app.Save(payment); err != nil {
					return err
				}
				order.Set("status", "COMPLETE")
				if err := app.Save(order); err != nil {
					return err
				}
				user.Set("first_test_done", true)
				if err := app.Save(user); err != nil {
					return err
				}
			} else {
				payment.Set("status", "UNPAID")
				if err := app.Save(payment); err != nil {
					return err
				}
			}
		}
		return nil
	})

	app.OnRecordAfterCreateSuccess("order_approval").BindFunc(func(e *core.RecordEvent) error {
		order_approval := e.Record
		orderId := order_approval.GetString("order")
		order, err := app.FindRecordById("orders", orderId)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		order.Set("status", "WAIT_FOR_APPROVE")
		order.Set("order_approval", order_approval.Id)
		if err := app.Save(order); err != nil {
			return err
		}
		tgAdminUsers, err := app.FindRecordsByFilter("users", "is_admin=true", "id", 100, 0)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		webhooks.SendNewOrderApprovalToAdmins(tgbotWebhookServer, order_approval.Id, tgAdminUsers)
		return nil
	})

	app.OnRecordAfterUpdateSuccess("orders").BindFunc(func(e *core.RecordEvent) error {
		order := e.Record
		if order.GetString("status") != "COMPLETE" || order.GetString("vpn_config") != "" {
			return nil
		}

		planId := order.GetString("plan")
		plan, err := app.FindFirstRecordByData("plans", "id", planId)
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

		servers, err := queries.GetActiveServers(app, serverIds, true, 1)
		if err != nil {
			return err
		}

		if len(servers) == 0 {
			return fmt.Errorf("no servers found")
		}

		// Handle the result
		server := servers[0]
		queries.CreateOrUpdateVpnConfig(app, server, plan, order, nil)
		// if server == nil {
		// 	log.Print("Error: Server is nil")
		// 	return fmt.Errorf("server not found")
		// }

		// log.Print("server connection: ", server.GetString("hostname"))
		// if server.GetString("type") == "OUTLINE" {
		// 	apiUrl := server.GetString("management_api_url")
		// 	vpnConfigsCollection, err := app.FindCollectionByNameOrId("vpn_configs")
		// 	if err != nil {
		// 		return nil
		// 	}
		// 	vpnConfig := core.NewRecord(vpnConfigsCollection)
		// 	if err := app.Save(vpnConfig); err != nil {
		// 		return err
		// 	}
		// 	accessKeyConfig, err := outlineApi.CreateAccessKey(apiUrl, vpnConfig.Id, int64(plan.GetInt("usage_limit_gb")))
		// 	if err != nil {
		// 		return nil
		// 	}
		// 	serverNewCapacity := server.GetInt("capacity") - 1
		// 	server.Set("capacity", serverNewCapacity)
		// 	if err := app.Save(server); err != nil {
		// 		return err
		// 	}
		// 	planNewCapacity := plan.GetInt("capacity") - 1
		// 	plan.Set("capacity", planNewCapacity)
		// 	if err := app.Save(plan); err != nil {
		// 		return err
		// 	}
		// 	// create new vpn config

		// 	startDate := time.Now()
		// 	endDate := helpers.AddDays(plan.GetInt("date_limit"), startDate)
		// 	// this salt added as prefixing solution to make the connection look like a protocol that is allowed in network
		// 	// more info: https://www.reddit.com/r/outlinevpn/wiki/index/prefixing/
		// 	accessKeyConfig.AccessUrl = accessKeyConfig.AccessUrl + "&" + "%13%03%03%3F"
		// 	jsonAccessKeyConfig, err := json.Marshal(accessKeyConfig)
		// 	if err != nil {
		// 		return nil
		// 	}
		// 	vpnConfig.Set("plan", planId)
		// 	vpnConfig.Set("user", order.GetString("user"))
		// 	vpnConfig.Set("start_date", startDate)
		// 	vpnConfig.Set("end_date", endDate)
		// 	vpnConfig.Set("type", "OUTLINE")
		// 	vpnConfig.Set("usage_in_gb", plan.GetInt("usage_limit_gb"))
		// 	vpnConfig.Set("server", server.Id)
		// 	vpnConfig.Set("connection_data", string(jsonAccessKeyConfig))
		// 	if err := app.Save(vpnConfig); err != nil {
		// 		return err
		// 	}
		// 	order.Set("vpn_config", vpnConfig.GetId())
		// 	if err := app.Save(order); err != nil {
		// 		return err
		// 	}
		// 	tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")

		// 	user, err := app.FindRecordById("users", order.GetString("user"))
		// 	if err != nil {
		// 		return err
		// 	}
		// 	_, err = tgbot.SendVpnConfig(tgbotWebhookServer, user.Username(), order.Id)
		// 	if err != nil {
		// 		return err
		// 	}
		// }

		return nil
	})

	// app.OnModelAfterUpdate("vpn_configs").Add(func(e *core.RecordEvent) error {
	// 	vpnConfig, ok := e.Record
	// 	if !ok {
	// 		log.Print("Error:  not cast model to *core.Record")
	// 		return fmt.Errorf("model casting error")
	// 	}
	// 	server, err := app.FindRecordById("servers", vpnConfig.GetString("server"))
	// 	if err != nil {
	// 		return fmt.Errorf("model casting error")
	// 	}
	// 	fmt.Printf("hello from %v %v", server, vpnConfig)
	// 	plan, err := app.FindRecordById("plans", vpnConfig.GetString("plan"))
	// 	if err != nil {
	// 		return fmt.Errorf("model casting error")
	// 	}
	// 	orders, err := app.FindRecordsByExpr("orders", dbx.HashExp{"vpn_config": vpnConfig.GetId()})
	// 	if err != nil || orders[0] == nil {
	// 		return fmt.Errorf("model casting error")
	// 	}
	// 	queries.CreateOrUpdateVpnConfig(app, server, plan, orders[0], vpnConfig)

	// 	return nil
	// })

	// app.OnModelAfterUpdate("vpn_configs").Add(func(e *core.RecordEvent) error {
	// 	vpnConfig, ok := e.Record
	// 	if !ok {
	// 		fmt.Printf("model casting error")
	// 		return nil
	// 	}
	// 	server, err := app.FindRecordById("servers", vpnConfig.GetString("server"))
	// 	if err != nil {
	// 		fmt.Print("server model casting error")
	// 		return nil
	// 	}

	// 	if vpnConfig.GetString("type") == "OUTLINE" {
	// 		connectionDataStr := vpnConfig.GetString("connection_data")

	// 		var connectionDataStruct outlineApi.AccessKey
	// 		err := json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
	// 		if err != nil {
	// 			return fmt.Errorf("failed to unmarshal connection_data: %w", err)
	// 		}
	// 		managementApiUrl := server.GetString("management_api_url")
	// 		if connectionDataStruct.ID != "" && managementApiUrl != "" {
	// 			err = outlineApi.DeleteAccessKey(managementApiUrl, connectionDataStruct.ID)
	// 			if err != nil {
	// 				fmt.Printf("failed to delete access key: %w", err)
	// 			}

	// 			plan, err := app.FindRecordById("plans", vpnConfig.GetString("plan"))
	// 			if err != nil {
	// 				log.Println(err)
	// 			}
	// 			_, err = app.DB().NewQuery(`UPDATE plans SET capacity={:planNewCapacity} WHERE id={:planId}`).Bind(dbx.Params{"planNewCapacity": plan.GetInt("capacity") + 1, "planId": plan.GetId()}).Execute()
	// 			if err != nil {
	// 				return fmt.Errorf("failed to update plan capacity: %w", err)
	// 			}
	// 			_, err = app.DB().NewQuery(`UPDATE servers SET capacity={:planNewCapacity} WHERE id={:serverId}`).Bind(dbx.Params{"planNewCapacity": server.GetInt("capacity") + 1, "serverId": server.GetId()}).Execute()
	// 			if err != nil {
	// 				return fmt.Errorf("failed to update server capacity: %w", err)
	// 			}
	// 		}
	// 	}
	// 	return nil
	// })

	app.OnRecordAfterUpdateSuccess("order_approval").BindFunc(func(e *core.RecordEvent) error {
		order_approval := e.Record
		orderId := order_approval.GetString("order")
		payment, err := app.FindFirstRecordByData("payments", "order", orderId)
		if err != nil {
			return err
		}
		is_approved := order_approval.GetBool(("is_approved"))
		if is_approved {
			fmt.Println("is_approved", orderId, payment)
			order, err := app.FindRecordById("orders", orderId)
			if err != nil {
				return err
			}
			order.Set("status", "COMPLETE")
			payment.Set("status", "PAID")
			if err := app.Save(order); err != nil {
				return err
			}
			if err := app.Save(payment); err != nil {
				return err
			}
		}
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}

}

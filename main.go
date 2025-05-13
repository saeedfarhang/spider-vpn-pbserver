package main

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	env "spider-vpn/config"
	"spider-vpn/constants"
	"spider-vpn/helpers/queries"
	outlineApi "spider-vpn/wrappers/outline/api"
	tgbot "spider-vpn/wrappers/tg-bot"
	webhooks "spider-vpn/wrappers/tg-bot"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/template"
	"github.com/robfig/cron/v3"
)

func main() {
	app := pocketbase.New()

	// loosely check if it was executed using "go run"
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Dashboard
		// (the isGoRun check is to enable it only during development)
		Automigrate: isGoRun,
	})

	tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")
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
			order, err := app.FindRecordById("orders", conf_id)
			if err != nil {
				return err
			}
			vpnConfig, err := app.FindRecordById("vpn_configs", order.GetString("vpn_config"))
			if err != nil {
				return err
			}
			connectionDataStr := vpnConfig.GetString("connection_data")

			var connectionDataStruct outlineApi.AccessKey
			err = json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
			if err != nil {
				return err
			}

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

		e.Router.GET("/v2ray/{conf_id}", func(e *core.RequestEvent) error {
			conf_id := e.Request.PathValue("conf_id")
			order, err := app.FindRecordById("orders", conf_id)
			if err != nil {
				return err
			}
			vpnConfig, err := app.FindRecordById("vpn_configs", order.GetString("vpn_config"))
			if err != nil {
				return err
			}
			// Check if it's Outline VPN
			if vpnConfig.GetString("type") != "OUTLINE" {
				return echo.NewHTTPError(http.StatusBadRequest, "This VPN is not an Outline VPN")
			}

			connectionDataStr := vpnConfig.GetString("connection_data")

			var connectionDataStruct outlineApi.AccessKey
			err = json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
			if err != nil {
				return err
			}

			v2rayConfig, err := outlineApi.ParseOutlineSS(connectionDataStruct.AccessUrl)
			if err != nil {
				return err
			}

			// Encode JSON to Base64
			jsonData, _ := json.Marshal(v2rayConfig)
			encodedData := base64.StdEncoding.EncodeToString(jsonData)

			html, err := registry.LoadFiles(
				"views/v2rayConf.html",
			).Render(map[string]any{
				"conf": "data:text/plain;base64," + encodedData,
			})

			if err != nil {
				// or redirect to a dedicated 404 HTML page
				return apis.NewNotFoundError("", err)
			}
			return e.HTML(http.StatusOK, html)

		})
		scheduler := cron.New()

		scheduler.AddFunc("30 * * * *", func() {
			err := queries.SyncVpnConfigsRemainUsage(app)
			if err != nil {
				fmt.Println("Failed: ", err)
				return
			}
			queries.HandleConfigsExpiry(app)
			log.Println("add SyncVpnConfigsRemainUsage() to cronjob. each 30 min")
		})

		scheduler.AddFunc("*/1 * * * *", func() {
			serverStatuses, err := queries.CheckActiveServersHealth(app)
			if err != nil {
				fmt.Println("CheckActiveServersHealth Failed:", err)
				return
			}
			fmt.Printf("Health Check Result: %v", serverStatuses)
			tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")
			tgAdminUsers, err := app.FindRecordsByFilter("users", "is_admin=true", "id", 100, 0)
			if err != nil {
				fmt.Print("FindRecordsByFilter Failed: ", err)
				return
			}
			webhooks.SendServersHealthToAdmins(tgbotWebhookServer, serverStatuses, tgAdminUsers)
			log.Println("add SendServersHealthToAdmins() to cronjob. each 1min")
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
		pricings := []constants.Pricing{} // Use a pointer slice
		err = app.DB().NewQuery(`
			SELECT pricing.*
			FROM pricing
			JOIN plans_pricing ON plans_pricing.pricing = pricing.id 
			WHERE plans_pricing.plan = {:planId} AND plans_pricing.gateway = {:gatewayId}
		`).Bind(dbx.Params{"planId": planId, "gatewayId": gatewayId}).All(&pricings)
		if err != nil {
			fmt.Println("Database query error:", err)
			return err
		}

		if len(pricings) == 0 {
			fmt.Println("No pricing records found")
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
				if user.GetBool("first_test_done") && !user.GetBool("is_admin") {
					_, err = tgbot.SendVpnConfig(tgbotWebhookServer, user.GetString("tg_id"), "Nil")
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
	app.OnRecordAfterUpdateSuccess("vpn_configs").BindFunc(func(e *core.RecordEvent) error {
		vpnConfig := e.Record
		if vpnConfig.GetString("server") != "" {
			return e.Next()
		}
		order, err := app.FindFirstRecordByData("orders", "vpn_config", vpnConfig.Id)
		if err != nil {
			return fmt.Errorf("no order found for this vpn config %s", vpnConfig.Id)
		}
		if order.GetString("status") != "COMPLETE" {
			return e.Next()
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
		queries.CreateOrUpdateVpnConfig(app, server, plan, order, vpnConfig)
		return e.Next()
	})
	app.OnRecordAfterDeleteSuccess("vpn_configs").BindFunc(func(e *core.RecordEvent) error {
		vpnConfig := e.Record
		fmt.Printf("vpn config %s", vpnConfig.GetString("connection_data"))
		connectionDataStr := vpnConfig.GetString("connection_data")
		var connectionDataStruct outlineApi.AccessKey
		err := json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
		if err != nil {
			fmt.Println("failed to unmarshal connection_data: ", connectionDataStr)
			return fmt.Errorf("failed to unmarshal connection_data: %w", err)
		}

		server, err := app.FindRecordById("servers", vpnConfig.GetString("server"))
		if err != nil {
			return fmt.Errorf("failed to get server: %w", err)
		}
		if vpnConfig.GetString("type") == "OUTLINE" {
			apiUrl := server.GetString("management_api_url")
			err = outlineApi.DeleteAccessKey(apiUrl, connectionDataStruct.ID)
			if err != nil {
				return fmt.Errorf("failed to delete config from outline server: %w", err)
			}
		}
		return e.Next()
	})
	app.OnRecordAfterCreateSuccess("order_approval").BindFunc(func(e *core.RecordEvent) error {
		orderApproval := e.Record
		orderId := orderApproval.GetString("order")
		order, err := app.FindRecordById("orders", orderId)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		order.Set("status", "WAIT_FOR_APPROVE")
		order.Set("order_approval", orderApproval.Id)
		if err := app.Save(order); err != nil {
			return err
		}
		tgAdminUsers, err := app.FindRecordsByFilter("users", "is_admin=true", "id", 100, 0)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		webhooks.SendNewOrderApprovalToAdmins(tgbotWebhookServer, orderApproval.Id, tgAdminUsers)
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
		return nil
	})

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

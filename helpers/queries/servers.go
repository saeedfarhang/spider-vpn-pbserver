package queries

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	env "spider-vpn/config"
	"spider-vpn/constants"
	"spider-vpn/helpers"
	outlineApi "spider-vpn/wrappers/outline/api"
	tgbot "spider-vpn/wrappers/tg-bot"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func GetActiveServers(app *pocketbase.PocketBase, serverIds []string, hasCapacity bool, limit int) (servers []*core.Record, err error) {
	enableStatusExpr := dbx.HashExp{"enable": true}
	var idsExpr dbx.Expression
	var query *dbx.SelectQuery
	// Create a raw query with placeholders for each ID
	if len(serverIds) != 0 {
		placeholders := make([]string, len(serverIds))
		params := dbx.Params{}
		for i, id := range serverIds {
			placeholders[i] = fmt.Sprintf("{:id%d}", i)
			params[fmt.Sprintf("id%d", i)] = id
		}
		idsExpr = dbx.NewExp(fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ", ")), params)
		if hasCapacity {
			existsCapacityExpr := dbx.Not(dbx.HashExp{"capacity": 0})
			query = app.RecordQuery("servers").
				AndWhere(idsExpr).
				AndWhere(existsCapacityExpr).
				AndWhere(enableStatusExpr).
				OrderBy("capacity DESC").
				Limit(100)
		} else {
			query = app.RecordQuery("servers").
				AndWhere(idsExpr).
				AndWhere(enableStatusExpr).
				OrderBy("capacity DESC").
				Limit(100)
		}
	} else {
		query = app.RecordQuery("servers").
			AndWhere(enableStatusExpr).
			OrderBy("capacity DESC").
			Limit(100)
	}

	if err := query.All(&servers); err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("error: no servers found")
	}

	// Handle the result
	server := servers[0]
	if server == nil {
		return nil, fmt.Errorf("error: server is nil")
	}
	return servers, nil
}

func CreateOrUpdateVpnConfig(app *pocketbase.PocketBase, server *core.Record, plan *core.Record, order *core.Record, updatedVpnConfig *core.Record) (err error) {
	if server == nil {
		log.Print("Error: Server is nil")
		return fmt.Errorf("server not found")
	}

	log.Print("server connection: ", server.GetString("hostname"))
	if server.GetString("type") == "OUTLINE" {
		apiUrl := server.GetString("management_api_url")

		var vpnConfig *core.Record
		var startDate time.Time
		var endDate time.Time
		var remainDataGb int
		if updatedVpnConfig == nil {
			vpnConfigsCollection, err := app.FindCollectionByNameOrId("vpn_configs")

			if err != nil {
				return fmt.Errorf("error %s", err.Error())
			}
			vpnConfig = core.NewRecord(vpnConfigsCollection)
			vpnConfig.Set("plan", plan.Id)
			vpnConfig.Set("user", order.GetString("user"))
			if err := app.Save(vpnConfig); err != nil {
				return fmt.Errorf("error %s", err.Error())
			}
			startDate = time.Now()
			endDate = helpers.AddDays(plan.GetInt("date_limit"), startDate)
			remainDataGb = plan.GetInt("usage_limit_gb")
		} else {
			vpnConfig = updatedVpnConfig
			startDate = vpnConfig.GetDateTime("start_date").Time()
			endDate = vpnConfig.GetDateTime("end_date").Time()
			remainDataGb = int(math.Ceil(float64(vpnConfig.GetInt("remain_data_mb")) / 1000)) // convert MB to GB
			if remainDataGb == 0 {
				remainDataGb = plan.GetInt("usage_limit_gb")
			}
		}
		accessKeyConfig, err := outlineApi.CreateAccessKey(apiUrl, vpnConfig.Id, int64(remainDataGb))

		if err != nil {
			return err
		}
		serverNewCapacity := server.GetInt("capacity") - 1
		server.Set("capacity", serverNewCapacity)
		if err := app.Save(server); err != nil {
			return err
		}
		planNewCapacity := plan.GetInt("capacity") - 1
		plan.Set("capacity", planNewCapacity)
		if err := app.Save(plan); err != nil {
			return err
		}
		// create new vpn config
		// this salt added as prefixing solution to make the connection look like a protocol that is allowed in network
		// more info: https://www.reddit.com/r/outlinevpn/wiki/index/prefixing/
		accessKeyConfig.AccessUrl = accessKeyConfig.AccessUrl + "&" + "%13%03%03%3F"
		jsonAccessKeyConfig, err := json.Marshal(accessKeyConfig)
		if err != nil {
			return err
		}
		vpnConfig.Set("start_date", startDate)
		vpnConfig.Set("end_date", endDate)
		vpnConfig.Set("type", "OUTLINE")
		vpnConfig.Set("usage_in_gb", remainDataGb)
		vpnConfig.Set("remain_data_mb", remainDataGb*1000)
		vpnConfig.Set("server", server.Id)
		vpnConfig.Set("connection_data", string(jsonAccessKeyConfig))
		if err := app.Save(vpnConfig); err != nil {
			log.Fatalln(err.Error())
			return err
		}
		order.Set("vpn_config", vpnConfig.Id)
		if err := app.Save(order); err != nil {
			return err
		}
		tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")

		user, err := app.FindRecordById("users", order.GetString("user"))
		if err != nil {
			return err
		}
		_, err = tgbot.SendVpnConfig(tgbotWebhookServer, user.GetString("tg_id"), order.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckActiveServersHealth(app *pocketbase.PocketBase) (serverStatuses []constants.ServerHealthStatus, err error) {
	servers, err := GetActiveServers(app, nil, false, 100)
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		apiUrl := server.GetString("management_api_url")
		healthy, err := outlineApi.CheckServerHealth(apiUrl)
		if err != nil {
			serverStatuses = append(serverStatuses, constants.ServerHealthStatus{
				ServerId:     server.Id,
				ErrorMessage: err.Error(),
				IsHealthy:    false,
			})
		} else {
			serverStatuses = append(serverStatuses, constants.ServerHealthStatus{
				ServerId:     server.Id,
				ErrorMessage: "",
				IsHealthy:    healthy,
			})
		}
	}
	return serverStatuses, nil
}

// this function has a cronjob
func SyncVpnConfigsRemainUsage(app *pocketbase.PocketBase) (err error) {
	servers, err := GetActiveServers(app, nil, false, 100)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}
	for _, server := range servers {
		apiUrl := server.GetString("management_api_url")
		// `usages` is a variable that stores the access key usages retrieved from the management API of a
		// server. It is used to track the data usage of each access key associated with VPN configurations.
		// The `GetAccessKeysUsages` function from the `outlineApi` package is called to fetch this
		// information for a specific server. The data usage information is then used to update the remaining
		// data allowance for each VPN configuration in the system.
		usages, err := outlineApi.GetAccessKeysUsages(apiUrl)
		if err != nil {
			log.Printf("Failed to get usages: %v", err)
			continue
		}
		vpnConfigs := []*core.Record{}
		err = app.RecordQuery("vpn_configs").
			AndWhere(dbx.Not(dbx.HashExp{"connection_data": ""})).
			// last update passed interval
			AndWhere(dbx.NewExp("updated < {:lastUpdate}", dbx.Params{"lastUpdate": time.Now().Add(time.Hour * 1)})).
			OrderBy("updated").
			Limit(100).
			All(&vpnConfigs)
		if err != nil {
			log.Fatalf("Failed to find vpn configs: %v", err)
		}
		for _, vpnConfig := range vpnConfigs {
			connectionDataStr := vpnConfig.GetString("connection_data")

			var connectionDataStruct outlineApi.AccessKey
			err := json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
			if err != nil {
				fmt.Println("failed to unmarshal connection_data: ", connectionDataStr)
				return fmt.Errorf("failed to unmarshal connection_data: %w", err)
			}
			usageInByte, usageInByteExists := usages.BytesTransferredByUserID[connectionDataStruct.ID]
			if usageInByteExists {
				vpnConfig.Set("remain_data_mb", (vpnConfig.GetInt("usage_in_gb")*1000)-int(usageInByte/1e+6))
			} else {
				vpnConfig.Set("remain_data_mb", vpnConfig.GetInt("usage_in_gb")*1000)
			}
			if err := app.Save(vpnConfig); err != nil {
				return err
			}
		}
	}
	return nil
}

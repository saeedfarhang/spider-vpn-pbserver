package queries

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"spider-vpn/constants"
	outlineApi "spider-vpn/wrappers/outline/api"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

func GetActiveServers(app *pocketbase.PocketBase, serverIds []string, hasCapacity bool, limit int) (servers []*models.Record, err error) {
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
			query = app.Dao().RecordQuery("servers").
				AndWhere(idsExpr).
				AndWhere(existsCapacityExpr).
				AndWhere(enableStatusExpr).
				OrderBy("capacity DESC").
				Limit(100)
		} else {
			query = app.Dao().RecordQuery("servers").
				AndWhere(idsExpr).
				AndWhere(enableStatusExpr).
				OrderBy("capacity DESC").
				Limit(100)
		}
	} else {
		query = app.Dao().RecordQuery("servers").
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

func CheckActiveServersHealth(app *pocketbase.PocketBase) (serverStatuses []constants.ServerHealthStatus, err error) {
	servers, err := GetActiveServers(app, nil, false, 100)
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		fmt.Printf("server: %v", server)
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
		vpnConfigs := []*models.Record{}
		err = app.Dao().RecordQuery("vpn_configs").
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
			if err := app.Dao().Save(vpnConfig); err != nil {
				return err
			}
		}
	}
	return nil
}

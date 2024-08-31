package queries

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	outlineApi "spider-vpn/wrappers/outline/api"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

func GetActiveServers (app *pocketbase.PocketBase, serverIds []string)(servers []*models.Record, err error){
	enableStatusExpr := dbx.HashExp{"enable": true,}
	var idsExpr dbx.Expression
	var query *dbx.SelectQuery
	// Create a raw query with placeholders for each ID
	if (len(serverIds) != 0){
		placeholders := make([]string, len(serverIds))
		params := dbx.Params{}
		for i, id := range serverIds {
			placeholders[i] = fmt.Sprintf("{:id%d}", i)
			params[fmt.Sprintf("id%d", i)] = id
		}
		idsExpr = dbx.NewExp(fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ", ")), params)
		query = app.Dao().RecordQuery("servers").
		AndWhere(idsExpr).
		AndWhere(enableStatusExpr).
		OrderBy("capacity DESC").
		Limit(1)
	}else {
		query = app.Dao().RecordQuery("servers").
		AndWhere(enableStatusExpr).
		OrderBy("capacity DESC").
		Limit(1)
	}

	if err := query.All(&servers); err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		log.Print("Error: No servers found")
		return nil, fmt.Errorf("no servers found")
	}

	// Handle the result
	server := servers[0]
	if server == nil {
		log.Print("Error: Server is nil")
		return nil, fmt.Errorf("server not found")
	}
	return servers, nil
}


func SyncVpnConfigsRemainUsage(app *pocketbase.PocketBase)(err error){
	servers, err := GetActiveServers(app, nil)
	if err != nil{
		log.Fatalf("Failed to execute query: %v", err)
	}

	for _, server := range(servers){
		apiUrl:= server.GetString("management_api_url")
		usages , err := outlineApi.GetAccessKeysUsages(apiUrl)
		if err != nil{
			log.Fatalf("Failed to get usages: %v", err)
		}
		vpnConfigs := []*models.Record{}
		err = app.Dao().RecordQuery("vpn_configs").
						AndWhere(dbx.Not(dbx.HashExp{"connection_data": ""})).
						// last update passed interval
						AndWhere(dbx.NewExp("updated < {:lastUpdate}", dbx.Params{"lastUpdate": time.Now().Add(time.Hour * 1)})).
						OrderBy("updated").
						Limit(100).
						All(&vpnConfigs)
		if err != nil{
			log.Fatalf("Failed to find vpn configs: %v", err)
		}
		log.Println("vpn configs to update remain_usage: ", vpnConfigs)
		for _, vpnConfig := range(vpnConfigs){
			connectionDataStr := vpnConfig.GetString("connection_data")

			var connectionDataStruct outlineApi.AccessKey
			err := json.Unmarshal([]byte(connectionDataStr), &connectionDataStruct)
			if err != nil {
				fmt.Println("failed to unmarshal connection_data: ", connectionDataStr)
				return fmt.Errorf("failed to unmarshal connection_data: %w", err)
			}
			usageInByte, usageInByteExists := usages.BytesTransferredByUserID[connectionDataStruct.ID]
			if usageInByteExists {
				vpnConfig.Set("remain_data_mb", (vpnConfig.GetInt("usage_in_gb") * 1000) - int(usageInByte / 1e+6))
				}else{
				vpnConfig.Set("remain_data_mb", vpnConfig.GetInt("usage_in_gb") * 1000)
			}
			if err := app.Dao().Save(vpnConfig); err != nil {
				return err
			}
		}
		}
	return nil
}
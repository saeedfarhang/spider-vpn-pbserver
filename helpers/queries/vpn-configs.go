package queries

import (
	"fmt"
	"log"
	env "spider-vpn/config"
	webhooks "spider-vpn/wrappers/tg-bot"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
)

func HandleConfigsExpiry(app *pocketbase.PocketBase)(err error){
	expiredOrders := []*models.Record{}
	err = app.Dao().RecordQuery("orders").
	InnerJoin("vpn_configs", dbx.NewExp("orders.vpn_config = vpn_configs.id")).
	AndWhere(dbx.NewExp("vpn_configs.end_date < {:nowDate}", dbx.Params{"nowDate": time.Now().Add(-time.Hour * 24)})). // condition for current time
	OrWhere(dbx.NewExp("vpn_configs.remain_data_mb < 1000")).
	All(&expiredOrders)
	fmt.Println("expiredOrders", expiredOrders)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}
	tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")
	fmt.Println(tgbotWebhookServer)

	for _, expiredOrder := range expiredOrders {
		vpnConfig, err := app.Dao().FindRecordById("vpn_configs",expiredOrder.GetString("vpn_config"))
		if err != nil{
			return fmt.Errorf("error: %v",err)
		}
		user, err := app.Dao().FindRecordById("users",expiredOrder.GetString("user"))
		if err != nil{
			return fmt.Errorf("error: %v",err)
		}
		if err != nil{
			return fmt.Errorf("error: %v",err)
		}
		if time.Since(vpnConfig.GetTime("end_date")) < 0 || vpnConfig.GetInt("remain_data_mb") == 0{
			if err := app.Dao().DeleteRecord(expiredOrder); err != nil {
				return fmt.Errorf("error: %v",err)
			} else if err := app.Dao().DeleteRecord(vpnConfig); err != nil {
				return fmt.Errorf("error: %v",err)
			}else{
				log.Print("Successfully Remove Expired Orders")
			}
			webhooks.SendDeleteDeprecatedVpnConfigNotification(tgbotWebhookServer,user.Username() , expiredOrder.Id)
			return nil
		}
		webhooks.SendExpiryVpnConfigNotification(tgbotWebhookServer,user.Username() , expiredOrder.Id, time.Until(vpnConfig.GetDateTime("end_date").Time()).Hours(), vpnConfig.GetInt("remain_data_mb"))
		
	}
	return nil

}

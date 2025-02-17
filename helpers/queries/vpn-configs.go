package queries

import (
	"fmt"
	"log"
	env "spider-vpn/config"
	webhooks "spider-vpn/wrappers/tg-bot"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func HandleConfigsExpiry(app *pocketbase.PocketBase) (err error) {
	nearExpiryOrders := []*core.Record{}
	expiredOrders := []*core.Record{}

	// `hoursBeforeEndDate` value use to get vpn configs that end_date field is bigger than now - hoursBeforeEndDate.
	hoursBeforeEndDate := 2
	// `remainDataMb` value use to get vpn configs that remain_data_mb field is smaller than remainDataMb.
	remainDataMb := 100
	err = app.RecordQuery("orders").
		InnerJoin("vpn_configs", dbx.NewExp("orders.vpn_config = vpn_configs.id")).
		AndWhere(dbx.NewExp("vpn_configs.end_date > {:nowDate} AND vpn_configs.end_date < {:startNoticeDuration}",
			dbx.Params{"startNoticeDuration": time.Now().UTC().Add(time.Hour * time.Duration(hoursBeforeEndDate)), "nowDate": time.Now().UTC()})). // condition for current time
		OrWhere(dbx.NewExp("vpn_configs.remain_data_mb < {:remainDataMb}", dbx.Params{"remainDataMb": remainDataMb})).
		All(&nearExpiryOrders)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}
	err = app.RecordQuery("orders").
		InnerJoin("vpn_configs", dbx.NewExp("orders.vpn_config = vpn_configs.id")).
		AndWhere(dbx.NewExp("vpn_configs.end_date < {:nowDate}",
			dbx.Params{"startNoticeDuration": time.Now().UTC().Add(time.Hour * time.Duration(hoursBeforeEndDate)), "nowDate": time.Now().UTC()})). // condition for current time
		OrWhere(dbx.NewExp("vpn_configs.remain_data_mb < {:expiredConfigMb}", dbx.Params{"expiredConfigMb": 1})).
		All(&expiredOrders)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}
	tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")

	fmt.Println("nearExpiryOrders: ", nearExpiryOrders)
	for _, nearExpiryOrder := range nearExpiryOrders {
		vpnConfig, err := app.FindRecordById("vpn_configs", nearExpiryOrder.GetString("vpn_config"))
		if err != nil {
			return fmt.Errorf("error: %v", err)
		}
		user, err := app.FindRecordById("users", nearExpiryOrder.GetString("user"))
		if err != nil {
			return fmt.Errorf("error: %v", err)
		}
		webhooks.SendExpiryVpnConfigNotification(tgbotWebhookServer, user.GetString("tg_id"), nearExpiryOrder.Id, time.Until(vpnConfig.GetDateTime("end_date").Time()).Hours(), vpnConfig.GetInt("remain_data_mb"))
	}
	fmt.Println("expiredOrders: ", expiredOrders)
	for _, expiredOrder := range expiredOrders {
		vpnConfig, err := app.FindRecordById("vpn_configs", expiredOrder.GetString("vpn_config"))
		if err != nil {
			return fmt.Errorf("error: %v", err)
		}
		user, err := app.FindRecordById("users", expiredOrder.GetString("user"))
		if err != nil {
			return fmt.Errorf("error: %v", err)
		}
		if time.Since(vpnConfig.GetDateTime("end_date").Time()).Seconds() > 0 || vpnConfig.GetInt("remain_data_mb") == 0 {
			if err := app.Delete(expiredOrder); err != nil {
				return fmt.Errorf("error: %v", err)
			} else if err := app.Delete(vpnConfig); err != nil {
				return fmt.Errorf("error: %v", err)
			} else {
				log.Print("Successfully Remove Expired Orders")
			}
			webhooks.SendDeleteDeprecatedVpnConfigNotification(tgbotWebhookServer, user.GetString("tg_id"), expiredOrder.Id)
			return nil
		}
	}
	return nil

}

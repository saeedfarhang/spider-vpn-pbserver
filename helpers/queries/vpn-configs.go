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
	now := time.Now().UTC()
	startNoticeDuration := now.Add(time.Hour * 4)
	remainDataMb := 500

	nearExpiryOrders := []*core.Record{}
	expiredOrders := []*core.Record{}

	// Query for near-expiry configs
	err = app.RecordQuery("orders").
		InnerJoin("vpn_configs", dbx.NewExp("orders.vpn_config = vpn_configs.id")).
		WhereGroup(func(g dbx.Group) {
			g.And(
				dbx.NewExp("vpn_configs.end_date > {:nowDate}", dbx.Params{"nowDate": now}),
				dbx.NewExp("vpn_configs.end_date < {:startNoticeDuration}", dbx.Params{"startNoticeDuration": startNoticeDuration}),
			)
			g.Or(
				dbx.NewExp("vpn_configs.remain_data_mb < {:remainDataMb}", dbx.Params{"remainDataMb": remainDataMb}),
			)
		}).
		All(&nearExpiryOrders)
	if err != nil {
		log.Fatalf("Failed to execute near-expiry query: %v", err)
	}

	// Query for expired configs
	err = app.RecordQuery("orders").
		InnerJoin("vpn_configs", dbx.NewExp("orders.vpn_config = vpn_configs.id")).
		WhereGroup(func(g dbx.Group) {
			g.And(
				dbx.NewExp("vpn_configs.end_date < {:nowDate}", dbx.Params{"nowDate": now}),
			)
			g.Or(
				dbx.NewExp("vpn_configs.remain_data_mb < {:expiredConfigMb}", dbx.Params{"expiredConfigMb": 1}),
			)
		}).
		All(&expiredOrders)
	if err != nil {
		log.Fatalf("Failed to execute expired query: %v", err)
	}

	tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")

	fmt.Println("nearExpiryOrders: ", nearExpiryOrders)
	for _, nearExpiryOrder := range nearExpiryOrders {
		vpnConfig, err := app.FindRecordById("vpn_configs", nearExpiryOrder.GetString("vpn_config"))
		if err != nil {
			return fmt.Errorf("error finding vpn_config: %v", err)
		}
		user, err := app.FindRecordById("users", nearExpiryOrder.GetString("user"))
		if err != nil {
			return fmt.Errorf("error finding user: %v", err)
		}
		webhooks.SendExpiryVpnConfigNotification(
			tgbotWebhookServer,
			user.GetString("tg_id"),
			nearExpiryOrder.Id,
			time.Until(vpnConfig.GetDateTime("end_date").Time()).Hours(),
			vpnConfig.GetInt("remain_data_mb"),
		)
	}

	fmt.Println("expiredOrders: ", expiredOrders)
	for _, expiredOrder := range expiredOrders {
		vpnConfig, err := app.FindRecordById("vpn_configs", expiredOrder.GetString("vpn_config"))
		if err != nil {
			return fmt.Errorf("error finding vpn_config: %v", err)
		}
		user, err := app.FindRecordById("users", expiredOrder.GetString("user"))
		if err != nil {
			return fmt.Errorf("error finding user: %v", err)
		}
		if time.Since(vpnConfig.GetDateTime("end_date").Time()).Seconds() > 0 || vpnConfig.GetInt("remain_data_mb") <= 0 {
			if err := app.Delete(expiredOrder); err != nil {
				return fmt.Errorf("error deleting order: %v", err)
			}
			if err := app.Delete(vpnConfig); err != nil {
				return fmt.Errorf("error deleting vpn_config: %v", err)
			}
			log.Print("Successfully Removed Expired Order")
			webhooks.SendDeleteDeprecatedVpnConfigNotification(tgbotWebhookServer, user.GetString("tg_id"), expiredOrder.Id)
		}
	}

	return nil
}

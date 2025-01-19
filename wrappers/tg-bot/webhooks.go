package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"spider-vpn/constants"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
)

func SendNewOrderApprovalToAdmins(tgbotWebhookServer string, orderApprovalId string, adminUsers []*core.Record) (any, error) {
	for _, adminUser := range adminUsers {
		_, err := http.Get(tgbotWebhookServer + "/trigger/send-new-order-approval-admin?order_approval_id=" + orderApprovalId + "&user_id=" + adminUser.GetString("tg_id"))
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func SendServersHealthToAdmins(tgbotWebhookServer string, ServerHealthStatuses []constants.ServerHealthStatus, adminUsers []*core.Record) (any, error) {
	var bodyReader *bytes.Reader
	jsonServerHealthStatuses, err := json.Marshal(ServerHealthStatuses)
	bodyReader = bytes.NewReader(jsonServerHealthStatuses)
	if err != nil {
		log.Fatal(err)
	}

	for _, adminUser := range adminUsers {
		_, err := http.Post(tgbotWebhookServer+"/trigger/send-server-health-admin?user_id="+adminUser.GetString("tg_id"), "application/json", bodyReader)
		fmt.Printf("here: %v, %v %v", bodyReader, adminUser, err)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func SendVpnConfig(tgbotWebhookServer string, tgUserId string, orderId string) (any, error) {
	_, err := http.Get(tgbotWebhookServer + "/trigger-vpn-config?user_id=" + tgUserId + "&order_id=" + orderId)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func SendExpiryVpnConfigNotification(tgbotWebhookServer string, tgUserId string, orderId string, hoursToExpire float64, remainInMb int) (any, error) {
	fmt.Println(tgbotWebhookServer)
	_, err := http.Get(tgbotWebhookServer + "/expiry-vpn-config?user_id=" + tgUserId + "&order_id=" + orderId + "&hours_to_expire=" + strconv.FormatFloat(hoursToExpire, 'f', 0, 32) + "&remain_in_mb=" + fmt.Sprint(remainInMb))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return nil, nil
}

func SendDeleteDeprecatedVpnConfigNotification(tgbotWebhookServer string, tgUserId string, orderId string) (any, error) {
	_, err := http.Get(tgbotWebhookServer + "/deprecated-vpn-config?user_id=" + tgUserId + "&order_id=" + orderId)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

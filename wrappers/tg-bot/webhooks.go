package webhooks

import (
	"net/http"
	"strconv"
)


func SendVpnConfig (tgbotWebhookServer string,tgUserId string,orderId string)(any, error){
	_, err := http.Get(tgbotWebhookServer+"/trigger-vpn-config?user_id="+tgUserId+"&order_id="+orderId)
	if err != nil{
		return nil, err
	}
	return nil, nil
}

func SendExpiryVpnConfigNotification (tgbotWebhookServer string,tgUserId string,orderId string, daysToExpire float64)(any, error){
	_, err := http.Get(tgbotWebhookServer+"/expiry-vpn-config?user_id="+tgUserId+"&order_id="+orderId+"&days_to_expire="+strconv.FormatFloat(daysToExpire, 'f', 0, 64))
	if err != nil{
		return nil, err
	}
	return nil, nil
}

func SendDeleteDeprecatedVpnConfigNotification (tgbotWebhookServer string,tgUserId string,orderId string)(any, error){
	_, err := http.Get(tgbotWebhookServer+"/deprecated-vpn-config?user_id="+tgUserId+"&order_id="+orderId)
	if err != nil{
		return nil, err
	}
	return nil, nil
}
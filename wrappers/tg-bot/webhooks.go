package webhooks

import (
	"net/http"
)


func SendVpnConfig (tgbotWebhookServer string,tgUserId string,orderId string)(any, error){
	_, err := http.Get(tgbotWebhookServer+"/trigger-vpn-config?user_id="+tgUserId+"&order_id="+orderId)
	if err != nil{
		return nil, err
	}
	return nil, nil
}
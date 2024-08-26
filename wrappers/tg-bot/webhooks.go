package webhooks

import (
	"fmt"
	"net/http"
)


func SendVpnConfig (tgbotWebhookServer string,tgUserId string,configId string)(any, error){
	fmt.Println(tgbotWebhookServer, tgUserId,configId, tgbotWebhookServer+"/submitconfig?user_id"+tgUserId+"&config_id="+configId)
	_, err := http.Get(tgbotWebhookServer+"/trigger-vpn-config?user_id="+tgUserId+"&config_id="+configId)
	if err != nil{
		return nil, err
	}
	return nil, nil
}
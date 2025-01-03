package constants

type ServerHealthStatus struct {
	ServerId     string `json:"serverId"`
	ErrorMessage string `json:"errorMessage"`
	IsHealthy    bool   `json:"isHealthy"`
}

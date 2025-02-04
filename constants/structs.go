package constants

type ServerHealthStatus struct {
	ServerId     string `json:"serverId"`
	ErrorMessage string `json:"errorMessage"`
	IsHealthy    bool   `json:"isHealthy"`
}

type PlansPricing struct {
	Id      string `db:"id" json:"id"`
	Plan    Plan
	Pricing Pricing
	Gateway string `db:"gateway" json:"gateway"`
}

type Pricing struct {
	Id       string  `db:"id" json:"id"`
	Price    float64 `db:"price" json:"price"`
	Currency string  `db:"currency" json:"currency"`
}

type Plan struct {
	Id           string  `db:"id" json:"id"`
	Name         string  `db:"name" json:"name"`
	DateLimit    int     `db:"date_limit" json:"date_limit"`
	UsageLimitGB int     `db:"usage_limit_gb" json:"usage_limit_gb"`
	Capacity     float64 `db:"capacity" json:"capacity"`
	CommonName   string  `db:"common_name" json:"common_name"`
	Active       bool    `db:"active" json:"active"`
	ShownPrice   float64 `db:"shown_price" json:"shown_price"`
	Servers      string  `db:"servers" json:"servers"`
	Type         string  `db:"type" json:"type"`
}

type Orders struct {
	Id             string `db:"id" json:"id"`
	User           string `db:"user" json:"user"`
	Plan           Plan
	MetaData       string `db:"meta_data" json:"meta_data"`
	Status         string `db:"status" json:"status"`
	PaymentGateway string `db:"payment_gateway" json:"payment_gateway"`
	VPNConfig      string `db:"vpn_config" json:"vpn_config"`
}

package api

type BR struct {
	Host   string `json:"host"`
	Master bool   `json:"master"`
}

type LND struct {
	PublicKey string `json:"publickey"`
	Host      string `json:"host"`
}

type LPD struct {
	Certificate string `json:"certificate"`
	Host        string `json:"host"`
}

type Status struct {
	LastUpdated int64 `json:"last_updated"`
	BRs         []BR  `json:"brs"`
	LNDs        []LND `json:"lnds"`
	LPDs        []LPD `json:"lpds"`
}

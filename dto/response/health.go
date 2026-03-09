package response

type HealthResp struct {
	Reply string `json:"reply"`
}

type ProbeResp struct {
	Status string `json:"status"`
	Store  string `json:"store,omitempty"`
}

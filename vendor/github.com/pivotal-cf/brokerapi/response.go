package brokerapi

type EmptyResponse struct{}

type ErrorResponse struct {
	Error       string `json:"error,omitempty"`
	Description string `json:"description"`
}

type CatalogResponse struct {
	Services []Service `json:"services"`
}

// here the Credentials part is to satisfy the request from critic
type ProvisioningResponse struct {
	DashboardURL string      `json:"dashboard_url,omitempty"`
	Credentials  interface{} `json:"credentials,omitempty"`
	//Credentials  map[string]string `json:"credentials,omitempty"`
}

type LastOperationResponse struct {
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
}

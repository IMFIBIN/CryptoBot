package httpapi

type PlanRequest struct {
	Base     string  `json:"base"`
	Quote    string  `json:"quote"`
	Amount   float64 `json:"amount"`
	Scenario string  `json:"scenario"`
}

type PlanLeg struct {
	Exchange string  `json:"exchange"`
	Amount   float64 `json:"amount"`
	Price    float64 `json:"price"`
}

type PlanResponse struct {
	Base        string    `json:"base"`
	Quote       string    `json:"quote"`
	Amount      float64   `json:"amount"`
	Scenario    string    `json:"scenario"`
	VWAP        float64   `json:"vwap"`
	TotalCost   float64   `json:"totalCost"`
	Unspent     float64   `json:"unspent"`
	Legs        []PlanLeg `json:"legs"`
	GeneratedAt string    `json:"generatedAt"`
}

type SymbolsResponse struct {
	Bases  []string `json:"bases"`
	Quotes []string `json:"quotes"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

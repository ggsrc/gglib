package mctx

// LocationInfo is a coarse geographic location.
type LocationInfo struct {
	Country  string `json:"country"`
	Province string `json:"province"`
	City     string `json:"city"`
}

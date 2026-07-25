package dto

// UpdateSettingsRequest uses pointers so a client can PATCH a single
// field (e.g. just dark_mode) without accidentally overwriting
// currency with a zero value — same pattern as
// UpdateTransactionRequest.
type UpdateSettingsRequest struct {
	DarkMode *bool   `json:"dark_mode" binding:"omitempty"`
	Currency *string `json:"currency" binding:"omitempty,len=3"`
}

type SettingsResponse struct {
	DarkMode bool   `json:"dark_mode"`
	Currency string `json:"currency"`
}

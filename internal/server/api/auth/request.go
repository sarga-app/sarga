package auth

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"` //nolint:gosec // OAuth-style API contract
	ExpiresIn   int    `json:"expires_in"`
}

type MeResponse struct {
	UserID  string `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
}

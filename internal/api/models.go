package api

type PaginatedResponse[T any] struct {
	Page       int `json:"currentPage"`
	TotalPages int `json:"totalPages"`
	Data       []T
}

type TokenResponse struct {
	Token string `json:"token"`
}

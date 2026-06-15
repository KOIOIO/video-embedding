package dto

type SuccessResponse[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   ErrorBody `json:"error"`
}

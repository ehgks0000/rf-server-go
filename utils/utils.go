package utils

// ResponseModel is a structure that standardizes the HTTP response format.
type ResponseModel struct {
	Code    int         `json:"code,omitempty"`    // HTTP status code
	Message string      `json:"message,omitempty"` // Response message or error description
	Data    interface{} `json:"data,omitempty"`    // Any data payload returned in the response
}

// NewResponseModel is a constructor for creating a new ResponseModel.
func NewResponseModel(code int, message string, data interface{}) *ResponseModel {
	return &ResponseModel{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

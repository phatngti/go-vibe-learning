package response

func Wrap(messageMap map[string]string, code string, jsonData any) Response {
	return Response{
		Code:    code,
		Message: messageMap[code],
		Data:    jsonData,
	}
}

func WrapError(messageMap map[string]string, code string, subErrors *[]ErrorDetailsInfo) ErrorResponse {
	buildSubErrors := []ErrorDetails{}
	if subErrors == nil {
		return ErrorResponse{
			Code:    code,
			Message: messageMap[code],
			Errors:  buildSubErrors,
		}
	}
	for _, subError := range *subErrors {
		buildSubErrors = append(buildSubErrors, ErrorDetails{
			Code:    subError.Code,
			Message: messageMap[subError.Code],
			Data:    subError.Data,
		})
	}
	return ErrorResponse{
		Code:    code,
		Message: messageMap[code],
		Errors:  buildSubErrors,
	}
}

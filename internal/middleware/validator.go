package middleware

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github/master_go_cqrs/internal/httpapi/response"
	models "github/master_go_cqrs/internal/httpapi/response"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type CustomValidator struct {
	Validator *validator.Validate
}

type Errors struct {
	Field   string
	Message string
	Code    []string
	Tag     string
	Value   any
}

func (e *Errors) Error() string {
	return e.Message
}

func (e *Errors) GetCode() []string {
	return e.Code
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.Validator.Struct(i); err != nil {

		var validationErrors Errors
		if errs, ok := err.(validator.ValidationErrors); ok {
			val := reflect.ValueOf(i)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			typ := val.Type()
			for _, err := range errs {
				field, _ := typ.FieldByName(err.StructField())
				message := field.Tag.Get("message")

				if message == "" {
					msg := fmt.Sprintf("Invalid value for field %s with tag %s", err.Field(), err.Tag())
					validationErrors = Errors{
						Field:   err.Field(),
						Message: msg,
						Code:    []string{},
						Tag:     err.Tag(),
						Value:   val.FieldByName(err.StructField()).Interface(),
					}
				} else {
					codes := strings.Split(message, ",")
					codeStrings := []string{}
					codeStrings = append(codeStrings, codes...)
					validationErrors = Errors{
						Field:   err.Field(),
						Message: message,
						Code:    codeStrings,
						Tag:     err.Tag(),
						Value:   val.FieldByName(err.StructField()).Interface(),
					}
				}
			}

		}
		return echo.NewHTTPError(http.StatusBadRequest, validationErrors)
	}
	return nil
}

func ValidateHandler(model interface{}, messageMap map[string]string) echo.MiddlewareFunc {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			newModel := reflect.New(modelType).Interface()

			if err := c.Bind(newModel); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, err)
			}

			if err := c.Validate(newModel); err != nil {
				httpErr := err.(*echo.HTTPError)
				validationErrors := httpErr.Message.(Errors)
				codes := validationErrors.GetCode()
				var subErrors []models.ErrorDetailsInfo = make([]models.ErrorDetailsInfo, len(codes))
				for i, code := range codes {
					subErrors[i] = models.ErrorDetailsInfo{
						Code: code,
					}
				}

				messageMap["validation_failed"] = "Validation failed"
				buildError := response.WrapError(messageMap, "validation_failed", &subErrors)
				return echo.NewHTTPError(http.StatusBadRequest, buildError)
			}

			c.Set("dto", newModel)
			return next(c)
		}
	}
}

package utils

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
)

func CheckUrl(value interface{}) error {
	return validation.Validate(value, is.URL)
}

func CheckEmail(value interface{}) error {
	return validation.Validate(value, is.Email)
}

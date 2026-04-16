package app

import (
	"github.com/go-playground/validator/v10"
	"mirea-qr/internal/config"
	"strings"
)

func NewValidator(cfg config.Config) *validator.Validate {
	validate := validator.New()

	err := validate.RegisterValidation("mireaDomain", mireaDomain)
	if err != nil {
		return nil
	}

	return validate
}

func mireaDomain(fl validator.FieldLevel) bool {
	email := fl.Field().String()

	return strings.Contains(email, "@edu.mirea.ru")
}

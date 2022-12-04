package validate

import (
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
)

// Struct is ...
func Struct(input any) *map[string]string {
	validate := validator.New()
	en := en.New()
	uni := ut.New(en, en)

	translator, _ := uni.GetTranslator("en")
	enTranslations.RegisterDefaultTranslations(validate, translator)

	if err := validate.Struct(input); err != nil {
		return buildTranslatedErrorMessages(err.(validator.ValidationErrors), translator)
	}
	return nil
}

func buildTranslatedErrorMessages(err validator.ValidationErrors, translator ut.Translator) *map[string]string {
	errors := make(map[string]string)
	for _, err := range err {
		errors[strings.ToLower(err.Field())] = err.Translate(translator)
	}
	return &errors
}

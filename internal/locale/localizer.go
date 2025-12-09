package locale

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localizedata embed.FS

const (
	Ru = "ru"
	En = "en"
)

type locale struct {
	locale string
}

type Locale interface {
	GetLocale() string
}

func NewLocale(l string) Locale {
	return &locale{
		locale: l,
	}
}

func (l *locale) GetLocale() string {
	return l.locale
}

type localizer struct {
	Locale
	*i18n.Localizer
}

type Localizer interface {
	Locale
	MustLocalize(id string) string
	MustLocalizeWithTemplate(id string, fields ...string) string
}

func NewLocalizer(ctx context.Context, locale Locale) (Localizer, error) {
	// Skip validation in production - it requires source files which aren't available in compiled binaries
	// Validation should be run during development/testing via tests
	// _, err := ValidateTranslations()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to validate translations: %w", err)
	// }

	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	files := []string{
		"en.json",
		"ru.json",
	}

	for _, f := range files {
		data, err := localizedata.ReadFile(fmt.Sprintf("locales/%s", f))
		if err != nil {
			return nil, fmt.Errorf("failed to load translation data: %s", f)
		}

		bundle.MustParseMessageFileBytes(data, f)
	}

	opts := defaultOptions(bundle)

	return &localizer{
		locale,
		i18n.NewLocalizer(opts.bundle, locale.GetLocale()),
	}, nil
}

type options struct {
	bundle *i18n.Bundle
}

func defaultOptions(bundle *i18n.Bundle) options {
	return options{
		bundle: bundle,
	}
}

func (l *localizer) MustLocalize(id string) string {
	return l.Localizer.MustLocalize(createLocalizeConfig(id))
}

func (l *localizer) MustLocalizeWithTemplate(id string, fields ...string) string {
	return l.Localizer.MustLocalize(createLocalizeConfigWithTemplate(id, fields...))
}

func createLocalizeConfig(id string) *i18n.LocalizeConfig {
	return &i18n.LocalizeConfig{
		MessageID: id,
	}
}

func createLocalizeConfigWithTemplate(id string, fields ...string) *i18n.LocalizeConfig {
	td := make(map[string]interface{}, len(fields))

	for i, f := range fields {
		td["f"+strconv.Itoa(i+1)] = f
	}

	return &i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: td,
	}
}

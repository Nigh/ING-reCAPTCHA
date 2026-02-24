package bot

import (
	"database/sql"
	"errors"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) t(user *tgbotapi.User, key string, args ...interface{}) string {
	lang := b.getUserLang(user)
	return b.i18n.T(lang, key, args...)
}

func (b *Bot) getUserLang(user *tgbotapi.User) string {
	if user != nil {
		lang, err := b.db.GetUserLanguage(user.ID)
		if err == nil && b.i18n.IsSupported(lang) {
			return lang
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[WARN] Failed to load user language for %d: %v", user.ID, err)
		}

		if user.LanguageCode != "" {
			normalized := normalizeLangCode(user.LanguageCode)
			if normalized != "" && b.i18n.IsSupported(normalized) {
				return normalized
			}
		}
	}

	return normalizeLangCode(b.cfg.DefaultLanguage)
}

func normalizeLangCode(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	if code == "" {
		return ""
	}
	if idx := strings.IndexAny(code, "-_"); idx > 0 {
		return code[:idx]
	}
	return code
}

func (b *Bot) supportedLanguagesList() string {
	return strings.Join(b.i18n.GetSupportedLanguages(), ", ")
}

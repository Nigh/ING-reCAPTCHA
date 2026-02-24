package bot

import (
	"fmt"
)

func (b *Bot) healthCheck() error {
	if b.i18n == nil {
		return fmt.Errorf("i18n translator not initialized")
	}
	if !b.i18n.IsSupported(b.cfg.DefaultLanguage) {
		return fmt.Errorf("default language %q is not supported", b.cfg.DefaultLanguage)
	}

	requiredTables := []string{
		"user_language_preferences",
		"image_set_labels",
	}
	for _, table := range requiredTables {
		ok, err := b.db.HasTable(table)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("missing required table %q", table)
		}
	}

	return nil
}

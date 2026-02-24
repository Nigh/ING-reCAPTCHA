package bot

import (
	"sort"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) cmdSetLang(msg *tgbotapi.Message, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		current := b.getUserLang(msg.From)
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "lang_current", current))
		return
	}

	lang := normalizeLangCode(args)
	if lang == "" || !b.i18n.IsSupported(lang) {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "lang_unsupported", escapeHTML(args), b.supportedLanguagesList()))
		return
	}

	if err := b.db.SetUserLanguage(msg.From.ID, lang); err != nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_db_error"))
		return
	}

	b.sendMessage(msg.Chat.ID, b.t(msg.From, "lang_set_success", lang))
}

func (b *Bot) cmdAddLabel(msg *tgbotapi.Message, args string) {
	if !b.canManageBot(msg) {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_permission_denied"))
		return
	}

	parts := strings.Fields(args)
	if len(parts) < 3 {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_usage_addlabel"))
		return
	}

	setLabel := parts[0]
	lang := normalizeLangCode(parts[1])
	labelText := strings.TrimSpace(strings.Join(parts[2:], " "))
	if labelText == "" {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_usage_addlabel"))
		return
	}

	if lang == "" || !b.i18n.IsSupported(lang) {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "lang_unsupported", escapeHTML(parts[1]), b.supportedLanguagesList()))
		return
	}

	set, err := b.db.GetImageSetByLabel(setLabel)
	if err != nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_db_error"))
		return
	}
	if set == nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_set_not_found", escapeHTML(setLabel)))
		return
	}

	if err := b.db.AddImageSetLabel(set.ID, lang, labelText); err != nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "label_add_failed"))
		return
	}

	b.sendMessage(msg.Chat.ID, b.t(msg.From, "label_added", escapeHTML(setLabel), lang, escapeHTML(labelText)))
}

func (b *Bot) cmdListLabels(msg *tgbotapi.Message, args string) {
	if !b.canManageBot(msg) {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_permission_denied"))
		return
	}

	setLabel := strings.TrimSpace(args)
	if setLabel == "" {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_usage_listlabels"))
		return
	}

	set, err := b.db.GetImageSetByLabel(setLabel)
	if err != nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_db_error"))
		return
	}
	if set == nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_set_not_found", escapeHTML(setLabel)))
		return
	}

	labels, err := b.db.GetImageSetLabels(set.ID)
	if err != nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_db_error"))
		return
	}
	if len(labels) == 0 {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "label_list_empty", escapeHTML(setLabel)))
		return
	}

	var langs []string
	for lang := range labels {
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	var sb strings.Builder
	sb.WriteString(b.t(msg.From, "label_list_title", escapeHTML(setLabel)))
	for _, lang := range langs {
		sb.WriteString(b.t(msg.From, "label_list_item", lang, escapeHTML(labels[lang])))
	}

	b.sendMessage(msg.Chat.ID, sb.String())
}

func (b *Bot) cmdDelLabel(msg *tgbotapi.Message, args string) {
	if !b.canManageBot(msg) {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_permission_denied"))
		return
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_usage_dellabel"))
		return
	}

	setLabel := parts[0]
	lang := normalizeLangCode(parts[1])
	if lang == "" || !b.i18n.IsSupported(lang) {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "lang_unsupported", escapeHTML(parts[1]), b.supportedLanguagesList()))
		return
	}

	set, err := b.db.GetImageSetByLabel(setLabel)
	if err != nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_db_error"))
		return
	}
	if set == nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "err_set_not_found", escapeHTML(setLabel)))
		return
	}

	if err := b.db.DeleteImageSetLabel(set.ID, lang); err != nil {
		b.sendMessage(msg.Chat.ID, b.t(msg.From, "label_delete_failed"))
		return
	}

	b.sendMessage(msg.Chat.ID, b.t(msg.From, "label_deleted", lang, escapeHTML(setLabel)))
}

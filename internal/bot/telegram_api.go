package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// TelegramAPI is the subset of *tgbotapi.BotAPI used by Bot.
// It exists to allow injection of a mock in tests.
type TelegramAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetChatMember(config tgbotapi.GetChatMemberConfig) (tgbotapi.ChatMember, error)
	GetFile(config tgbotapi.FileConfig) (tgbotapi.File, error)
	GetMe() (tgbotapi.User, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

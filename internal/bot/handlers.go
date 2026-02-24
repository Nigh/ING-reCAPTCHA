package bot

import (
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		b.handleCommand(msg)
		return
	}

	// Check if user is pending verification and delete their messages
	if msg.Chat.Type == "group" || msg.Chat.Type == "supergroup" {
		pv, err := b.db.GetPendingVerification(msg.Chat.ID, msg.From.ID)
		if err != nil {
			log.Printf("Error checking pending verification: %v", err)
			return
		}
		if pv != nil {
			// User is pending verification, delete their message
			deleteMsg := tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)
			b.api.Request(deleteMsg)
		}
	}
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	cmd := msg.Command()
	args := msg.CommandArguments()

	switch cmd {
	case "start":
		b.cmdStart(msg)
	case "help":
		b.cmdHelp(msg)
	case "addset":
		b.cmdAddSet(msg, args)
	case "addimage":
		b.cmdAddImage(msg, args)
	case "listsets":
		b.cmdListSets(msg)
	case "delset":
		b.cmdDelSet(msg, args)
	case "setstats":
		b.cmdSetStats(msg)
	case "test":
		b.cmdTest(msg)
	case "addadmin":
		b.cmdAddAdmin(msg, args)
	case "removeadmin":
		b.cmdRemoveAdmin(msg, args)
	case "listadmins":
		b.cmdListAdmins(msg)
	case "myid":
		b.cmdMyID(msg)
	case "setlang":
		b.cmdSetLang(msg, args)
	case "addlabel":
		b.cmdAddLabel(msg, args)
	case "listlabels":
		b.cmdListLabels(msg, args)
	case "dellabel":
		b.cmdDelLabel(msg, args)
	case "cachemissing":
		b.cmdCacheMissing(msg, args)
	}
}

func (b *Bot) handleChatMemberUpdate(update *tgbotapi.ChatMemberUpdated) {
	log.Printf("[DEBUG] Chat member update: old status=%s, new status=%s, user=%d, chat=%d",
		update.OldChatMember.Status, update.NewChatMember.Status,
		update.NewChatMember.User.ID, update.Chat.ID)

	// Only handle new members joining
	if update.NewChatMember.Status != "member" {
		log.Printf("[DEBUG] New status is not 'member', ignoring")
		return
	}

	// Skip if old status was already member (not a new join)
	if update.OldChatMember.Status == "member" {
		log.Printf("[DEBUG] Old status was already 'member', ignoring")
		return
	}

	// Skip bots
	if update.NewChatMember.User.IsBot {
		log.Printf("[DEBUG] User is a bot, ignoring")
		return
	}

	chatID := update.Chat.ID
	userID := update.NewChatMember.User.ID

	log.Printf("[INFO] New member joined: user %d in chat %d", userID, chatID)

	// Check bot permissions in this chat
	if !b.checkBotPermissions(chatID) {
		log.Printf("[ERROR] Bot lacks required permissions in chat %d, cannot verify users", chatID)
		return
	}

	// If user already has a failure record, this is the expected second-attempt flow.
	hasPreviousFailure, err := b.db.HasPreviousFailure(chatID, userID)
	if err != nil {
		log.Printf("[ERROR] Error checking failure history: %v", err)
		hasPreviousFailure = false
	}

	// Check for spam (multiple joins in short time) only for users without failure history.
	if !hasPreviousFailure && b.cfg.RejoinCooldownSeconds > 0 {
		recentJoins, err := b.db.GetRecentJoinCount(chatID, userID, b.cfg.RejoinCooldownSeconds)
		if err != nil {
			log.Printf("[ERROR] Error checking recent joins: %v", err)
		}
		if recentJoins > 0 {
			until := time.Now().Add(time.Duration(b.cfg.RejoinCooldownSeconds) * time.Second).Unix()
			log.Printf("[WARN] User %d is rejoining too quickly (%d recent joins), temporary ban until %d", userID, recentJoins, until)
			kickConfig := tgbotapi.KickChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{
					ChatID: chatID,
					UserID: userID,
				},
				UntilDate: until,
			}
			if _, err := b.api.Request(kickConfig); err != nil {
				log.Printf("[ERROR] Failed to temporary-ban spam rejoiner: %v", err)
			}
			return
		}
	}

	// Record join
	log.Printf("[DEBUG] Recording user join in database")
	if err := b.db.RecordUserJoin(chatID, userID); err != nil {
		log.Printf("[ERROR] Error recording user join: %v", err)
	}

	// Restrict user permissions
	log.Printf("[DEBUG] Restricting user %d permissions", userID)
	if err := b.restrictUser(chatID, userID); err != nil {
		log.Printf("[ERROR] Error restricting user: %v", err)
		return
	}

	// Start verification
	log.Printf("[DEBUG] Starting verification for user %d", userID)
	if err := b.startVerification(chatID, userID, update.NewChatMember.User); err != nil {
		log.Printf("[ERROR] Error starting verification: %v", err)
		// Unrestrict user if verification fails to start
		log.Printf("[DEBUG] Unrestricting user due to verification start failure")
		b.unrestrictUser(chatID, userID)
	}
}

func (b *Bot) restrictUser(chatID, userID int64) error {
	permissions := tgbotapi.ChatPermissions{
		CanSendMessages:       false,
		CanSendMediaMessages:  false,
		CanSendOtherMessages:  false,
		CanAddWebPagePreviews: false,
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		Permissions: &permissions,
	}

	_, err := b.api.Request(restrictConfig)
	return err
}

func (b *Bot) unrestrictUser(chatID, userID int64) error {
	permissions := tgbotapi.ChatPermissions{
		CanSendMessages:       true,
		CanSendMediaMessages:  true,
		CanSendOtherMessages:  true,
		CanAddWebPagePreviews: true,
		CanSendPolls:          true,
		CanInviteUsers:        true,
		CanPinMessages:        false,
		CanChangeInfo:         false,
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		Permissions: &permissions,
	}

	_, err := b.api.Request(restrictConfig)
	return err
}

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	b.api.Send(msg)
}

func (b *Bot) isGroupAdmin(chatID, userID int64) bool {
	member, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	})
	if err != nil {
		return false
	}
	return member.Status == "administrator" || member.Status == "creator"
}

func (b *Bot) isBotAdmin(userID int64) bool {
	isAdmin, err := b.db.IsAdmin(userID)
	if err != nil {
		return false
	}
	return isAdmin
}

func (b *Bot) canManageBot(msg *tgbotapi.Message) bool {
	// Only bot admins can manage image sets (both in private chat and groups)
	// This prevents malicious group admins from modifying the shared image database
	return b.isBotAdmin(msg.From.ID)
}

func (b *Bot) checkBotPermissions(chatID int64) bool {
	// Get bot's member status in the chat
	botMember, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: b.self.ID,
		},
	})
	if err != nil {
		log.Printf("[ERROR] Failed to get bot member status: %v", err)
		return false
	}

	// Check if bot is admin
	if botMember.Status != "administrator" && botMember.Status != "creator" {
		log.Printf("[WARN] Bot is not an administrator in chat %d (status: %s)", chatID, botMember.Status)
		return false
	}

	// Check required permissions
	requiredPerms := map[string]bool{
		"can_restrict_members": botMember.CanRestrictMembers,
		"can_delete_messages":  botMember.CanDeleteMessages,
	}

	missingPerms := []string{}
	for perm, has := range requiredPerms {
		if !has {
			missingPerms = append(missingPerms, perm)
		}
	}

	if len(missingPerms) > 0 {
		log.Printf("[WARN] Bot lacks required permissions in chat %d: %v", chatID, missingPerms)
		return false
	}

	log.Printf("[DEBUG] Bot has all required permissions in chat %d", chatID)
	return true
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

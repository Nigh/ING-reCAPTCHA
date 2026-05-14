package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/szres/ing-recaptcha/internal/ai"
	"github.com/szres/ing-recaptcha/internal/database"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func (b *Bot) startVerification(chatID, userID int64, user *tgbotapi.User) error {
	return b.startVerificationInternal(chatID, userID, user, false)
}

func (b *Bot) startTestVerification(chatID, userID int64, user *tgbotapi.User) error {
	return b.startVerificationInternal(chatID, userID, user, true)
}

func (b *Bot) startVerificationInternal(chatID, userID int64, user *tgbotapi.User, isTest bool) error {
	log.Printf("[DEBUG] Starting verification for user %d in chat %d (test mode: %v)", userID, chatID, isTest)
	userLang := b.getUserLang(user)

	// Check if there's already a pending verification for this user
	// Purpose: Only to get old MessageID for cleanup, data overwrite is handled by CreatePendingVerification's UPSERT
	existingPV, err := b.db.GetPendingVerification(chatID, userID)
	if err != nil {
		log.Printf("[WARN] Failed to check existing verification: %v", err)
	}

	if existingPV != nil && existingPV.MessageID.Valid {
		oldMessageID := int(existingPV.MessageID.Int64)
		log.Printf("[DEBUG] Found existing verification, deleting old message %d", oldMessageID)

		// Delete old message to avoid confusion
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, oldMessageID)
		if _, err := b.api.Request(deleteMsg); err != nil {
			// Ignore "message not found" errors (user may have deleted it)
			log.Printf("[DEBUG] Failed to delete old verification message (may already be deleted): %v", err)
		}
	}
	// Note: No need to call DeletePendingVerification here
	// CreatePendingVerification uses ON CONFLICT DO UPDATE to handle overwrites

	// Determine question count via AI risk assessment or fallback to config
	questionCount := b.cfg.VerifyImageCount
	if b.ai.IsEnabled() {
		aiCtx, aiCancel := context.WithTimeout(context.Background(), time.Duration(b.cfg.ModelTimeoutSeconds)*time.Second)
		defer aiCancel()

		userInfo := &ai.UserInfo{
			UserID:       user.ID,
			Username:     user.UserName,
			FirstName:    user.FirstName,
			LastName:     user.LastName,
			LanguageCode: user.LanguageCode,
			IsBot:        user.IsBot,
		}

		if count, err := b.ai.AssessRisk(aiCtx, userInfo); err != nil {
			log.Printf("[WARN] AI risk assessment failed, using default count %d: %v", questionCount, err)
		} else {
			questionCount = count
			log.Printf("[INFO] AI assessed risk for user %d, question count: %d", userID, questionCount)
		}
	}

	// Check if we have enough image sets
	setCount, err := b.db.GetImageSetCount()
	if err != nil {
		return fmt.Errorf("failed to get image set count: %w", err)
	}

	// Clamp questionCount to available sets
	maxQuestions := setCount - b.cfg.VerifyDistractorCount
	if maxQuestions < 3 {
		maxQuestions = 3
	}
	if questionCount > maxQuestions {
		log.Printf("[INFO] Clamping question count from %d to %d (max available)", questionCount, maxQuestions)
		questionCount = maxQuestions
	}
	if questionCount > 7 {
		questionCount = 7
	}
	if questionCount < 3 {
		questionCount = 3
	}

	// We need: correct answers + distractors
	minSets := questionCount + b.cfg.VerifyDistractorCount
	log.Printf("[DEBUG] Image set count: %d (minimum required: %d)", setCount, minSets)
	if setCount < minSets {
		if isTest {
			return fmt.Errorf(b.t(user, "err_not_enough_sets", setCount, minSets))
		}
		log.Printf("[WARN] Not enough image sets (%d < %d), auto-approving user", setCount, minSets)
		return b.unrestrictUser(chatID, userID)
	}

	// Get random image sets for the challenge
	log.Printf("[DEBUG] Getting %d random image sets", questionCount)
	challengeSets, err := b.db.GetRandomImageSets(questionCount)
	if err != nil {
		return fmt.Errorf("failed to get random image sets: %w", err)
	}

	// Get one random image from each set
	var imagePaths []string
	var correctLabels []string

	for i, set := range challengeSets {
		log.Printf("[DEBUG] Getting random image from set %d: %s", i+1, set.Label)
		img, err := b.db.GetRandomImageFromSet(set.ID)
		if err != nil || img == nil {
			return fmt.Errorf("failed to get image from set %s: %w", set.Label, err)
		}

		// Download image if not cached
		imgPath := img.FilePath.String
		if !img.FilePath.Valid || imgPath == "" {
			log.Printf("[DEBUG] Image not cached, downloading file_id: %s", img.FileID)
			imgPath, err = b.downloadAndCacheImage(img.FileID, img.ID)
			if err != nil {
				return fmt.Errorf("failed to download image: %w", err)
			}
		} else {
			log.Printf("[DEBUG] Using cached image: %s", imgPath)
		}

		imagePaths = append(imagePaths, imgPath)
		correctLabels = append(correctLabels, set.Label)
	}

	log.Printf("[DEBUG] Correct labels for verification: %v", correctLabels)

	// Compose verification image with bounded concurrency and timeout to avoid global stalls.
	select {
	case b.workerPool <- struct{}{}:
	case <-time.After(5 * time.Second):
		return fmt.Errorf("compose worker pool is busy")
	}
	defer func() {
		<-b.workerPool
	}()

	composedPath, err := b.composer.ComposeVerificationImage(imagePaths)
	if err != nil {
		return fmt.Errorf("failed to compose verification image: %w", err)
	}
	defer b.composer.CleanupTempFile(composedPath)

	// Get distractor labels
	allSets, err := b.db.GetAllImageSets()
	if err != nil {
		return fmt.Errorf("failed to get all image sets: %w", err)
	}

	var distractorLabels []string
	correctMap := make(map[string]bool)
	for _, l := range correctLabels {
		correctMap[l] = true
	}
	for _, set := range allSets {
		if !correctMap[set.Label] {
			distractorLabels = append(distractorLabels, set.Label)
		}
	}

	// Shuffle and take configured number of distractors
	rand.Shuffle(len(distractorLabels), func(i, j int) {
		distractorLabels[i], distractorLabels[j] = distractorLabels[j], distractorLabels[i]
	})
	numDistractors := b.cfg.VerifyDistractorCount
	if len(distractorLabels) < numDistractors {
		numDistractors = len(distractorLabels)
	}
	distractorLabels = distractorLabels[:numDistractors]

	// Combine and shuffle all options
	allOptions := append(correctLabels, distractorLabels...)
	log.Printf("[DEBUG] All options before shuffle: %v", allOptions)
	rand.Shuffle(len(allOptions), func(i, j int) {
		allOptions[i], allOptions[j] = allOptions[j], allOptions[i]
	})
	log.Printf("[DEBUG] All options after shuffle: %v", allOptions)

	// Create pending verification record
	expiresAt := time.Now().Add(time.Duration(b.cfg.VerifyTimeoutSeconds) * time.Second)
	log.Printf("[DEBUG] Creating pending verification, expires at: %v", expiresAt)
	if err := b.db.CreatePendingVerification(chatID, userID, correctLabels, questionCount, expiresAt); err != nil {
		return fmt.Errorf("failed to create pending verification: %w", err)
	}

	// Build inline keyboard
	keyboard := b.buildVerificationKeyboard(userID, 0, allOptions, userLang)

	// Send verification message
	userName := user.FirstName
	if user.LastName != "" {
		userName += " " + user.LastName
	}

	caption := b.t(user, "verify_welcome",
		escapeHTML(userName),
		b.cfg.VerifyTimeoutSeconds,
		questionCount,
	)

	log.Printf("[DEBUG] Sending verification message to chat %d", chatID)
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(composedPath))
	photo.Caption = caption
	photo.ParseMode = "HTML"
	photo.ReplyMarkup = keyboard

	sentMsg, err := b.api.Send(photo)
	if err != nil {
		return fmt.Errorf("failed to send verification message: %w", err)
	}

	log.Printf("[DEBUG] Verification message sent, message ID: %d", sentMsg.MessageID)

	// Update message ID in database
	if err := b.db.UpdateVerificationMessageID(chatID, userID, sentMsg.MessageID); err != nil {
		log.Printf("[ERROR] Failed to update message ID: %v", err)
	} else {
		log.Printf("[DEBUG] Updated message ID in database")
	}

	return nil
}

func (b *Bot) buildVerificationKeyboard(userID int64, step int, options []string, userLang string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i := 0; i < len(options); i += 2 {
		var row []tgbotapi.InlineKeyboardButton
		for j := i; j < i+2 && j < len(options); j++ {
			callbackData := fmt.Sprintf("v:%d:%d:%s", userID, step, options[j])
			displayLabel := b.getDisplayLabelBySetLabel(options[j])
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(displayLabel, callbackData))
		}
		rows = append(rows, row)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (b *Bot) getDisplayLabelBySetLabel(setLabel string) string {
	set, err := b.db.GetImageSetByLabel(setLabel)
	if err != nil {
		log.Printf("[WARN] Failed to resolve image set %q: %v", setLabel, err)
		return setLabel
	}
	if set == nil {
		log.Printf("[WARN] Image set not found for label %q", setLabel)
		return setLabel
	}

	labels, err := b.db.GetImageSetLabels(set.ID)
	if err != nil {
		log.Printf("[WARN] Failed to get labels for set %d: %v", set.ID, err)
		return set.Label
	}

	return buildMultiLangLabel(labels, set.Label)
}

// buildMultiLangLabel joins all unique label values into a single display string.
// Language priority: zh first, en second, then remaining languages in sorted order.
// Duplicate label values (same text in multiple languages) are shown only once.
func buildMultiLangLabel(labels map[string]string, fallback string) string {
	if len(labels) == 0 {
		return fallback
	}

	priority := []string{"zh", "en"}
	seen := make(map[string]bool)
	var parts []string

	for _, lang := range priority {
		if label, ok := labels[lang]; ok && !seen[label] {
			parts = append(parts, label)
			seen[label] = true
		}
	}

	// Collect remaining languages in sorted order
	var rest []string
	for lang := range labels {
		isPriority := false
		for _, p := range priority {
			if lang == p {
				isPriority = true
				break
			}
		}
		if !isPriority {
			rest = append(rest, lang)
		}
	}
	sort.Strings(rest)
	for _, lang := range rest {
		if label := labels[lang]; !seen[label] {
			parts = append(parts, label)
			seen[label] = true
		}
	}

	if len(parts) == 0 {
		return fallback
	}
	return strings.Join(parts, " / ")
}

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	data := query.Data

	log.Printf("[DEBUG] Received callback query from user %d: %s", query.From.ID, data)

	// Parse callback data: v:{user_id}:{step}:{label}
	if !strings.HasPrefix(data, "v:") {
		log.Printf("[DEBUG] Callback data does not start with 'v:', ignoring")
		return
	}

	parts := strings.SplitN(data[2:], ":", 3)
	if len(parts) != 3 {
		log.Printf("[WARN] Invalid callback data format: %s", data)
		return
	}

	targetUserID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		log.Printf("[ERROR] Failed to parse target user ID: %v", err)
		return
	}

	// Verify the callback is from the correct user
	if query.From.ID != targetUserID {
		log.Printf("[WARN] Callback from user %d but target is %d", query.From.ID, targetUserID)
		callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_callback_not_yours"))
		b.api.Request(callback)
		return
	}

	step, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Printf("[ERROR] Failed to parse step: %v", err)
		return
	}

	selectedLabel := parts[2]
	chatID := query.Message.Chat.ID

	log.Printf("[DEBUG] User %d selected label '%s' for step %d in chat %d", targetUserID, selectedLabel, step, chatID)

	// Get pending verification
	pv, err := b.db.GetPendingVerification(chatID, targetUserID)
	if err != nil || pv == nil {
		log.Printf("[WARN] No pending verification found for user %d in chat %d: %v", targetUserID, chatID, err)
		callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_callback_expired"))
		b.api.Request(callback)
		return
	}

	// Check if step matches
	if step != pv.CurrentStep {
		log.Printf("[WARN] Step mismatch: expected %d, got %d", pv.CurrentStep, step)
		callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_callback_order"))
		b.api.Request(callback)
		return
	}

	// Record answer
	answers := append(pv.UserAnswers, selectedLabel)
	newStep := step + 1

	log.Printf("[DEBUG] Recording answer, new step: %d, total answers: %d", newStep, len(answers))

	if err := b.db.UpdateVerificationStep(chatID, targetUserID, newStep, answers); err != nil {
		log.Printf("[ERROR] Failed to update verification step: %v", err)
		return
	}

	// Check if all answers collected
	if newStep >= pv.QuestionCount {
		log.Printf("[DEBUG] All answers collected, evaluating verification")
		b.evaluateVerification(query, pv, answers)
		return
	}

	// Update message to show next step
	caption := b.t(query.From, "verify_step_prompt",
		newStep,
		pv.QuestionCount,
		newStep+1,
	)

	// Get all options from current keyboard
	var options []string
	if query.Message.ReplyMarkup != nil {
		for _, row := range query.Message.ReplyMarkup.InlineKeyboard {
			for _, btn := range row {
				// Extract label from callback data
				if btn.CallbackData != nil {
					btnParts := strings.SplitN((*btn.CallbackData)[2:], ":", 3)
					if len(btnParts) == 3 {
						options = append(options, btnParts[2])
					}
				}
			}
		}
	}

	log.Printf("[DEBUG] Updating keyboard with %d options for step %d", len(options), newStep)

	keyboard := b.buildVerificationKeyboard(targetUserID, newStep, options, b.getUserLang(query.From))

	editMsg := tgbotapi.NewEditMessageCaption(chatID, query.Message.MessageID, caption)
	editMsg.ParseMode = "HTML"
	editMsg.ReplyMarkup = &keyboard
	if _, err := b.api.Send(editMsg); err != nil {
		log.Printf("[ERROR] Failed to update message caption: %v", err)
	}

	displayLabel := b.getDisplayLabelBySetLabel(selectedLabel)
	callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_callback_selected", displayLabel))
	b.api.Request(callback)
}

func (b *Bot) evaluateVerification(query *tgbotapi.CallbackQuery, pv *database.PendingVerification, answers []string) {
	chatID := query.Message.Chat.ID
	userID := pv.UserID

	log.Printf("[DEBUG] Evaluating verification for user %d in chat %d", userID, chatID)
	log.Printf("[DEBUG] User answers: %v", answers)
	log.Printf("[DEBUG] Correct labels: %v", pv.CorrectLabels)

	// Count correct answers
	correctCount := 0
	for i, answer := range answers {
		if i < len(pv.CorrectLabels) && answer == pv.CorrectLabels[i] {
			correctCount++
			log.Printf("[DEBUG] Answer %d correct: %s", i+1, answer)
		} else if i < len(pv.CorrectLabels) {
			log.Printf("[DEBUG] Answer %d incorrect: got %s, expected %s", i+1, answer, pv.CorrectLabels[i])
		}
	}

	log.Printf("[DEBUG] Correct count: %d/%d (required: %d)", correctCount, len(answers), b.cfg.VerifyRequiredCorrect)

	if correctCount >= b.cfg.VerifyRequiredCorrect {
		// Verification passed
		log.Printf("[INFO] User %d passed verification (%d/%d correct)", userID, correctCount, len(answers))
		b.handleVerificationSuccess(query, pv)
	} else {
		// Verification failed - no retry, kick immediately
		log.Printf("[INFO] User %d failed verification (%d/%d correct)", userID, correctCount, len(answers))
		b.handleVerificationFailure(query, pv)
	}

	// Clean up
	log.Printf("[DEBUG] Cleaning up pending verification for user %d", userID)
	if err := b.db.DeletePendingVerification(chatID, userID); err != nil {
		log.Printf("[ERROR] Failed to delete pending verification: %v", err)
	}
}

func (b *Bot) handleVerificationSuccess(query *tgbotapi.CallbackQuery, pv *database.PendingVerification) {
	chatID := query.Message.Chat.ID
	userID := pv.UserID

	log.Printf("[DEBUG] Handling verification success for user %d in chat %d", userID, chatID)

	// Clear any previous failure record
	if err := b.db.ClearVerificationFailure(chatID, userID); err != nil {
		log.Printf("[ERROR] Failed to clear failure record: %v", err)
	} else {
		log.Printf("[DEBUG] Cleared any previous failure record for user %d", userID)
	}

	// Try to unrestrict user (may fail for admins, which is fine)
	log.Printf("[DEBUG] Attempting to unrestrict user %d", userID)
	if err := b.unrestrictUser(chatID, userID); err != nil {
		log.Printf("[WARN] Failed to unrestrict user (may be admin): %v", err)
	} else {
		log.Printf("[DEBUG] Successfully unrestricted user %d", userID)
	}

	// Delete verification message
	log.Printf("[DEBUG] Attempting to delete verification message %d", query.Message.MessageID)
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, query.Message.MessageID)
	resp, err := b.api.Request(deleteMsg)
	if err != nil {
		log.Printf("[ERROR] Failed to delete verification message: %v", err)
	} else {
		log.Printf("[DEBUG] Successfully deleted verification message, response: %+v", resp)
	}

	// Send welcome message
	log.Printf("[DEBUG] Sending welcome message to chat %d", chatID)
	userName := query.From.FirstName
	if query.From.LastName != "" {
		userName += " " + query.From.LastName
	}
	welcomeText := b.t(query.From, "verify_passed", escapeHTML(userName))
	welcomeMsg := tgbotapi.NewMessage(chatID, welcomeText)
	welcomeMsg.ParseMode = "HTML"
	if _, err := b.api.Send(welcomeMsg); err != nil {
		log.Printf("[ERROR] Failed to send welcome message: %v", err)
	}

	callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_passed_callback"))
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("[ERROR] Failed to send callback response: %v", err)
	}

	log.Printf("[INFO] User %d passed verification in chat %d", userID, chatID)
}

func (b *Bot) handleVerificationRetry(query *tgbotapi.CallbackQuery, pv *database.PendingVerification) {
	chatID := query.Message.Chat.ID
	userID := pv.UserID

	log.Printf("[DEBUG] Handling verification retry for user %d in chat %d (current retry count: %d)", userID, chatID, pv.RetryCount)

	// Increment retry count
	retryCount, err := b.db.IncrementRetryCount(chatID, userID)
	if err != nil {
		log.Printf("[ERROR] Failed to increment retry count: %v", err)
	} else {
		log.Printf("[DEBUG] Incremented retry count to %d", retryCount)
	}

	// Delete old message
	log.Printf("[DEBUG] Deleting old verification message %d", query.Message.MessageID)
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, query.Message.MessageID)
	resp, err := b.api.Request(deleteMsg)
	if err != nil {
		log.Printf("[ERROR] Failed to delete old verification message: %v", err)
	} else {
		log.Printf("[DEBUG] Successfully deleted old message, response: %+v", resp)
	}

	callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_failed_retry", b.cfg.VerifyMaxRetry-retryCount+1))
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("[ERROR] Failed to send callback response: %v", err)
	}

	// Start new verification (use test mode if user is admin)
	user := query.From
	if b.isGroupAdmin(chatID, userID) {
		log.Printf("[DEBUG] User %d is admin, starting test verification", userID)
		if err := b.startTestVerification(chatID, userID, user); err != nil {
			log.Printf("[ERROR] Failed to restart test verification: %v", err)
		}
	} else {
		log.Printf("[DEBUG] Starting new verification for user %d", userID)
		if err := b.startVerification(chatID, userID, user); err != nil {
			log.Printf("[ERROR] Failed to restart verification: %v", err)
			b.unrestrictUser(chatID, userID)
		}
	}
}

func (b *Bot) handleVerificationFailure(query *tgbotapi.CallbackQuery, pv *database.PendingVerification) {
	chatID := query.Message.Chat.ID
	userID := pv.UserID

	log.Printf("[DEBUG] Handling verification failure for user %d in chat %d", userID, chatID)

	// Delete verification message
	log.Printf("[DEBUG] Deleting verification message %d", query.Message.MessageID)
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, query.Message.MessageID)
	resp, err := b.api.Request(deleteMsg)
	if err != nil {
		log.Printf("[ERROR] Failed to delete verification message: %v", err)
	} else {
		log.Printf("[DEBUG] Successfully deleted message, response: %+v", resp)
	}

	// Check if user is admin (can't kick admins)
	if b.isGroupAdmin(chatID, userID) {
		log.Printf("[DEBUG] User %d is admin, skipping kick (test mode)", userID)
		callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_failed_test"))
		b.api.Request(callback)
		log.Printf("[INFO] Admin %d failed test verification in chat %d", userID, chatID)
		return
	}

	// Check if user has previous failure
	hasPreviousFailure, err := b.db.HasPreviousFailure(chatID, userID)
	if err != nil {
		log.Printf("[ERROR] Failed to check previous failure: %v", err)
		hasPreviousFailure = false
	}

	if hasPreviousFailure {
		// Second failure - kick and ban permanently
		log.Printf("[INFO] User %d has previous failure, kicking with permanent ban", userID)
		kickConfig := tgbotapi.KickChatMemberConfig{
			ChatMemberConfig: tgbotapi.ChatMemberConfig{
				ChatID: chatID,
				UserID: userID,
			},
			// UntilDate = 0 means permanent ban
		}
		kickResp, err := b.api.Request(kickConfig)
		if err != nil {
			log.Printf("[ERROR] Failed to kick and ban user %d: %v", userID, err)
		} else {
			log.Printf("[DEBUG] Successfully kicked and banned user %d, response: %+v", userID, kickResp)

			// Only delete join history if kick succeeded
			if err := b.db.DeleteUserJoinHistory(chatID, userID); err != nil {
				log.Printf("[ERROR] Failed to delete join history: %v", err)
			}
		}

		// Clear failure record after permanent ban
		if err := b.db.ClearVerificationFailure(chatID, userID); err != nil {
			log.Printf("[ERROR] Failed to clear failure record: %v", err)
		}

		callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_failed_banned"))
		if _, err := b.api.Request(callback); err != nil {
			log.Printf("[ERROR] Failed to send callback response: %v", err)
		}

		log.Printf("[INFO] User %d failed verification (2nd time) in chat %d and was permanently banned", userID, chatID)
	} else {
		// First failure - kick without ban (can rejoin immediately)
		log.Printf("[INFO] User %d first failure, kicking without ban", userID)

		// Record the failure
		if err := b.db.RecordVerificationFailure(chatID, userID); err != nil {
			log.Printf("[ERROR] Failed to record verification failure: %v", err)
		}

		// Kick user (permanent ban first)
		kickConfig := tgbotapi.KickChatMemberConfig{
			ChatMemberConfig: tgbotapi.ChatMemberConfig{
				ChatID: chatID,
				UserID: userID,
			},
			// No UntilDate = permanent ban
		}
		kickResp, err := b.api.Request(kickConfig)
		if err != nil {
			log.Printf("[ERROR] Failed to kick user %d: %v", userID, err)
		} else {
			log.Printf("[DEBUG] Successfully kicked user %d, response: %+v", userID, kickResp)

			// Only delete join history if kick succeeded (before unban to minimize race window)
			if err := b.db.DeleteUserJoinHistory(chatID, userID); err != nil {
				log.Printf("[ERROR] Failed to delete join history: %v", err)
			}

			// Immediately unban to allow rejoin
			unbanConfig := tgbotapi.UnbanChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{
					ChatID: chatID,
					UserID: userID,
				},
				OnlyIfBanned: true,
			}
			unbanResp, err := b.api.Request(unbanConfig)
			if err != nil {
				log.Printf("[ERROR] Failed to unban user %d: %v", userID, err)
			} else {
				log.Printf("[DEBUG] Successfully unbanned user %d (can rejoin), response: %+v", userID, unbanResp)
			}
		}

		callback := tgbotapi.NewCallback(query.ID, b.t(query.From, "verify_failed_kicked"))
		if _, err := b.api.Request(callback); err != nil {
			log.Printf("[ERROR] Failed to send callback response: %v", err)
		}

		log.Printf("[INFO] User %d failed verification (1st time) in chat %d and was kicked (no ban)", userID, chatID)
	}
}

func (b *Bot) downloadAndCacheImage(fileID string, imageID int64) (string, error) {
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("failed to get file: %w", err)
	}

	fileURL := file.Link(b.token)

	resp, err := httpClient.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	ext := filepath.Ext(file.FilePath)
	if ext == "" {
		ext = ".jpg"
	}

	localPath := filepath.Join(b.cfg.ImageCachePath, fmt.Sprintf("%d%s", imageID, ext))

	out, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Update database with local path
	if err := b.db.UpdateImageFilePath(imageID, localPath); err != nil {
		log.Printf("Failed to update image file path: %v", err)
	}

	return localPath, nil
}

// OptionsData stores the shuffled options for a verification session
type OptionsData struct {
	Options []string `json:"options"`
}

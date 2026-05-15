package ai

import "fmt"

type UserInfo struct {
	UserID           int64
	Username         string
	FirstName        string
	LastName         string
	LanguageCode     string
	IsBot            bool
	ProfilePhotoData []byte
}

func buildSystemPrompt(hasPhoto bool) string {
	base := `You are a risk assessment system for a Telegram group verification bot. Your job is to evaluate how likely a new user is to be a spam bot or malicious actor, and return a difficulty score.

Consider these factors:
- Username patterns (random strings, numbers-only, suspicious patterns suggest bots)
- Account age hints (very new accounts are riskier)
- Name patterns (first/last name with random characters, all caps, emoji-heavy)
- Language code (matching group language is lower risk)
- Whether the user has common/realistic names
- Whether the user appears to be a bot (IsBot flag)`

	if hasPhoto {
		base += `
- Profile photo analysis: a real person's photo lowers risk; no photo, a generic/stock image, anime/cartoon avatar, celebrity photo, or overly polished AI-generated image raises risk; sexually explicit or provocative avatars are high risk`
	} else {
		base += `
- The user has no profile photo, which slightly increases risk`
	}

	base += `

You MUST respond with ONLY a JSON object in this exact format:
{"question_count": N}

Where N is an integer from 3 (lowest risk, easiest verification) to 7 (highest risk, hardest verification).
Do NOT include any other text, explanation, or markdown formatting.`

	return base
}

func buildUserPrompt(info *UserInfo) string {
	isBotStr := "no"
	if info.IsBot {
		isBotStr = "yes"
	}

	username := info.Username
	if username == "" {
		username = "(none)"
	}

	lastName := info.LastName
	if lastName == "" {
		lastName = "(none)"
	}

	langCode := info.LanguageCode
	if langCode == "" {
		langCode = "(unknown)"
	}

	photoStatus := "no"
	if len(info.ProfilePhotoData) > 0 {
		photoStatus = "yes (see attached image)"
	}

	return fmt.Sprintf(`Evaluate the risk level of this Telegram user joining a group:

- User ID: %d
- Username: %s
- First name: %s
- Last name: %s
- Language code: %s
- Is bot: %s
- Has profile photo: %s

Return ONLY the JSON object with question_count.`,
		info.UserID, username, info.FirstName, lastName, langCode, isBotStr, photoStatus)
}

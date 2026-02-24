package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/szres/ing-recaptcha/internal/config"
	"github.com/szres/ing-recaptcha/internal/database"
	"github.com/szres/ing-recaptcha/internal/i18n"
)

func TestHealthCheck(t *testing.T) {
	t.Run("migrated DB and valid i18n returns no error", func(t *testing.T) {
		b, _ := newTestBot(t)
		err := b.healthCheck()
		assert.NoError(t, err)
	})

	t.Run("unmigrated DB returns error for missing table", func(t *testing.T) {
		// Open a fresh in-memory DB without running migrations.
		db, err := database.New(":memory:")
		require.NoError(t, err)
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { db.Close() })

		tr, err := i18n.New("en")
		require.NoError(t, err)

		b := &Bot{
			db:   db,
			i18n: tr,
			cfg: &config.Config{
				DefaultLanguage: "en",
			},
		}
		// cfg is the first return value; discard the mock
		err = b.healthCheck()
		assert.Error(t, err, "expected error when required tables are absent")
	})
}

func TestIsBotAdmin(t *testing.T) {
	const userID int64 = 42

	t.Run("user not in admins table returns false", func(t *testing.T) {
		b, _ := newTestBot(t)
		assert.False(t, b.isBotAdmin(userID))
	})

	t.Run("user added via AddAdmin returns true", func(t *testing.T) {
		b, _ := newTestBot(t)
		require.NoError(t, b.db.AddAdmin(userID))
		assert.True(t, b.isBotAdmin(userID))
	})
}

func TestCanManageBot(t *testing.T) {
	const userID int64 = 99

	makeMsg := func(id int64) *tgbotapi.Message {
		return &tgbotapi.Message{
			From: &tgbotapi.User{ID: id},
		}
	}

	t.Run("non-admin cannot manage bot", func(t *testing.T) {
		b, _ := newTestBot(t)
		assert.False(t, b.canManageBot(makeMsg(userID)))
	})

	t.Run("bot admin can manage bot", func(t *testing.T) {
		b, _ := newTestBot(t)
		require.NoError(t, b.db.AddAdmin(userID))
		assert.True(t, b.canManageBot(makeMsg(userID)))
	})
}

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("disabled when any field empty", func(t *testing.T) {
		c := NewClient("", "key", "model", 5)
		assert.False(t, c.IsEnabled())

		c = NewClient("url", "", "model", 5)
		assert.False(t, c.IsEnabled())

		c = NewClient("url", "key", "", 5)
		assert.False(t, c.IsEnabled())
	})

	t.Run("enabled when all fields set", func(t *testing.T) {
		c := NewClient("http://localhost", "key", "model", 5)
		assert.True(t, c.IsEnabled())
	})
}

func TestClampQuestionCount(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{-1, 3},
		{0, 3},
		{2, 3},
		{3, 3},
		{5, 5},
		{7, 7},
		{8, 7},
		{100, 7},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d->%d", tc.input, tc.want), func(t *testing.T) {
			assert.Equal(t, tc.want, clampQuestionCount(tc.input))
		})
	}
}

func TestParseQuestionCount(t *testing.T) {
	t.Run("plain JSON", func(t *testing.T) {
		n, err := parseQuestionCount(`{"question_count": 5}`)
		require.NoError(t, err)
		assert.Equal(t, 5, n)
	})

	t.Run("JSON with extra fields", func(t *testing.T) {
		n, err := parseQuestionCount(`{"risk_level": 3, "question_count": 6, "reason": "test"}`)
		require.NoError(t, err)
		assert.Equal(t, 6, n)
	})

	t.Run("JSON in markdown code block", func(t *testing.T) {
		n, err := parseQuestionCount("Here is the result:\n```json\n{\"question_count\": 4}\n```\nDone.")
		require.NoError(t, err)
		assert.Equal(t, 4, n)
	})

	t.Run("JSON in plain code block", func(t *testing.T) {
		n, err := parseQuestionCount("```\n{\"question_count\": 7}\n```")
		require.NoError(t, err)
		assert.Equal(t, 7, n)
	})

	t.Run("value below 3 is clamped", func(t *testing.T) {
		n, err := parseQuestionCount(`{"question_count": 1}`)
		require.NoError(t, err)
		assert.Equal(t, 3, n)
	})

	t.Run("value above 7 is clamped", func(t *testing.T) {
		n, err := parseQuestionCount(`{"question_count": 10}`)
		require.NoError(t, err)
		assert.Equal(t, 7, n)
	})

	t.Run("fallback regex extraction", func(t *testing.T) {
		n, err := parseQuestionCount(`The answer is "question_count": 5 based on my analysis.`)
		require.NoError(t, err)
		assert.Equal(t, 5, n)
	})

	t.Run("unparseable returns error", func(t *testing.T) {
		_, err := parseQuestionCount("I cannot comply with this request.")
		assert.Error(t, err)
	})
}

func TestHealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/chat/completions", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{{Message: struct {
					Content string `json:"content"`
				}{Content: "pong"}}},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		err := c.HealthCheck(context.Background())
		assert.NoError(t, err)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		err := c.HealthCheck(context.Background())
		assert.Error(t, err)
	})

	t.Run("API error in response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(chatResponse{
				Error: &struct {
					Message string `json:"message"`
				}{Message: "invalid model"},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		err := c.HealthCheck(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid model")
	})
}

func TestAssessRisk(t *testing.T) {
	t.Run("returns question count on valid response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req chatRequest
			json.NewDecoder(r.Body).Decode(&req)
			assert.Len(t, req.Messages, 2)
			assert.Equal(t, "system", req.Messages[0].Role)
			assert.Equal(t, "user", req.Messages[1].Role)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{{Message: struct {
					Content string `json:"content"`
				}{Content: `{"question_count": 5}`}}},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		info := &UserInfo{
			UserID:       12345,
			Username:     "testuser",
			FirstName:    "Test",
			LastName:     "User",
			LanguageCode: "en",
			IsBot:        false,
		}

		n, err := c.AssessRisk(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, 5, n)
	})

	t.Run("returns error when disabled", func(t *testing.T) {
		c := NewClient("", "", "", 5)
		info := &UserInfo{UserID: 1}
		_, err := c.AssessRisk(context.Background(), info)
		assert.Error(t, err)
	})

	t.Run("clamps out of range values", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{{Message: struct {
					Content string `json:"content"`
				}{Content: `{"question_count": 10}`}}},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		info := &UserInfo{UserID: 1}

		n, err := c.AssessRisk(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, 7, n)
	})

	t.Run("handles context timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Don't respond, let context cancel
			<-r.Context().Done()
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		info := &UserInfo{UserID: 1}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := c.AssessRisk(ctx, info)
		assert.Error(t, err)
	})
}

func TestBuildUserPrompt(t *testing.T) {
	info := &UserInfo{
		UserID:       42,
		Username:     "johndoe",
		FirstName:    "John",
		LastName:     "Doe",
		LanguageCode: "en",
		IsBot:        false,
	}
	prompt := buildUserPrompt(info)
	assert.Contains(t, prompt, "42")
	assert.Contains(t, prompt, "johndoe")
	assert.Contains(t, prompt, "John")
	assert.Contains(t, prompt, "Doe")
	assert.Contains(t, prompt, "en")
	assert.Contains(t, prompt, "no")
	assert.Contains(t, prompt, "Has profile photo: no")
}

func TestBuildUserPromptEmptyFields(t *testing.T) {
	info := &UserInfo{
		UserID:    1,
		FirstName: "Test",
	}
	prompt := buildUserPrompt(info)
	assert.Contains(t, prompt, "(none)")
	assert.Contains(t, prompt, "(unknown)")
}

func TestBuildUserPromptWithPhoto(t *testing.T) {
	info := &UserInfo{
		UserID:           42,
		Username:         "johndoe",
		FirstName:        "John",
		LanguageCode:     "en",
		ProfilePhotoData: []byte("fake-photo-data"),
	}
	prompt := buildUserPrompt(info)
	assert.Contains(t, prompt, "Has profile photo: yes (see attached image)")
}

func TestBuildSystemPrompt(t *testing.T) {
	t.Run("without photo", func(t *testing.T) {
		prompt := buildSystemPrompt(false)
		assert.Contains(t, prompt, "no profile photo")
		assert.NotContains(t, prompt, "Profile photo analysis")
	})

	t.Run("with photo", func(t *testing.T) {
		prompt := buildSystemPrompt(true)
		assert.Contains(t, prompt, "Profile photo analysis")
		assert.NotContains(t, prompt, "no profile photo")
	})
}

func TestAssessRiskWithPhoto(t *testing.T) {
	t.Run("sends multimodal content when photo provided", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req chatRequest
			json.NewDecoder(r.Body).Decode(&req)
			assert.Len(t, req.Messages, 2)

			userMsg := req.Messages[1]
			content, ok := userMsg.Content.([]interface{})
			require.True(t, ok, "expected content to be an array for multimodal")
			assert.Len(t, content, 2)

			textPart, ok := content[0].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "text", textPart["type"])

			imgPart, ok := content[1].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "image_url", imgPart["type"])
			imgURL, ok := imgPart["image_url"].(map[string]interface{})
			require.True(t, ok)
			assert.Contains(t, imgURL["url"], "data:image/jpeg;base64,")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{{Message: struct {
					Content string `json:"content"`
				}{Content: `{"question_count": 6}`}}},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		info := &UserInfo{
			UserID:           12345,
			Username:         "testuser",
			FirstName:        "Test",
			ProfilePhotoData: []byte("fake-photo-bytes"),
		}

		n, err := c.AssessRisk(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, 6, n)
	})

	t.Run("sends text-only content when no photo", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req chatRequest
			json.NewDecoder(r.Body).Decode(&req)
			assert.Len(t, req.Messages, 2)

			userMsg := req.Messages[1]
			_, isString := userMsg.Content.(string)
			assert.True(t, isString, "expected content to be a string for text-only")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{{Message: struct {
					Content string `json:"content"`
				}{Content: `{"question_count": 3}`}}},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key", "test-model", 5)
		info := &UserInfo{
			UserID:       12345,
			Username:     "testuser",
			FirstName:    "Test",
			LanguageCode: "en",
		}

		n, err := c.AssessRisk(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, 3, n)
	})
}

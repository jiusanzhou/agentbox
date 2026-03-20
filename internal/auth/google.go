package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.zoe.im/agentbox/internal/model"
)

// GoogleConfig holds Google OAuth settings.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// HandleGoogleLogin redirects to Google's authorization URL with CSRF state.
func (a *Auth) HandleGoogleLogin(cfg GoogleConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate random state for CSRF protection
		stateBytes := make([]byte, 16)
		rand.Read(stateBytes)
		state := hex.EncodeToString(stateBytes)

		// Store state in a secure httponly cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "abox_oauth_state",
			Value:    state,
			Path:     "/",
			MaxAge:   600, // 10 minutes
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   r.TLS != nil,
		})

		u := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
			url.QueryEscape(cfg.ClientID),
			url.QueryEscape(cfg.CallbackURL),
			url.QueryEscape("openid email profile"),
			url.QueryEscape(state),
		)
		http.Redirect(w, r, u, http.StatusTemporaryRedirect)
	}
}

// HandleGoogleCallback exchanges the code for a token, fetches user info, and returns JWT.
func (a *Auth) HandleGoogleCallback(cfg GoogleConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify CSRF state
		stateCookie, err := r.Cookie("abox_oauth_state")
		if err != nil || stateCookie.Value == "" {
			http.Error(w, `{"error":"missing oauth state"}`, http.StatusForbidden)
			return
		}
		stateParam := r.URL.Query().Get("state")
		if stateParam == "" || stateParam != stateCookie.Value {
			http.Error(w, `{"error":"oauth state mismatch"}`, http.StatusForbidden)
			return
		}

		// Clear the state cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "abox_oauth_state",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, `{"error":"missing code"}`, http.StatusBadRequest)
			return
		}

		// Exchange code for access token
		accessToken, err := exchangeGoogleCode(cfg, code)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
			return
		}

		// Get Google user profile
		gUser, err := getGoogleUser(accessToken)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
			return
		}

		ctx := r.Context()

		// Try to find existing user by email
		user, err := a.store.GetUserByEmail(ctx, gUser.Email)
		if err != nil {
			// Create new user
			user = &model.User{
				ID:        generateID(),
				Email:     gUser.Email,
				Name:      gUser.Name,
				Avatar:    gUser.Picture,
				GoogleID:  gUser.ID,
				Plan:      "free",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := a.store.CreateUser(ctx, user); err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
				return
			}
		} else {
			// Link Google ID if not set
			if user.GoogleID == "" {
				user.GoogleID = gUser.ID
				user.UpdatedAt = time.Now()
				if user.Avatar == "" && gUser.Picture != "" {
					user.Avatar = gUser.Picture
				}
				_ = a.store.UpdateUser(ctx, user)
			}
		}

		// Generate JWT
		token, err := a.generateJWT(user)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token": token,
			"user":  user,
		})
	}
}

type googleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

func exchangeGoogleCode(cfg GoogleConfig, code string) (string, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"redirect_uri":  {cfg.CallbackURL},
		"grant_type":    {"authorization_code"},
	}

	req, _ := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("google oauth: %s", result.Error)
	}
	return result.AccessToken, nil
}

func getGoogleUser(token string) (*googleUser, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var u googleUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	if u.Email == "" {
		return nil, fmt.Errorf("google account has no email")
	}

	return &u, nil
}

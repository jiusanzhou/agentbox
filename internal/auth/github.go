package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.zoe.im/agentbox/internal/model"
)

// GitHubConfig holds GitHub OAuth settings.
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// HandleGitHubLogin redirects to GitHub's authorization URL.
func (a *Auth) HandleGitHubLogin(cfg GitHubConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email",
			url.QueryEscape(cfg.ClientID),
			url.QueryEscape(cfg.CallbackURL),
		)
		http.Redirect(w, r, u, http.StatusTemporaryRedirect)
	}
}

// HandleGitHubCallback exchanges the code for a token, fetches user info, and returns JWT.
func (a *Auth) HandleGitHubCallback(cfg GitHubConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, `{"error":"missing code"}`, http.StatusBadRequest)
			return
		}

		// Exchange code for access token
		accessToken, err := exchangeGitHubCode(cfg, code)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
			return
		}

		// Get GitHub user profile
		ghUser, err := getGitHubUser(accessToken)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
			return
		}

		ctx := r.Context()

		// Try to find existing user by email
		user, err := a.store.GetUserByEmail(ctx, ghUser.Email)
		if err != nil {
			// Create new user
			user = &model.User{
				ID:        generateID(),
				Email:     ghUser.Email,
				Name:      ghUser.Name,
				Avatar:    ghUser.Avatar,
				GitHubID:  ghUser.ID,
				Plan:      "free",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if user.Name == "" {
				user.Name = ghUser.Login
			}
			if err := a.store.CreateUser(ctx, user); err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
				return
			}
		} else {
			// Link GitHub ID if not set
			if user.GitHubID == "" {
				user.GitHubID = ghUser.ID
				user.UpdatedAt = time.Now()
				if user.Avatar == "" && ghUser.Avatar != "" {
					user.Avatar = ghUser.Avatar
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

type githubUser struct {
	ID     string `json:"id"`
	Login  string `json:"login"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar_url"`
}

func exchangeGitHubCode(cfg GitHubConfig, code string) (string, error) {
	data := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
	}

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
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
		return "", fmt.Errorf("github oauth: %s", result.Error)
	}
	return result.AccessToken, nil
}

func getGitHubUser(token string) (*githubUser, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}

	// If email is empty, fetch from /user/emails
	if u.Email == "" {
		u.Email, _ = getGitHubPrimaryEmail(token)
	}
	if u.Email == "" {
		return nil, fmt.Errorf("github account has no email")
	}

	// GitHub returns numeric ID, convert to string
	if u.ID == "" {
		// The id field from GitHub API is a number, re-parse
		var raw map[string]interface{}
		json.Unmarshal(body, &raw)
		if id, ok := raw["id"].(float64); ok {
			u.ID = fmt.Sprintf("%.0f", id)
		}
	}

	return &u, nil
}

func getGitHubPrimaryEmail(token string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", nil
}

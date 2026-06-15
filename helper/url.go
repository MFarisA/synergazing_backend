package helper

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// GetFrontendURL returns the frontend URL for redirects with fallback logic
func GetFrontendURL() string {
	// First try to get FRONTEND_URL from environment
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL != "" {
		if strings.Contains(frontendURL, ",") {
			parts := strings.Split(frontendURL, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
		return frontendURL
	}

	// Fallback: try to get CLIENT_URL from environment
	frontendURL = os.Getenv("CLIENT_URL")
	if frontendURL != "" {
		return frontendURL
	}

	// Default fallback - assumes frontend runs on port 3000
	backendURL := os.Getenv("APP_URL")
	if backendURL != "" {
		// Parse backend URL and change port to 3000
		if parsedURL, err := url.Parse(backendURL); err == nil {
			parsedURL.Host = fmt.Sprintf("%s:3000", parsedURL.Hostname())
			return parsedURL.String()
		}
	}

	// Final fallback
	return "http://localhost:3000"
}

// BuildOAuthSuccessURL builds the OAuth success redirect URL with query parameters
func BuildOAuthSuccessURL(token string, userID uint, userName, userEmail string) string {
	frontendURL := GetFrontendURL()
	return fmt.Sprintf("%s/callback?success=true&token=%s&user_id=%d&user_name=%s&user_email=%s",
		frontendURL,
		url.QueryEscape(token),
		userID,
		url.QueryEscape(userName),
		url.QueryEscape(userEmail))
}

// BuildOAuthErrorURL builds the OAuth error redirect URL with error type
func BuildOAuthErrorURL(errorType string) string {
	frontendURL := GetFrontendURL()
	return fmt.Sprintf("%s/callback?error=%s", frontendURL, url.QueryEscape(errorType))
}

// BuildOAuthErrorURLWithDescription builds the OAuth error redirect URL with error type and description
func BuildOAuthErrorURLWithDescription(errorType, description string) string {
	frontendURL := GetFrontendURL()
	return fmt.Sprintf("%s/callback?error=%s&error_description=%s",
		frontendURL,
		url.QueryEscape(errorType),
		url.QueryEscape(description))
}

// ValidateURL validates if a given string is a valid URL
func ValidateURL(urlString string) error {
	_, err := url.Parse(urlString)
	return err
}

// IsValidHTTPURL checks if a URL is a valid HTTP/HTTPS URL
func IsValidHTTPURL(urlString string) bool {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false
	}
	return parsedURL.Scheme == "http" || parsedURL.Scheme == "https"
}

package helper

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

func SendPasswordResetEmail(email, token string) {
	emailHost := os.Getenv("EMAIL_HOST")
	emailPortStr := os.Getenv("EMAIL_PORT")
	emailUser := os.Getenv("EMAIL_USERNAME")
	emailPass := os.Getenv("EMAIL_PASSWORD")
	frontendURL := GetFrontendURL()

	emailPort, err := strconv.Atoi(emailPortStr)
	if err != nil {
		log.Printf("Error: Could not parse EMAIL_PORT from .env file: %v", err)
		return
	}

	m := gomail.NewMessage()

	m.SetHeader("From", fmt.Sprintf("Synergazing <%s>", emailUser))
	m.SetHeader("To", email)
	m.SetHeader("Subject", "Password Reset Request")

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, token)

	htmlBody := fmt.Sprintf(`
	<div style="font-family: Arial, sans-serif; line-height: 1.6;">
		<h2>Password Reset Request</h2>
		<p>Hi,</p>
		<p>We received a request to reset your password. Please click the button below to set a new password:</p>
		<a href="%s" target="_blank" style="background-color: #007bff; color: white; padding: 10px 15px; text-decoration: none; border-radius: 5px; display: inline-block;">Reset Password</a>
		<p style="margin-top: 20px;">If the button doesn't work, you can copy and paste this link into your browser:</p>
		<p><a href="%s" target="_blank">%s</a></p>
		<p>This link will expire in 5 minutes.</p>
		<p>If you did not request a password reset, please ignore this email.</p>
		<br>
		<p>Thanks,</p>
		<p>The Synergazing Team</p>
	</div>
	`, resetURL, resetURL, resetURL)

	plainBody := fmt.Sprintf(
		"Password Reset Request\n\n"+
			"Hi,\n\n"+
			"We received a request to reset your password. Please use the following link to set a new password:\n%s\n\n"+
			"This link will expire in 5 minutes.\n\n"+
			"If you did not request a password reset, please ignore this email.\n\n"+
			"Thanks,\nThe Synergazing Team", resetURL)

	m.SetBody("text/html", htmlBody)
	m.AddAlternative("text/plain", plainBody)

	d := gomail.NewDialer(emailHost, emailPort, emailUser, emailPass)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Could not send password reset email to %s: %v", email, err)
	} else {
		log.Printf("Password reset email sent successfully to %s", email)
	}
}

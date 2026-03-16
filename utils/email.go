package utils

import (
	"fmt"
	"net/smtp"
	"os"
)

// SendEmail sends an email via SMTP using environment variables for configuration.
// Returns an error if SMTP is not configured or sending fails.
func SendEmail(to, subject, body string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")

	if host == "" || port == "" || user == "" || pass == "" || from == "" {
		return fmt.Errorf("SMTP not configured: missing one or more SMTP_* env vars")
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	auth := smtp.PlainAuth("", user, pass, host)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		from, to, subject, body)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

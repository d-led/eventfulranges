package main

import "strings"

// adminList gates the admin area. The emails are read from the ADMIN_EMAILS
// environment variable (a Fly secret) as a comma-separated list.
type adminList struct {
	emails map[string]bool
}

// parseAdminList turns a comma-separated list of emails into a lookup set.
// Entries are trimmed and lower-cased, and empty entries are ignored.
func parseAdminList(raw string) *adminList {
	list := &adminList{emails: make(map[string]bool)}
	for _, part := range strings.Split(raw, ",") {
		if email := strings.ToLower(strings.TrimSpace(part)); email != "" {
			list.emails[email] = true
		}
	}
	return list
}

// IsAdmin reports whether email is on the admin list.
func (a *adminList) IsAdmin(email string) bool {
	return a.emails[strings.ToLower(strings.TrimSpace(email))]
}

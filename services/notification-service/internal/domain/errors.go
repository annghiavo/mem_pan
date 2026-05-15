package domain

import "errors"

var (
	ErrTokenNotFound    = errors.New("device token not found")
	ErrForbidden        = errors.New("forbidden")
	ErrTemplateNotFound = errors.New("email template not found")
	ErrAdminRequired    = errors.New("admin access required")
)

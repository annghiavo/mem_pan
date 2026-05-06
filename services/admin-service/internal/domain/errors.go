package domain

import "errors"

var (
	ErrReportNotFound = errors.New("report not found")
	ErrForbidden      = errors.New("access denied")
	ErrAdminRequired  = errors.New("admin access required")
)

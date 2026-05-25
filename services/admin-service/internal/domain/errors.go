package domain

import "errors"

var (
	ErrReportNotFound       = errors.New("report not found")
	ErrForbidden            = errors.New("access denied")
	ErrAdminRequired        = errors.New("admin access required")
	ErrAppealNotFound       = errors.New("appeal not found")
	ErrAppealAlreadyExists  = errors.New("appeal already exists for deck")
	ErrAppealNotSubmittable = errors.New("appeal is not in a state that can accept this action")
	ErrInvalidAppealAction  = errors.New("invalid appeal action")
)

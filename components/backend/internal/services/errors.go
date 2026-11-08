package services

import "errors"

var ErrExportLimitExceeded = errors.New("export limit exceeded")
var ErrYearClosed = errors.New("graduation year closed")

package services

import "errors"

var ErrExportLimitExceeded = errors.New("export limit exceeded")
var ErrYearClosed = errors.New("graduation year closed")
var ErrRefKampusNotFound = errors.New("ref kampus not found")
var ErrSiswaNotInRoster = errors.New("siswa not in roster")
var ErrInvalidFilter = errors.New("invalid filter")
var ErrPTAnnouncementRequired = errors.New("pt announcement required")
var ErrPTAnnouncementMismatch = errors.New("pt announcement mismatch")

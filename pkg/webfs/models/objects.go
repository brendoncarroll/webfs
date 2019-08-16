package models

import (
	"time"
)

type Snapshot struct {
	Cell      VolumeSpec `json:"cell"`
	Commit    Commit     `json:"contents"`
	Timestamp time.Time  `json:"timestamp"`
}

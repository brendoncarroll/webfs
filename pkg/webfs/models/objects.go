package models

import (
	"time"
)

type Snapshot struct {
	Cell      CellSpec  `json:"cell"`
	Contents  Volume    `json:"contents"`
	Timestamp time.Time `json:"timestamp"`
}

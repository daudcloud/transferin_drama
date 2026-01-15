package models

import "time"

// TransactionPending represents pending VIP activation request
// models/transaction_pending.go
type TransactionPending struct {
	TransactionID string    `bson:"transactionID"`
	TelegramID    int64     `bson:"telegramID"`
	Duration      int       `bson:"duration"` // dalam bulan
	UpdatedAt     time.Time `bson:"updated_at,omitempty"`
}

// TransactionSuccess represents a completed/activated VIP
type TransactionSuccess struct {
	TransactionID string    `bson:"transactionID"`
	TelegramID    int64     `bson:"telegramID"`
	ActivatedAt   time.Time `bson:"activatedAt"`
	Duration      int       `bson:"duration"`
}

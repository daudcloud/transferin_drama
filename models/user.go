package models

import "time"

type User struct {
	TelegramName     string     `bson:"telegram_name"`
	TelegramUsername string     `bson:"telegram_username"`
	TelegramUserID   int64      `bson:"telegram_user_id"`
	IsVIP            bool       `bson:"is_vip"`
	ExpireTime       *time.Time `bson:"expire_time,omitempty"`
	DailyLimit       int        `bson:"daily_limit"`
	LastAccess       time.Time  `bson:"last_access"`
	CreatedAt        time.Time  `bson:"created_at"`
	Code             string     `bson:"code"`
	ReferralCode     string     `bson:"referral_code,omitempty"`
}

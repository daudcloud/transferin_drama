package utils

import (
	"crypto/rand"
	"fmt"
	"time"
)

func GetJakartaTime() time.Time {
	// RDP (US) time is currently 15 hours 11 minutes behind Jakarta
	offset := 15*time.Hour + 11*time.Minute
	return time.Now().Add(offset)
}

func GenerateUniqueCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func SameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func GetGreeting() string {
	now := GetJakartaTime()
	hour := now.Hour()
	switch {
	case hour >= 5 && hour < 11:
		return "Selamat pagi"
	case hour >= 11 && hour < 15:
		return "Selamat siang"
	case hour >= 15 && hour < 18:
		return "Selamat sore"
	default:
		return "Selamat malam"
	}
}

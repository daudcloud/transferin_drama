package utils

import (
	"context"
	"fmt"
	"log"
	"strings"
	"transferin-drama/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gopkg.in/telebot.v3"
)

var nextBtn telebot.Btn
var prevBtn telebot.Btn
var ctx = context.Background()

func SendVideo(c telebot.Context, slug string, db *mongo.Database) error {
	user := c.Sender()
	videoCollection := db.Collection("videos")
	userCollection := db.Collection("users")
	var existingUser models.User
	filter := bson.M{"telegram_user_id": user.ID}
	now := GetJakartaTime()
	err := userCollection.FindOne(ctx, filter).Decode(&existingUser)
	if err != nil {
		newUser := models.User{
			TelegramName:     strings.TrimSpace(user.FirstName + " " + user.LastName),
			TelegramUsername: user.Username,
			TelegramUserID:   user.ID,
			IsVIP:            false,
			DailyLimit:       10,
			LastAccess:       now,
			CreatedAt:        now,
			Code:             GenerateUniqueCode(),
		}
		_, err = userCollection.InsertOne(ctx, newUser)
		if err != nil {
			log.Println("❌ Failed to insert user:", err)
		} else {
			log.Printf("✅ New user saved: %s (%s)", newUser.TelegramName, newUser.TelegramUsername)
		}
		existingUser = newUser
	}

	if !SameDay(existingUser.LastAccess, now) {
		existingUser.DailyLimit = 10
	}
	if existingUser.ExpireTime != nil && existingUser.ExpireTime.Before(now) {
		_, err := userCollection.UpdateOne(
			ctx,
			bson.M{"telegram_user_id": user.ID},
			bson.M{"$set": bson.M{"expire_time": nil, "is_vip": false}},
		)
		if err != nil {
			return c.Send("❌ Ada kesalahan sistem.")
		}
		existingUser.IsVIP = false
	}

	partMenu := &telebot.ReplyMarkup{}
	var video models.Video

	err = videoCollection.FindOne(ctx, bson.M{"slug": slug}).Decode(&video)
	if err != nil {
		log.Print(err)
		return c.Send("❌ Video tidak ditemukan.")
	}

	partInt := video.Part
	nextPart := partInt + 1
	prevPart := partInt - 1
	nextBtn = partMenu.Data("Next Part", "next_part", fmt.Sprintf("%s|%d", slug, nextPart))
	prevBtn = partMenu.Data("Previous Part", "prev_part", fmt.Sprintf("%s|%d", slug, prevPart))

	// VIP check
	if video.VIPOnly && !existingUser.IsVIP {
		return c.Send("🔐 Video ini hanya bisa diakses oleh pengguna VIP.\n\nKetik /vip untuk upgrade.")
	}

	if !existingUser.IsVIP {
		if existingUser.DailyLimit <= 0 {
			return c.Send("❌ Batas harian kamu sudah habis. Silakan tunggu besok atau upgrade VIP.")
		}
		// Decrease limit
		existingUser.DailyLimit--
		existingUser.LastAccess = now

		// Save changes
		_, err := userCollection.UpdateOne(ctx,
			bson.M{"telegram_user_id": user.ID},
			bson.M{
				"$set": bson.M{
					"daily_limit": existingUser.DailyLimit,
					"last_access": existingUser.LastAccess,
				},
			},
		)
		if err != nil {
			log.Println("❌ Failed to update user limit:", err)
		}
	}

	if video.Part == 1 {
		partMenu.Inline(partMenu.Row(nextBtn))
	} else if video.Part == video.TotalPart {
		partMenu.Inline(partMenu.Row(prevBtn))
	} else {
		partMenu.Inline(partMenu.Row(prevBtn, nextBtn))
	}

	opts := &telebot.SendOptions{
		Protected: true,
	}

	// 1️⃣ Check Redis for FileID
	// fileIDKey := "fileid:" + slug
	// cachedFileID, err := redisClient.Get(ctx, fileIDKey).Result()
	// if err == nil && cachedFileID != "" {
	// 	log.Println("📥 Using cached FileID from Redis:", cachedFileID)
	// 	videoToSend := &telebot.Video{
	// 		File:    telebot.File{FileID: cachedFileID},
	// 		Caption: fmt.Sprintf("🎞️ <b>%s</b>\n\nDipersembahkan oleh DRAMATRANS", video.Title),
	// 	}
	// 	return c.Send(videoToSend, opts, partMenu, telebot.ModeHTML)
	// }

	// 2️⃣ If Redis missed, check MongoDB for backup
	if video.FileID != "" {
		log.Println("📥 Using backup FileID from DB:", video.FileID)
		videoToSend := &telebot.Video{
			File:    telebot.File{FileID: video.FileID},
			Caption: fmt.Sprintf("🎞️ <b>%s</b>\n\nDipersembahkan oleh DRAMATRANS", video.Title),
		}
		// Refresh Redis cache
		// _ = redisClient.Set(ctx, fileIDKey, video.FileID, 30*24*time.Hour).Err()
		return c.Send(videoToSend, opts, partMenu, telebot.ModeHTML)
	}

	return nil
}

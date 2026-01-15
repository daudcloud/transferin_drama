package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"transferin-drama/models"
	"transferin-drama/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gopkg.in/telebot.v3"
)

var (
	db  *mongo.Database
	ctx = context.Background()
)

func HandleStart(c telebot.Context) error {
	now := utils.GetJakartaTime()
	slug := c.Data()
	user := c.Sender()

	if slug != "" {
		utils.SendVideo(c, slug, db)
		return nil
	}

	userCollection := db.Collection("users")
	var existingUser models.User
	filter := bson.M{"telegram_user_id": user.ID}

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
			Code:             utils.GenerateUniqueCode(),
		}
		_, err := userCollection.InsertOne(ctx, newUser)
		if err != nil {
			log.Println("❌ Failed to insert user:", err)
		} else {
			log.Printf("✅ New user saved: %s (%s)", newUser.TelegramName, newUser.TelegramUsername)
		}
	}

	welcome := fmt.Sprintf(
		"%s, %s! 👋\n\n"+
			"Lagi cari tempat nonton drama China/Korea/Barat yang simpel?\n"+
			"<b>DRAMATRANS</b> jawabannya!\n\n"+
			"Langsung tonton dari Telegram tanpa harus buka aplikasi lain.\n"+
			"Klik tombol di bawah buat mulai 🎥\n\n"+
			"🎁 <b>Program Referral VIP Gratis!</b>\n"+
			"Undang temanmu untuk join DRAMATRANS pakai kode referral kamu (lihat di /status) dan dapatkan VIP GRATIS sesuai durasi VIP yang dibeli temanmu — hingga <b>maksimal 3 hari</b>! 💎\n"+
			"Temanmu juga akan dapat <b>bonus durasi 100%%</b> dari paket VIP yang mereka beli (maksimal 7 hari). 🤝\n\n"+
			"🔑 Mau pakai referral code dari teman? Ketik <b>/referral</b> lalu masukkan kodenya.\n\n"+
			"Kalau ada yang mau ditanya, chat aja admin @domi_nuc ya!",
		utils.GetGreeting(), user.FirstName,
	)

	return c.Send(welcome, telebot.ModeHTML)
}

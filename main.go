package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/telebot.v3"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"transferin-drama/models"
)

var redisClient *redis.Client
var (
	bot *telebot.Bot
	db  *mongo.Database
	ctx = context.Background()
)

var qrLink = map[string]int{
	"vip_1d":  2000,
	"vip_3d":  4000,
	"vip_7d":  9000,
	"vip_30d": 25000,
}

var packageDuration = map[string]int{
	"vip_1d":  1,
	"vip_3d":  3,
	"vip_7d":  7,
	"vip_30d": 30,
}

var menu = &telebot.ReplyMarkup{}
var startBtn = menu.Data("🎬 Mulai Nonton", "start_watch")
var vipBtn = menu.Data("💎 Langganan VIP 🔐", "vip")
var statusBtn = menu.Data("ℹ️ Cek Status Akun", "cek_status")
var backBtn = menu.Data("🔙 Kembali", "back_to_start")
var cancelBtn telebot.Btn
var nextBtn telebot.Btn
var prevBtn telebot.Btn
var parentDir = `home/melolo`

type Package struct {
	Days   int
	Amount int
}

type APIResponse struct {
	Payment struct {
		PaymentNumber string `json:"payment_number"`
		ExpiredAt     string `json:"expired_at"`
	} `json:"payment"`
}

type CancelResponse struct {
	Success bool `json:"success"`
}

const (
	spreadsheetID = "1IRnsdu7xEWL3kPApxHBwlew8uf2YmjPc1lcZk6U58ng"
	sheetName     = "sheet1"
)

func GetJakartaTime() time.Time {
	// RDP (US) time is currently 15 hours 11 minutes behind Jakarta
	offset := 0 * time.Hour
	return time.Now().Add(offset)
}

func getPackages(amount int) (map[int]int, int) {
	packages := []Package{
		{30, 25000},
		{7, 9000},
		{3, 4000},
		{1, 2000},
	}

	result := make(map[int]int)
	totalDays := 0

	for _, pkg := range packages {
		count := amount / pkg.Amount
		if count > 0 {
			result[pkg.Days] = count
			totalDays += count * pkg.Days
			amount -= count * pkg.Amount
		}
	}

	return result, totalDays
}

func generatePost(c telebot.Context, strTitle string, title string, totalParts int) error {
	// Title is everything except last argument
	slug := slugify(title)

	var post strings.Builder
	post.WriteString("⧐⧐DRAMATRANS⧏⧏\n")
	post.WriteString(fmt.Sprintf("<b>⭐ JUDUL: %s ⭐</b>\n", strings.ToUpper(strTitle)))
	post.WriteString("━━━━━━━━━━━━━━━━━\n")

	for i := 1; i < totalParts; i++ {
		if i == 1 {
			post.WriteString(fmt.Sprintf("➠ PART %d → (Gratis Nonton) 🆓 <a href=\"https://t.me/dramatrans_bot?start=%s_part_%d\">Tonton sekarang</a>\n", i, slug, i))
		} else {
			post.WriteString(fmt.Sprintf("➠ PART %d → (Berlangganan VIP) 🔒 <a href=\"https://t.me/dramatrans_bot?start=%s_part_%d\">Tonton sekarang</a>\n", i, slug, i))
		}
	}

	post.WriteString("\n🔥 <b>LANGGANAN VIP</b> 🔥\n")
	post.WriteString("Mulai Rp2.000 / Hari kamu bisa menonton semua drama bebas full episode.\n")
	post.WriteString("🫰 Mulai langganan : <a href=\"https://t.me/dramatrans_bot\">Beli VIP melalui bot</a> 🫰\n\n")

	post.WriteString("⚡ <b>PERHATIAN</b> ⚡\n")
	post.WriteString("Perhatikan panduan di bawah untuk berlangganan VIP\n")
	post.WriteString("<a href=\"https://t.me/dramatranspanduan\">Cara berlangganan VIP</a>\n\n")

	post.WriteString("🔗 <b>BANTUAN</b>\n")
	post.WriteString("Hubungi admin kami jika ada pertanyaan seputar DramaTrans\n")
	post.WriteString("☎️<a href=\"https://t.me/domi_nuc\">Admin</a>\n")

	photo := &telebot.Photo{
		File:    telebot.FromDisk(fmt.Sprintf(`%s/%s/%s`, parentDir, title, fmt.Sprintf("cover_%s.jpg", slug))),
		Caption: post.String(),
	}

	return c.Send(photo, telebot.ModeHTML)
}

func handleVIP(c telebot.Context) error {
	user := c.Sender()
	userCollection := db.Collection("users")

	var u models.User
	err := userCollection.FindOne(ctx, bson.M{"telegram_user_id": user.ID}).Decode(&u)
	if err != nil {
		log.Println("❌ Gagal mengambil data user:", err)
		return c.Send("Terjadi kesalahan. Silakan coba lagi nanti.")
	}

	// Tombol pilihan paket VIP
	vipMenu := &telebot.ReplyMarkup{}
	vip1 := vipMenu.Data("🎟️ VIP 1 Hari – Coba Dulu 💰Rp2.000", "vip_1d")
	vip3 := vipMenu.Data("🌟 VIP 3 Hari – Penonton Setia 💰Rp4.000", "vip_3d")
	vip7 := vipMenu.Data("🎬 VIP 7 Hari – Pecinta Drama 💰Rp9.000", "vip_7d")
	vip30 := vipMenu.Data("👑 VIP 30 Hari – Sultan Drama 💰Rp25.000", "vip_30d")
	vipMenu.Inline(
		vipMenu.Row(vip1),
		vipMenu.Row(vip3),
		vipMenu.Row(vip7),
		vipMenu.Row(vip30),
		vipMenu.Row(backBtn),
	)

	now := GetJakartaTime()
	if u.IsVIP && u.ExpireTime != nil && u.ExpireTime.After(now) {
		remaining := u.ExpireTime.Sub(now).Round(time.Hour)
		hours := int(remaining.Hours())

		msg := fmt.Sprintf(
			"✨ <b>STATUS VIP AKTIF</b> ✨\n\n"+
				"⏳ Masa aktif VIP kamu tersisa: <b>%d jam</b>\n\n"+
				"Terima kasih sudah jadi bagian dari <b>DRAMATRANS VIP</b> — komunitas eksklusif pecinta drama berkualitas!\n"+
				"Kamu sekarang punya akses ke <b>semua part drama</b>, termasuk yang terkunci.\n\n"+
				"🔁 Mau lanjut langganan setelah masa aktif habis?\n"+
				"Pantau terus karena akan ada <b>promo spesial</b> hanya untuk member aktif 💝\n\n"+
				"📬 Butuh bantuan? Hubungi admin:\n👉 <a href='https://t.me/domi_nuc'>@domi_nuc</a>",
			hours,
		)
		if c.Callback() != nil {
			return c.Edit(msg, vipMenu, telebot.ModeHTML)
		}
		return c.Send(msg, vipMenu, telebot.ModeHTML)
	}

	msg := "🚫 <b>AKSES VIP BELUM AKTIF</b> 🚫\n\n" +
		"<b>Langkah penting sebelum membeli:</b>\n\n" +
		"▶️ Silahkan memilih paket yang tersedia\n" +
		"▶️ Paket akan langsung aktif setelah berhasil melakukan pembayaran\n\n" +
		"👤 <b>Perlu bantuan langsung?</b>\nHubungi admin:\n📩 <a href='https://t.me/domi_nuc'>@domi_nuc</a>"

	if c.Callback() != nil {
		return c.Edit(msg, vipMenu, telebot.ModeHTML)
	}
	return c.Send(msg, vipMenu, telebot.ModeHTML)
}

func handleStatus(c telebot.Context) error {
	user := c.Sender()
	userCollection := db.Collection("users")

	var u models.User
	err := userCollection.FindOne(ctx, bson.M{"telegram_user_id": user.ID}).Decode(&u)
	if err != nil {
		log.Println("❌ Gagal mengambil data user:", err)
		return c.Send("Terjadi kesalahan saat memuat status akun kamu. Coba beberapa saat lagi.")
	}

	now := GetJakartaTime()
	if u.IsVIP && u.ExpireTime != nil && u.ExpireTime.After(now) {
		expTime := u.ExpireTime.Format("2 January 2006 - 15:04")
		msg := fmt.Sprintf(
			"👤 <b>Status Akun Kamu</b>\n\n"+
				"🆔 User ID: <code>%d</code>\n"+
				"📛 Nama: %s\n"+
				"👑 VIP: <b>AKTIF</b> (hingga %s WIB)\n"+
				"🎁 Referral: <code>%s</code>\n"+
				"📊 Limit Harian: <i>Tidak Berlaku</i>\n\n"+
				"🎉 Kamu adalah <b>Member VIP</b>!\nNikmati semua konten tanpa batas setiap hari. Terima kasih sudah mendukung <b>DRAMATRANS</b>! 💖",
			u.TelegramUserID,
			u.TelegramName,
			expTime,
			u.Code,
		)

		reply := &telebot.ReplyMarkup{}
		reply.Inline(reply.Row(backBtn))
		if c.Callback() != nil {
			return c.Edit(msg, reply, telebot.ModeHTML)
		}
		return c.Send(msg, reply, telebot.ModeHTML)
	}

	if u.ExpireTime != nil && u.ExpireTime.Before(now) {
		_, err = userCollection.UpdateOne(
			ctx,
			bson.M{"telegram_user_id": user.ID},
			bson.M{"$set": bson.M{"expire_time": nil, "is_vip": false}},
		)
		if err != nil {
			return c.Send("❌ Ada kesalahan sistem.")
		}
	}

	msg := fmt.Sprintf(
		"👤 <b>Status Akun Kamu</b>\n\n"+
			"🆔 User ID: <code>%d</code>\n"+
			"📛 Nama: %s\n"+
			"👑 VIP: <b>Tidak Aktif</b>\n"+
			"🎁 Referral: <code>%s</code>\n"+
			"📊 Limit Harian: %d/10\n\n"+
			"🚫 Kamu masih pengguna <b>Gratis</b>. Akses dibatasi maksimal <b>10 part</b> per hari.\n\n"+
			"💎 Ingin akses tanpa batas ke semua konten?\n➡️ Upgrade ke VIP sekarang! Ketik /vip atau klik tombol di bawah.\n\n"+
			"📩 Ada pertanyaan? Chat admin: @domi_nuc",
		u.TelegramUserID,
		u.TelegramName,
		u.Code,
		u.DailyLimit,
	)

	reply := &telebot.ReplyMarkup{}
	reply.Inline(reply.Row(vipBtn), reply.Row(backBtn))

	if c.Callback() != nil {
		return c.Edit(msg, reply, telebot.ModeHTML)
	}
	return c.Send(msg, reply, telebot.ModeHTML)
}

func generateTransactionID() string {
	// Seed the random number generator (important for different IDs each time)
	rand.Seed(time.Now().UnixNano())

	// Get current time components
	now := time.Now()
	year := now.Format("2006") // YYYY
	month := now.Format("01")  // MM (with leading zero)
	day := now.Format("02")    // DD (with leading zero)
	hour := now.Format("15")   // HH (24-hour)
	minute := now.Format("04") // MM
	second := now.Format("05") // SS

	// Generate 3 random digits (000-999)
	// rand.Intn(1000) gives 0 to 999, then format to ensure 3 digits (e.g., "007")
	randomDigits := fmt.Sprintf("%03d", rand.Intn(1000))

	// Combine all parts
	txID := fmt.Sprintf("INV%s%s%s%s%s%s%s", year, month, day, hour, minute, second, randomDigits)

	return txID
}

func sendQris(c telebot.Context, vipCode string) error {

	user := c.Sender()
	userCollection := db.Collection("users")

	var u models.User

	result := userCollection.FindOne(ctx, bson.M{"telegram_user_id": user.ID})
	if err := result.Decode(&u); err != nil {
		log.Println("❌ Gagal mengambil data user:", err)
		return c.Send("Terjadi kesalahan saat memuat status akun kamu. Coba beberapa saat lagi.")
	}

	amount, ok := qrLink[vipCode]
	if !ok {
		return c.Send("❌ Paket VIP tidak ditemukan.")
	}
	url := "https://app.pakasir.com/api/transactioncreate/qris"
	transactionID := generateTransactionID()
	cancelBtn = menu.Data("❌ Batalkan pembayaran", "cancel_payment", fmt.Sprintf("%s|%d", transactionID, amount))
	payload := map[string]interface{}{
		"project":  "drama-trans",
		"order_id": transactionID,
		"amount":   amount,
		"api_key":  os.Getenv("PAKASIR_API_KEY"),
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Read the response body
	var results APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}

	fmt.Println(results)

	content := results.Payment.PaymentNumber
	filename := fmt.Sprintf("qr-%d.png", user.ID)
	err = qrcode.WriteFile(content, qrcode.Medium, 256, filename)

	if err != nil {
		fmt.Printf("Error generating QR code: %v\n", err)
		return nil
	}

	fmt.Println("QR Code saved to:", filename)

	duration, ok := packageDuration[vipCode]
	if !ok {
		duration = 1 // fallback default: 1 bulan
	}

	telegramID := user.ID

	// Masukkan ke collection transactionPending
	pendingCol := db.Collection("transactionPending")
	filter := bson.M{"telegramID": telegramID, "transactionID": transactionID}
	update := bson.M{
		"$set": bson.M{
			"duration": duration,
		},
		"$setOnInsert": bson.M{
			"telegramID":    telegramID,
			"transactionID": transactionID,
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err = pendingCol.UpdateOne(ctx, filter, update, opts)

	if err != nil {
		log.Printf("❌ Gagal menyimpan transaksi pending: %v", err)
		return c.Send("⚠️ Terjadi kesalahan saat menyiapkan transaksi.")
	}

	p := message.NewPrinter(language.Indonesian)

	// Format with the currency symbol
	formatted := p.Sprintf("Rp %d", amount)

	// Kirim pesan ke user

	var msg strings.Builder
	fmt.Println(results)
	if results.Payment.ExpiredAt == "" {
		fmt.Println("Error: expired_at is still empty. Check JSON key names.")
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, results.Payment.ExpiredAt)
	if err != nil {
		fmt.Println("Error parsing time:", err) // This will tell you why it failed
		return nil
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	fmt.Println("Original (UTC):", t.String())

	// 3. Convert and format
	wibTime := t.In(loc)
	displayTime := wibTime.Format("02-01-2006 15:04:05")

	msg.WriteString("💎 <b>Pembayaran Paket VIP (QRIS)</b>\n\n")
	msg.WriteString(fmt.Sprintf("💲 Nominal : %s\n", formatted))
	msg.WriteString(fmt.Sprintf("🔐 Paket VIP : %d hari\n", duration))
	msg.WriteString(fmt.Sprintf("🧾 ID Transaksi : %s\n\n", transactionID))
	msg.WriteString(fmt.Sprintf("✅ Berlaku sampai : %s ✅", displayTime))

	// post := "TEST"

	photo := &telebot.Photo{
		File:    telebot.FromDisk(filename),
		Caption: msg.String(),
	}

	reply := &telebot.ReplyMarkup{}
	reply.Inline(reply.Row(cancelBtn))

	var sentMsg *telebot.Message

	sentMsg, err = c.Bot().Send(c.Chat(), photo, reply, telebot.ModeHTML)

	if err != nil {
		return c.Send("Terjadi kesalahan!")
	}

	lastVideoMessages[telegramID] = sentMsg
	return nil
}

// Webhook handler di file utama bot
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	log.Print("Hitted")
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	amount, _ := payload["amount"].(float64)
	log.Print(amount)
	order_id, _ := payload["order_id"].(string)
	log.Printf("🔔 Received Order ID: %s", order_id)

	ctx := context.Background()
	pendingCol := db.Collection("transactionPending")
	successCol := db.Collection("transactionSuccess")
	userCol := db.Collection("users")

	var pendingTx models.TransactionPending
	err := pendingCol.FindOne(ctx, bson.M{"transactionID": order_id}).Decode(&pendingTx)
	if err != nil {
		log.Printf("❌ Order ID not found in pending transactions: %s", order_id)
		w.WriteHeader(http.StatusOK)
		return
	}

	var payer models.User
	err = userCol.FindOne(ctx, bson.M{"telegram_user_id": pendingTx.TelegramID}).Decode(&payer)
	if err != nil {
		log.Printf("❌ Failed to fetch payer user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, duration := getPackages(int(amount))
	if duration <= 0 {
		duration = 0 // default fallback
	}

	bonusForPayer := 0
	bonusForReferrer := 0
	if payer.ReferralCode != "" {
		// payer gets 100% bonus, max 7
		if duration > 0 {
			bonusForPayer = duration
			if bonusForPayer > 7 {
				bonusForPayer = 7
			}
		}
		// referrer gets same as payer's package, max 3
		bonusForReferrer = duration
		if bonusForReferrer > 3 {
			bonusForReferrer = 3
		}
	}

	finalDuration := duration + bonusForPayer

	// Indonesian timezone
	now := GetJakartaTime()
	var updatedUser models.User
	// Update expire_time without querying first
	err = userCol.FindOneAndUpdate(ctx,
		bson.M{"telegram_user_id": pendingTx.TelegramID},
		mongo.Pipeline{
			{{
				Key: "$set",
				Value: bson.D{
					{
						Key: "expire_time",
						Value: bson.D{
							{
								Key: "$cond",
								Value: bson.A{
									bson.D{
										{Key: "$gt", Value: bson.A{"$expire_time", now}},
									},
									bson.D{
										{Key: "$dateAdd", Value: bson.D{
											{Key: "startDate", Value: "$expire_time"},
											{Key: "unit", Value: "day"},
											{Key: "amount", Value: finalDuration},
										}},
									},
									now.AddDate(0, 0, finalDuration),
								},
							},
						},
					},
					{
						Key:   "is_vip",
						Value: true,
					},
				},
			}},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updatedUser)
	if err != nil {
		log.Printf("❌ Failed to update user VIP: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if payer.ReferralCode != "" && bonusForReferrer > 0 {
		var referrer models.User

		err := userCol.FindOneAndUpdate(ctx,
			bson.M{"code": payer.ReferralCode},
			mongo.Pipeline{
				{{
					Key: "$set",
					Value: bson.D{
						{
							Key: "expire_time",
							Value: bson.D{
								{
									Key: "$cond",
									Value: bson.A{
										bson.D{
											{Key: "$gt", Value: bson.A{"$expire_time", now}},
										},
										bson.D{
											{Key: "$dateAdd", Value: bson.D{
												{Key: "startDate", Value: "$expire_time"},
												{Key: "unit", Value: "day"},
												{Key: "amount", Value: bonusForReferrer},
											}},
										},
										now.AddDate(0, 0, bonusForReferrer),
									},
								},
							},
						},
						{
							Key:   "is_vip",
							Value: true,
						},
					},
				}},
			},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&referrer)
		if err != nil {
			log.Printf("⚠️ Failed to update referrer VIP: %v", err)
		} else {
			// Notify referrer
			recipient := &telebot.User{ID: referrer.TelegramUserID}
			msg := fmt.Sprintf(
				"🎉 Bonus VIP!\n\n"+
					"👤 Teman kamu <b>%s</b> baru berlangganan VIP.\n"+
					"🎁 Kamu dapat tambahan VIP %d Hari (maksimal).\n"+
					"⏰ VIP berlaku sampai: %s WIB.",
				payer.TelegramName,
				bonusForReferrer,
				referrer.ExpireTime.Format("02 January 2006 - 15:04"),
			)
			_, _ = bot.Send(recipient, msg, telebot.ModeHTML)
		}
	}

	// Simpan ke transactionSuccess

	successTx := models.TransactionSuccess{
		TransactionID: pendingTx.TransactionID,
		TelegramID:    pendingTx.TelegramID,
		ActivatedAt:   now,
		Duration:      duration,
	}
	_, err = successCol.InsertOne(ctx, successTx)
	if err != nil {
		log.Printf("❌ Failed to save to transactionSuccess: %v", err)
	}

	// Hapus dari pending
	_, err = pendingCol.DeleteOne(ctx, bson.M{"trasactionID": order_id})
	if err != nil {
		log.Printf("⚠️ Failed to delete from pending: %v", err)
	}

	// Kirim pesan konfirmasi
	recipient := &telebot.User{ID: pendingTx.TelegramID}
	msg := fmt.Sprintf(
		"✨ Hore! VIP kamu sudah aktif! ✨\n\n"+
			"🎁 <b>Paket:</b> Akses VIP %d Hari\n"+
			"⏰ <b>Berlaku sampai:</b> %s WIB\n"+
			"📺 Sekarang kamu bisa nonton semua drama tanpa batas!\n"+
			"❤️ Terima kasih sudah dukung kami di DRAMATRANS!",
		duration,
		updatedUser.ExpireTime.Format("02 January 2006 - 15:04"),
	)

	_, err = bot.Send(recipient, msg, telebot.ModeHTML)
	if err != nil {
		log.Printf("⚠️ Failed to send message to user: %v", err)
	}

	log.Printf("✅ VIP activated for user ID %d with transaction ID %s (duration: %d hari)", pendingTx.TelegramID, order_id, duration)
	w.WriteHeader(http.StatusOK)
}

func sameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

var lastVideoMessages = make(map[int64]*telebot.Message)

func sendVideo(c telebot.Context, slug string) error {
	user := c.Sender()
	chatID := user.ID
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
			Code:             generateUniqueCode(),
		}
		_, err = userCollection.InsertOne(ctx, newUser)
		if err != nil {
			log.Println("❌ Failed to insert user:", err)
		} else {
			log.Printf("✅ New user saved: %s (%s)", newUser.TelegramName, newUser.TelegramUsername)
		}
		existingUser = newUser
	}

	if !sameDay(existingUser.LastAccess, now) {
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

	var sentMsg *telebot.Message

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
		sentMsg, err = c.Bot().Send(c.Chat(), videoToSend, opts, partMenu, telebot.ModeHTML)
	}
	if err != nil {
		return c.Send("Gagal mengirim video")
	}

	lastVideoMessages[chatID] = sentMsg
	return nil
}

func generateUniqueCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func processReferral(c telebot.Context, code string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	code = strings.TrimSpace(code)
	if code == "" {
		return c.Send("❌ Referral code cannot be empty.")
	}

	userCollection := db.Collection("users")

	// Get current user from DB
	var currentUser models.User
	err := userCollection.FindOne(ctx, bson.M{"telegram_user_id": c.Sender().ID}).Decode(&currentUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Send("❌ You are not registered in the system.")
		}
		return c.Send("❌ Error fetching your data.")
	}

	// Check if already set
	if currentUser.ReferralCode != "" {
		return c.Send(fmt.Sprintf("ℹ️ You have already set a referral code : %s", currentUser.ReferralCode))
	}

	// Find the user who owns this code
	var referrer models.User
	err = userCollection.FindOne(ctx, bson.M{"code": code}).Decode(&referrer)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Send("❌ Invalid referral code.")
		}
		return c.Send("❌ Error checking referral code.")
	}

	// Optional: prevent self-referral
	if referrer.TelegramUserID == c.Sender().ID {
		return c.Send("❌ You cannot use your own referral code.")
	}

	// Save the referral code to current user
	update := bson.M{"$set": bson.M{"referral_code": code}}
	_, err = userCollection.UpdateOne(ctx, bson.M{"telegram_user_id": c.Sender().ID}, update)
	if err != nil {
		return c.Send("❌ Failed to save your referral code.")
	}

	// TODO: Add logic here to give rewards, points, etc.

	return c.Send(fmt.Sprintf("✅ Referral code set! You were referred by @%s", referrer.TelegramUsername))
}

// ==== CONFIG ====
// Untuk channel publik: isi username TANPA '@', mis. "DramaTrans"
const requiredChannelID = int64(-1002820138086) // Your channel ID

func MustJoinChannel(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		chat := &telebot.Chat{ID: requiredChannelID}

		member, err := c.Bot().ChatMemberOf(chat, c.Sender())
		if err != nil {
			return c.Send("⚠️ Tidak bisa memeriksa keanggotaan. Pastikan bot sudah menjadi admin di channel.")
		}

		if member.Role == telebot.Left || member.Role == telebot.Kicked {
			kb := &telebot.ReplyMarkup{}
			kb.Inline(kb.Row(
				telebot.Btn{Text: "🔔 Join Channel", URL: "https://t.me/DramaTrans"},
			))
			return c.Send(
				"🚫 Kamu harus join channel dulu untuk menggunakan bot ini.",
				kb,
			)
		}

		return next(c)
	}
}

func slugify(title string) string {
	// lower case, replace spaces with underscore, remove non-alphanum
	re := regexp.MustCompile(`[^a-z0-9_]+`)
	slug := strings.ToLower(strings.ReplaceAll(title, " ", "_"))
	return re.ReplaceAllString(slug, "")
}

func cleanTitle(rawTitle string) string {
	var builder strings.Builder

	for _, r := range rawTitle {
		// Check if alphanumeric or specific allowed characters
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '_' || r == '(' || r == ')' {
			builder.WriteRune(r)
		}
	}

	return strings.TrimSpace(builder.String())
}

func main() {
	_ = godotenv.Load()
	now := GetJakartaTime()

	log.Print(now.Format("02 January 2006 - 15:04"))

	var waitingForReferral = make(map[int64]bool)

	mongoURI := os.Getenv("MONGO_URI")
	dbName := os.Getenv("MONGO_DB")
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("❌ MongoDB connection failed:", err)
	}
	db = client.Database(dbName)
	log.Println("✅ Connected to MongoDB")

	// redisClient = redis.NewClient(&redis.Options{
	// 	Addr:     os.Getenv("REDIS_ADDR"), // example: "localhost:6379"
	// 	Password: "",                      // or from env if set
	// 	DB:       0,
	// })
	// _, err = redisClient.Ping(ctx).Result()
	// if err != nil {
	// 	log.Fatal("❌ Redis connection failed:", err)
	// }
	// log.Println("✅ Connected to Redis")

	pref := telebot.Settings{
		Token: os.Getenv("BOT_TOKEN"),
		Client: &http.Client{
			Timeout: 15 * time.Minute,
		},
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
		// URL:    "http://localhost:8081",
	}
	bot, err = telebot.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}

	bot.Use(func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			ownerIDStr := os.Getenv("BOT_OWNER_ID")
			ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
			if err != nil {
				log.Fatalf("Invalid BOT_OWNER_ID: %v", err)
			}
			if c.Sender().ID == ownerID {
				return next(c)
			}
			if c.Chat().Type != telebot.ChatPrivate {
				// You can also silently ignore:
				// return nil
				return c.Send("❌ This bot only works in direct messages.")
			}
			return next(c) // continue to actual handler
		}
	})

	bot.Use(MustJoinChannel)

	// Buttons
	menu.Inline(menu.Row(startBtn), menu.Row(vipBtn, statusBtn))

	// Handle /start
	bot.Handle("/start", func(c telebot.Context) error {
		slug := c.Data()
		user := c.Sender()

		if slug != "" {
			sendVideo(c, slug)
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
				Code:             generateUniqueCode(),
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
				"Temanmu juga akan dapat <b>bonus durasi 100%%</b> dari paket VIP yang mereka beli (maksimal 7 hari). 🤝\n"+
				"<b>Bonus Referral bisa diuangkan</b> dan akan mendapatkan 20%% dari total durasi, estimasi hitungan per hari sebesar Rp400.\n"+
				"Contoh: total durasi kamu yang didapatkan dari bonus referral sebanyak 10 hari (10 X Rp400 = Rp4000)\n\n"+
				"🔑 Mau pakai referral code dari teman? Ketik <b>/referral</b> lalu masukkan kodenya.\n\n"+
				"Kalau ada yang mau ditanya, chat aja admin @domi_nuc ya!",
			getGreeting(), user.FirstName,
		)

		return c.Send(welcome, menu, telebot.ModeHTML)
	})

	// 🎬 Mulai Nonton
	bot.Handle(&startBtn, func(c telebot.Context) error {
		msg := "🎬 <b>Cara Nonton Drama di DRAMATRANS</b>\n\n" +
			"1️⃣ Buka channel 👉 @Dramatrans\n" +
			"2️⃣ Temukan drama yang kamu mau:\n" +
			"   • Gunakan fitur 🔍 pencarian & ketik judulnya\n" +
			"   • Atau klik profil channel → masuk tab <b>Media</b>\n" +
			"3️⃣ Klik judul atau part → tekan TONTON\n" +
			"4️⃣ Bot akan otomatis kirim videonya ke kamu 🎥\n\n" +
			"🔐 <b>Part VIP</b> hanya untuk yang sudah berlangganan\n" +
			"❓ Ada pertanyaan? Hubungi admin: @domi_nuc"

		reply := &telebot.ReplyMarkup{}
		reply.Inline(reply.Row(backBtn))

		return c.Edit(msg, reply, telebot.ModeHTML)
	})

	// 💎 Langganan VIP
	bot.Handle(&vipBtn, handleVIP)
	bot.Handle("/vip", handleVIP)

	// ℹ️ Cek Status Akun
	bot.Handle(&statusBtn, handleStatus)
	bot.Handle("/status", handleStatus)

	bot.Handle("/referral", func(c telebot.Context) error {
		waitingForReferral[c.Sender().ID] = true
		return c.Send("Please send me your referral code now.")
	})

	bot.Handle("/process", func(c telebot.Context) error {
		ownerID := os.Getenv("BOT_OWNER_ID")
		if fmt.Sprint(c.Sender().ID) != ownerID {
			return c.Send("❌ Kamu tidak punya akses ke perintah ini.")
		}
		// args := c.Args()
		// if len(args) < 2 {
		// 	return c.Send("Please provide a Series ID. Example: /download {title} {video_id}")
		// }
		// seriesID := args[len(args)-1]
		// // seriesID, err := strconv.Atoi(seriesIDStr)
		// if err != nil {
		// 	return c.Send("❌ Total part harus berupa angka!", telebot.ModeHTML)
		// }

		// // Judul = semua argumen kecuali angka terakhir
		// title := cleanTitle(strings.Join(args[:len(args)-1], " "))

		b, err := os.ReadFile("service-account.json")
		if err != nil {
			log.Fatal(err)
		}

		config, err := google.JWTConfigFromJSON(b, sheets.SpreadsheetsScope)
		if err != nil {
			log.Fatal(err)
		}

		client := config.Client(ctx)
		srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			log.Fatal(err)
		}

		readRange := sheetName + "!A2:E"
		resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
		if err != nil {
			log.Fatal(err)
		}

		for i, row := range resp.Values {
			if len(row) < 5 {
				continue
			}

			title := fmt.Sprint(row[0])
			title = strings.ReplaceAll(title, "(", " ")
			title = strings.ReplaceAll(title, ")", " ")
			title = cleanTitle(title)
			seriesID := fmt.Sprint(row[1])
			status := fmt.Sprint(row[3])
			telegramSeriesID := fmt.Sprint(row[4])
			telegramLink := fmt.Sprintf("https://t.me/DramaTrans/%s", telegramSeriesID)

			if status != "Pending" {
				continue
			}

			fmt.Println("Processing ID:", seriesID)
			titleFolder := strings.ToLower(strings.ReplaceAll(title, " ", "_")) // untuk folder + slug dasar
			c.Send(fmt.Sprintf("Starting download for series ID: %s...", seriesID))
			cmd := exec.Command("python3", "download.py", seriesID)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				log.Print(err)
				return c.Send("Failed to download: ")
			}
			c.Send("Download complete")
			baseDir := fmt.Sprintf(`%s/%s`, parentDir, title)
			// folderName := strings.Join(c.Args(), "")
			targetDir := filepath.Join(baseDir)
			log.Println(baseDir)
			log.Println(targetDir)
			files, err := os.ReadDir(targetDir)
			now := GetJakartaTime()
			videoCol := db.Collection("videos")
			dramaCol := db.Collection("drama")
			totalPart := 0
			// var targetFile string

			log.Print(len(files))

			for _, f := range files {
				if !f.IsDir() && filepath.Ext(f.Name()) == ".mp4" {
					totalPart++
				}
			}

			if totalPart == 0 {
				return c.Send("No mp4 files")
			}

			for i := 1; i <= totalPart; i++ {
				partTitle := fmt.Sprintf("%s part %d", title, i)
				partSlug := fmt.Sprintf("%s_part_%d", titleFolder, i)
				videoURL := fmt.Sprintf("%s/%s/%s.mp4", parentDir, titleFolder, partSlug)

				video := models.Video{
					Title:      partTitle,
					Slug:       partSlug,
					VIPOnly:    i != 1, // part 1 gratis, lainnya VIP
					VideoURL:   videoURL,
					UploadTime: now,
					Part:       i,
					TotalPart:  totalPart,
				}

				_, err := videoCol.InsertOne(ctx, video)
				if err != nil {
					log.Println("❌ Gagal insert video:", err)
					return c.Send("❌ Gagal menyimpan ke database.")
				}
			}

			drama := models.Drama{
				Id:               seriesID,
				Title:            title,
				Slug:             titleFolder,
				TotalPart:        totalPart,
				KeyWord:          "", // part 1 gratis, lainnya VIP
				Cast:             "",
				Tag:              "",
				TelegramSeriesID: telegramLink,
			}

			_, err = dramaCol.InsertOne(ctx, drama)
			if err != nil {
				log.Println("❌ Gagal insert drama:", err)
				return c.Send("❌ Gagal menyimpan ke database.")
			}

			msg := fmt.Sprintf("✅ Drama berhasil ditambahkan untuk judul <b>%s</b>\n✅ %d video berhasil ditambahkan untuk judul <b>%s</b>", title, totalPart, title)
			c.Send(fmt.Sprintf("%s, Uploading the video...", msg), telebot.ModeHTML)

			part := 1
			for _, f := range files {
				if f.IsDir() || filepath.Ext(f.Name()) != ".mp4" {
					continue
				}

				targetFile := filepath.Join(targetDir, f.Name())
				slug := fmt.Sprintf("%s_part_%d", titleFolder, part)
				var msg *telebot.Message
				maxRetries := 3
				success := false

				for attempt := 1; attempt <= maxRetries; attempt++ {
					c.Send(fmt.Sprintf("📤 Uploading (Attempt %d/%d): %s", attempt, maxRetries, filepath.Base(targetFile)))

					v := &telebot.Video{
						File:      telebot.FromDisk(targetFile),
						Caption:   slug,
						Streaming: true,
					}

					var err error
					msg, err = bot.Send(c.Chat(), v)

					if err == nil {
						success = true
						break // Success! Exit the retry loop
					}

					log.Printf("⚠️ Attempt %d failed for %s: %v", attempt, f.Name(), err)

					if attempt < maxRetries {
						time.Sleep(5 * time.Second) // Wait 5 seconds before retrying
					} else {
						log.Printf("❌ Final failure for %s after %d attempts", f.Name(), maxRetries)
					}
				}

				if success && msg != nil {
					fileID := msg.Video.FileID

					filter := bson.M{"slug": slug}
					update := bson.M{"$set": bson.M{"file_id": fileID}}

					_, err = videoCol.UpdateOne(ctx, filter, update)
					if err != nil {
						log.Println("❌ Gagal simpan file_id:", err)
					}

					if err := os.Remove(targetFile); err != nil {
						log.Printf("⚠️ Gagal menghapus file %s: %v", targetFile, err)
					} else {
						log.Printf("🗑️ File dihapus: %s", targetFile)
					}

					part++

					// Simpan ke Redis juga
					// fileIDKey := "fileid:" + slug
					// _ = redisClient.Set(ctx, fileIDKey, fileID, 30*24*time.Hour).Err()

					msgs := fmt.Sprintf("✅ FileID berhasil disimpan untuk slug <code>%s</code>:\n<code>%s</code>", slug, fileID)
					c.Send(msgs, telebot.ModeHTML)
				} else {
					return c.Send(fmt.Sprintf("❌ Failed to upload %s after multiple attempts.", f.Name()))
				}
			}
			cover := fmt.Sprintf("cover_%s.jpg", titleFolder)
			generatePost(c, fmt.Sprint(row[0]), title, part)
			os.Remove(cover)

			if err := os.RemoveAll(targetDir); err != nil {
				log.Printf("❌ Gagal menghapus folder %s: %v", targetDir, err)
			} else {
				log.Printf("✅ Folder %s berhasil dibersihkan", targetDir)
			}
			rowNumber := i + 2 // because start from A2
			updateRange := fmt.Sprintf("%s!D%d", sheetName, rowNumber)

			_, err = srv.Spreadsheets.Values.Update(
				spreadsheetID,
				updateRange,
				&sheets.ValueRange{
					Values: [][]interface{}{{"Done"}},
				},
			).ValueInputOption("RAW").Do()

			if err != nil {
				log.Println("Failed update status:", err)
			} else {
				fmt.Println("Marked Done:", seriesID)
			}
		}
		return c.Send("✅ Semua video berhasil diupload & disimpan.", telebot.ModeHTML)
	})

	type Tag struct {
		TagID   int    `json:"tag_id"`
		TagName string `json:"tag_name"`
		Hit     int    `json:"hit"`
	}

	type Playlet struct {
		PlayletID int    `json:"playlet_id"`
		Title     string `json:"title"`
		Cover     string `json:"cover"`
		UploadNum int    `json:"upload_num"`
		TagList   []Tag  `json:"tag_list"`
		Introduce string `json:"introduce"`
	}

	type ApiResponse struct {
		StatusCode int       `json:"status_code"`
		Msg        string    `json:"msg"`
		Data       []Playlet `json:"data"`
	}

	bot.Handle("/search", func(c telebot.Context) error {
		ownerID := os.Getenv("BOT_OWNER_ID")
		if fmt.Sprint(c.Sender().ID) != ownerID {
			return c.Send("❌ Kamu tidak punya akses ke perintah ini.")
		}
		query := c.Args()[0]
		resp, err := http.Get(fmt.Sprintf("https://api.sansekai.my.id/api/flickreels/search?query=%s", query))
		if err != nil {
			fmt.Println("Error:", err)
			return nil
		}

		// IMPORTANT: Always close the body to prevent memory leaks
		defer resp.Body.Close()

		// Read the response body
		if err != nil {
			fmt.Println("Read Error:", err)
			return nil
		}

		var apiData ApiResponse
		err = json.NewDecoder(resp.Body).Decode(&apiData)
		if err != nil {
			panic(err)
		}

		// Loop through the results
		msgs := "<b>📋 List Processing Results:</b>\n\n"

		// 2. Loop through your data to build the message
		for _, item := range apiData.Data {
			// We use <code> for the ID and bold for the title to match your style
			line := fmt.Sprintf("✅ <code>/process %s %d</code>\n", item.Title, item.PlayletID)

			// Check if adding this line exceeds Telegram's 4096 char limit
			if len(msgs)+len(line) > 4000 {
				c.Send(msgs, telebot.ModeHTML) // Send current batch
				msgs = ""                      // Reset for next batch
			}

			msgs += line
		}

		// 3. Send the final (or only) message
		return c.Send(msgs, telebot.ModeHTML)

		// fmt.Println(string(body))
		// return nil
	})

	bot.Handle("/upload", func(c telebot.Context) error {
		if len(c.Args()) == 0 {
			return c.Send("Please specify the folder name. Example: /upload video id")
		}
		baseDir := `C:\Users\hp\melolo\Nenek Muda Kebangkitan Keluarga`
		folderName := strings.Join(c.Args(), "")
		targetDir := filepath.Join(baseDir, folderName)
		files, err := os.ReadDir(targetDir)
		c.Send("Uploading...")
		if err != nil {
			return c.Send("Folder not found")
		}
		var targetFile string
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".mp4" {
				targetFile = filepath.Join(targetDir, f.Name())
			}
			if targetFile == "" {
				return c.Send("No mp4 files")
			}

			c.Send("Testing upload for: " + filepath.Base(targetFile))
			v := &telebot.Video{
				File:      telebot.FromDisk(targetFile),
				Caption:   "Test upload",
				Streaming: true,
			}

			msg, err := bot.Send(c.Chat(), v)
			if err != nil {
				return c.Send("Upload Failed: " + err.Error())
			}
			slug := "test_part_1"
			fileID := msg.Video.FileID
			videoCollection := db.Collection("videos")

			filter := bson.M{"slug": slug}
			update := bson.M{"$set": bson.M{"file_id": fileID}}
			res, err := videoCollection.UpdateOne(ctx, filter, update)
			if err != nil {
				log.Println("❌ Gagal menyimpan file ID:", err)
				return c.Send("❌ Gagal menyimpan ke database.")
			}

			if res.ModifiedCount == 0 {
				return c.Send("⚠️ Tidak ada video yang diperbarui. Pastikan slug benar.")
			}

			// Simpan ke Redis juga
			// fileIDKey := "fileid:" + slug
			// _ = redisClient.Set(ctx, fileIDKey, fileID, 30*24*time.Hour).Err()

			msgs := fmt.Sprintf("✅ FileID berhasil disimpan untuk slug <code>%s</code>:\n<code>%s</code>", slug, fileID)
			return c.Send(msgs, telebot.ModeHTML)
			// log.Print(msg.Video.FileID)
			// return c.Send(fmt.Sprintf("Test Success! ID: `%s", msg.Video.FileID), telebot.ModeMarkdown)
		}
		return nil
	})

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if waitingForReferral[c.Sender().ID] {
			delete(waitingForReferral, c.Sender().ID) // remove state after receiving
			code := strings.TrimSpace(c.Text())
			if code == "" {
				return c.Send("Referral anda adalah ")
			}
			return processReferral(c, code)
		}
		return nil // ignore other messages
	})

	bot.Handle(&telebot.Btn{Unique: "vip_1d"}, func(c telebot.Context) error {
		return sendQris(c, "vip_1d")
	})
	bot.Handle(&telebot.Btn{Unique: "vip_3d"}, func(c telebot.Context) error {
		return sendQris(c, "vip_3d")
	})
	bot.Handle(&telebot.Btn{Unique: "vip_7d"}, func(c telebot.Context) error {
		return sendQris(c, "vip_7d")
	})
	bot.Handle(&telebot.Btn{Unique: "vip_30d"}, func(c telebot.Context) error {
		return sendQris(c, "vip_30d")
	})

	// 🔙 Kembali

	bot.Handle(&backBtn, func(c telebot.Context) error {
		user := c.Sender()
		welcome := fmt.Sprintf(
			"%s, %s! 👋\n\nLagi cari tempat nonton drama China yang simpel?\n<b>DRAMATRANS</b> jawabannya!\n\nLangsung tonton dari Telegram tanpa harus buka aplikasi lain.\nKlik tombol di bawah buat mulai 🎥\nKalau ada yang mau ditanya, chat aja admin @domi_nuc ya!",
			getGreeting(), user.FirstName,
		)

		reply := &telebot.ReplyMarkup{}
		reply.Inline(reply.Row(startBtn), reply.Row(vipBtn, statusBtn))

		return c.Edit(welcome, reply, telebot.ModeHTML)
	})

	bot.Handle(&telebot.Btn{Unique: "cancel_payment"}, func(c telebot.Context) error {
		log.Print(c.Chat().ID)

		if lastMsg, ok := lastVideoMessages[c.Chat().ID]; ok {
			_ = bot.Delete(lastMsg)
		}

		return nil
	})

	bot.Handle(&telebot.Btn{Unique: "next_part"}, func(c telebot.Context) error {
		datas := c.Data() // e.g. "slug123|part2"
		data := strings.Split(datas, "|")
		part := data[1]
		slug_data := data[0]
		lastUnderscore := strings.LastIndex(slug_data, "_")
		slug := slug_data[:lastUnderscore+1] + part
		if lastMsg, ok := lastVideoMessages[c.Chat().ID]; ok {
			_ = bot.Delete(lastMsg)
		}
		sendVideo(c, slug)
		return nil
	})

	bot.Handle(&telebot.Btn{Unique: "prev_part"}, func(c telebot.Context) error {
		datas := c.Data() // e.g. "slug123|part2"
		data := strings.Split(datas, "|")
		part := data[1]
		slug_data := data[0]
		lastUnderscore := strings.LastIndex(slug_data, "_")
		slug := slug_data[:lastUnderscore+1] + part
		if lastMsg, ok := lastVideoMessages[c.Chat().ID]; ok {
			_ = bot.Delete(lastMsg)
		}
		sendVideo(c, slug)
		return nil
	})

	// bot.Handle(&prevBtn, func(c telebot.Context) error {
	// 	datas := c.Callback().Data
	// 	data := strings.Split(datas, "|")
	// 	part := data[1]
	// 	slug_data := data[0]
	// 	lastUnderscore := strings.LastIndex(slug_data, "_")
	// 	slug := slug_data[:lastUnderscore+1] + fmt.Sprint(part)
	// 	sendVideo(c, slug)
	// 	return nil
	// })

	bot.Handle("/addvideo", func(c telebot.Context) error {
		ownerID := os.Getenv("BOT_OWNER_ID")
		if fmt.Sprint(c.Sender().ID) != ownerID {
			return c.Send("❌ Kamu tidak punya akses ke perintah ini.")
		}

		args := strings.Split(c.Message().Payload, " ")
		if len(args) < 2 {
			return c.Send("❌ Format salah!\nGunakan:\n<code>/addvideo Judul TotalPart</code>", telebot.ModeHTML)
		}

		// Ambil total part (argumen terakhir)
		totalPartStr := args[len(args)-1]
		totalPart, err := strconv.Atoi(totalPartStr)
		if err != nil {
			return c.Send("❌ Total part harus berupa angka!", telebot.ModeHTML)
		}

		// Judul = semua argumen kecuali angka terakhir
		title := strings.Join(args[:len(args)-1], " ")
		titleFolder := strings.ToLower(strings.ReplaceAll(title, " ", "_")) // untuk folder + slug dasar

		now := GetJakartaTime()
		videoCol := db.Collection("videos")

		for i := 1; i <= totalPart; i++ {
			partTitle := fmt.Sprintf("%s part %d", title, i)
			partSlug := fmt.Sprintf("%s_part_%d", titleFolder, i)
			videoURL := fmt.Sprintf("C://%s/%s.mp4", titleFolder, partSlug)

			video := models.Video{
				Title:      partTitle,
				Slug:       partSlug,
				VIPOnly:    i != 1, // part 1 gratis, lainnya VIP
				VideoURL:   videoURL,
				UploadTime: now,
				Part:       i,
				TotalPart:  totalPart,
			}

			_, err := videoCol.InsertOne(ctx, video)
			if err != nil {
				log.Println("❌ Gagal insert video:", err)
				return c.Send("❌ Gagal menyimpan ke database.")
			}
		}

		msg := fmt.Sprintf("✅ %d video berhasil ditambahkan untuk judul <b>%s</b>", totalPart, title)
		return c.Send(msg, telebot.ModeHTML)
	})

	bot.Handle("/addduration", func(c telebot.Context) error {
		ownerID := os.Getenv("BOT_OWNER_ID")
		if fmt.Sprint(c.Sender().ID) != ownerID {
			return c.Send("❌ Kamu tidak punya akses ke perintah ini.")
		}

		args := strings.Split(c.Message().Text, " ")
		if len(args) != 3 {
			return c.Send("⚠️ Format salah!\nContoh: /addduration 123456789 7")
		}

		targetUserName := args[1]
		daysToAdd, err := strconv.Atoi(args[2])
		if err != nil {
			return c.Send("⚠️ Jumlah hari tidak valid.")
		}

		var user models.User
		userCollection := db.Collection("users")
		err = userCollection.FindOne(ctx, bson.M{"telegram_username": targetUserName}).Decode(&user)
		if err != nil {
			return c.Send("❌ User tidak ditemukan.")
		}
		now := GetJakartaTime()

		// Convert to Jakarta time

		if user.ExpireTime == nil || user.ExpireTime.Before(now) {
			// If expired or nil, start from now
			if daysToAdd > 0 {
				newExpire := now.Add(time.Duration(daysToAdd) * 24 * time.Hour)
				user.ExpireTime = &newExpire
			}
		} else {
			// If still active, extend from current expire time
			newExpire := user.ExpireTime.Add(time.Duration(daysToAdd) * 24 * time.Hour)
			if newExpire.Before(now.Add(24 * time.Hour)) {
				user.ExpireTime = nil
			} else {
				user.ExpireTime = &newExpire
			}
		}

		if user.ExpireTime == nil {
			user.IsVIP = false
		} else {
			user.IsVIP = true
		}
		_, err = userCollection.UpdateOne(
			ctx,
			bson.M{"telegram_username": targetUserName},
			bson.M{"$set": bson.M{"expire_time": user.ExpireTime, "is_vip": user.IsVIP}},
		)
		if err != nil {
			return c.Send("❌ Gagal memperbarui expire_time.")
		}

		if user.ExpireTime == nil {
			return c.Send(fmt.Sprintf("✅ VIP user %s dihapus (habis masa aktif).", targetUserName))
		}

		return c.Send(fmt.Sprintf("✅ VIP user %s diperpanjang/dikurangi. Expire baru: %s",
			targetUserName, user.ExpireTime.Format("2006-01-02 15:04:05")))
	})

	bot.Handle("/savefileid", func(c telebot.Context) error {
		ownerID := os.Getenv("BOT_OWNER_ID")
		if fmt.Sprint(c.Sender().ID) != ownerID {
			return c.Send("❌ Kamu tidak punya akses ke perintah ini.")
		}

		if c.Message().ReplyTo == nil || c.Message().ReplyTo.Video == nil {
			return c.Send("❌ Balas pesan video yang dikirim bot untuk menyimpan FileID.")
		}

		args := strings.Split(c.Message().Payload, " ")
		if len(args) != 1 {
			return c.Send("❌ Format salah. Gunakan: <code>/savefileid slugnya</code>", telebot.ModeHTML)
		}

		slug := args[0]
		fileID := c.Message().ReplyTo.Video.File.FileID
		videoCollection := db.Collection("videos")

		filter := bson.M{"slug": slug}
		update := bson.M{"$set": bson.M{"file_id": fileID}}
		res, err := videoCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			log.Println("❌ Gagal menyimpan file ID:", err)
			return c.Send("❌ Gagal menyimpan ke database.")
		}

		if res.ModifiedCount == 0 {
			return c.Send("⚠️ Tidak ada video yang diperbarui. Pastikan slug benar.")
		}

		// Simpan ke Redis juga
		// fileIDKey := "fileid:" + slug
		// _ = redisClient.Set(ctx, fileIDKey, fileID, 30*24*time.Hour).Err()

		msg := fmt.Sprintf("✅ FileID berhasil disimpan untuk slug <code>%s</code>:\n<code>%s</code>", slug, fileID)
		return c.Send(msg, telebot.ModeHTML)
	})
	bot.SetCommands([]telebot.Command{
		{Text: "start", Description: "Mulai bot"},
		{Text: "vip", Description: "Langganan VIP"},
		{Text: "status", Description: "Cek status akun"},
	})

	log.Println("🤖 Bot is running...")
	go bot.Start() // Jalankan bot

	http.HandleFunc("/webhook/pakasir", handleWebhook)
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getGreeting() string {
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

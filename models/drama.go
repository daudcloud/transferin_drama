package models

type Drama struct {
	Id               string `bson:"id"`
	Title            string `bson:"title"`
	Slug             string `bson:"slug"`
	TotalPart        int    `bson:"total_part"`
	KeyWord          string `bson:"key_word, omitempty"`
	Cast             string `bson:"cast, omitempty"`
	Tag              string `bson:"tag, omitempty"`
	TelegramSeriesID string `bson:"telegram_series_id"`
}

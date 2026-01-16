package models

import "time"

type Video struct {
	Title      string    `bson:"title"`
	Slug       string    `bson:"slug"`
	VIPOnly    bool      `bson:"vip_only"`
	VideoURL   string    `bson:"video_url"`
	FileID     string    `bson:"file_id,omitempty"` // store backup here
	UploadTime time.Time `bson:"upload_time"`
	Part       int       `bson:"part"`
	TotalPart  int       `bson:"total_part"`
}

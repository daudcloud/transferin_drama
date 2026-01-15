package models

type Drama struct {
	Id        int    `bson:"id"`
	Title     string `bson:"title"`
	Slug      string `bson:"slug"`
	TotalPart int    `bson:"total_part"`
	KeyWord   string `bson:"key_word, omitempty"`
	Cast      string `bson:"cast, omitempty"`
}

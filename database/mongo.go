// database/mongo.go
package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var databaseName string

func Connect(uri, db string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(500). // Increase pool size
		SetMinPoolSize(50).  // Keep connections warm
		SetMaxConnIdleTime(30 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)
	client, err = mongo.Connect(ctx, clientOptions)
	if err == nil {
		databaseName = db
	}
	return err
}

func GetUserCollection() *mongo.Collection {
	return client.Database(databaseName).Collection("users")
}

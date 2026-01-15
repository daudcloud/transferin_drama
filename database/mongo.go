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
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err == nil {
		databaseName = db
	}
	return err
}

func GetUserCollection() *mongo.Collection {
	return client.Database(databaseName).Collection("users")
}

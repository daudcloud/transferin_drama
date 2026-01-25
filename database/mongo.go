package database

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var databaseName string

func Connect(uri, db string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(500).
		SetMinPoolSize(50).
		SetMaxConnIdleTime(30 * time.Second).
		SetServerSelectionTimeout(5 * time.Second).
		SetConnectTimeout(10 * time.Second).
		SetSocketTimeout(15 * time.Second)

	var err error
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}

	if err = client.Ping(ctx, nil); err != nil {
		return err
	}

	databaseName = db
	log.Println("✅ Connected to MongoDB with connection pool configured")
	return nil
}

// In database/mongo.go
func GetClient() *mongo.Client {
	if client == nil {
		log.Fatal("❌ MongoDB client not initialized")
	}
	return client
}

func Disconnect() error {
	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return client.Disconnect(ctx)
}

func GetDatabase() *mongo.Database {
	if client == nil {
		log.Fatal("❌ CRITICAL: MongoDB client is nil. Database not connected!")
	}
	if databaseName == "" {
		log.Fatal("❌ CRITICAL: Database name not set!")
	}
	return client.Database(databaseName)
}

func GetUserCollection() *mongo.Collection {
	db := GetDatabase()
	if db == nil {
		log.Fatal("❌ CRITICAL: Database is nil in GetUserCollection")
	}
	return db.Collection("users")
}

func GetVideoCollection() *mongo.Collection {
	db := GetDatabase()
	if db == nil {
		log.Fatal("❌ CRITICAL: Database is nil in GetVideoCollection")
	}
	return db.Collection("videos")
}

func GetDramaCollection() *mongo.Collection {
	db := GetDatabase()
	if db == nil {
		log.Fatal("❌ CRITICAL: Database is nil in GetDramaCollection")
	}
	return db.Collection("drama")
}

func GetTransactionPendingCollection() *mongo.Collection {
	db := GetDatabase()
	if db == nil {
		log.Fatal("❌ CRITICAL: Database is nil in GetTransactionPendingCollection")
	}
	return db.Collection("transactionPending")
}

func GetTransactionSuccessCollection() *mongo.Collection {
	db := GetDatabase()
	if db == nil {
		log.Fatal("❌ CRITICAL: Database is nil in GetTransactionSuccessCollection")
	}
	return db.Collection("transactionSuccess")
}

func HealthCheck(ctx context.Context) error {
	if client == nil {
		return mongo.ErrClientDisconnected
	}
	return client.Ping(ctx, nil)
}

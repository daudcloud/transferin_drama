// database/mongo.go
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

// Connect establishes connection to MongoDB with proper configuration
func Connect(uri, db string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(500).                        // Increase pool size
		SetMinPoolSize(50).                         // Keep warm connections
		SetMaxConnIdleTime(30 * time.Second).       // Close idle connections after 30s
		SetServerSelectionTimeout(5 * time.Second). // Fail fast if server unavailable
		SetSocketTimeout(15 * time.Second)          // Individual query timeout

	var err error
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}

	// Verify connection with ping
	if err = client.Ping(ctx, nil); err != nil {
		return err
	}

	databaseName = db
	log.Println("✅ Connected to MongoDB with connection pool configured")
	return nil
}

// Disconnect closes the MongoDB connection gracefully
func Disconnect() error {
	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return client.Disconnect(ctx)
}

// GetDatabase returns the database instance
func GetDatabase() *mongo.Database {
	if client == nil {
		log.Fatal("❌ MongoDB client not initialized. Call Connect() first.")
	}
	return client.Database(databaseName)
}

// GetUserCollection returns the users collection
func GetUserCollection() *mongo.Collection {
	return GetDatabase().Collection("users")
}

// GetVideoCollection returns the videos collection
func GetVideoCollection() *mongo.Collection {
	return GetDatabase().Collection("videos")
}

// GetDramaCollection returns the drama collection
func GetDramaCollection() *mongo.Collection {
	return GetDatabase().Collection("drama")
}

// GetTransactionPendingCollection returns the transactionPending collection
func GetTransactionPendingCollection() *mongo.Collection {
	return GetDatabase().Collection("transactionPending")
}

// GetTransactionSuccessCollection returns the transactionSuccess collection
func GetTransactionSuccessCollection() *mongo.Collection {
	return GetDatabase().Collection("transactionSuccess")
}

// GetClient returns the MongoDB client (use sparingly)
func GetClient() *mongo.Client {
	if client == nil {
		log.Fatal("❌ MongoDB client not initialized. Call Connect() first.")
	}
	return client
}

// HealthCheck verifies MongoDB connection is alive
func HealthCheck(ctx context.Context) error {
	if client == nil {
		return mongo.ErrClientDisconnected
	}
	return client.Ping(ctx, nil)
}

package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Connect establishes a MongoDB connection and pings it.
func Connect(ctx context.Context, uri, dbName string) (*mongo.Database, error) {
	opts := options.Client().ApplyURI(uri).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	db := client.Database(dbName)
	if err := ensureIndexes(ctx, db); err != nil {
		return nil, err
	}
	return db, nil
}

// ensureIndexes creates all required indexes idempotently.
func ensureIndexes(ctx context.Context, db *mongo.Database) error {
	// jobs: scheduler scan + owner listing
	_, err := db.Collection("jobs").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "next_run_at", Value: 1}},
			Options: options.Index().SetName("status_next_run_at"),
		},
		{
			Keys:    bson.D{{Key: "owner_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("owner_created"),
		},
	})
	if err != nil {
		return err
	}

	// runs: job history + reaper + dedup + TTL
	_, err = db.Collection("runs").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "job_id", Value: 1}, {Key: "scheduled_for", Value: -1}},
			Options: options.Index().SetName("job_scheduled"),
		},
		{
			Keys:    bson.D{{Key: "state", Value: 1}, {Key: "worker_id", Value: 1}},
			Options: options.Index().SetName("state_worker"),
		},
		{
			// Dedup: a job can only have one run per scheduled_for time.
			Keys:    bson.D{{Key: "job_id", Value: 1}, {Key: "scheduled_for", Value: 1}},
			Options: options.Index().SetName("job_scheduled_uniq").SetUnique(true).SetSparse(true),
		},
		{
			// TTL: runs expire after 30 days (expires_at set on creation).
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetName("expires_at_ttl").SetExpireAfterSeconds(0),
		},
	})
	if err != nil {
		return err
	}

	// workers: heartbeat scan
	_, err = db.Collection("workers").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "last_heartbeat", Value: 1}},
		Options: options.Index().SetName("last_heartbeat"),
	})
	return err
}

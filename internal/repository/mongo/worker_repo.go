package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/yourname/jobscheduler/internal/domain"
	"github.com/yourname/jobscheduler/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type workerRepo struct {
	col *mongo.Collection
}

// NewWorkerRepo returns a MongoDB-backed WorkerRepository.
func NewWorkerRepo(db *mongo.Database) repository.WorkerRepository {
	return &workerRepo{col: db.Collection("workers")}
}

func (r *workerRepo) Upsert(ctx context.Context, w *domain.Worker) error {
	filter := bson.M{"_id": w.ID}
	update := bson.M{"$set": w}
	opts := options.Update().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *workerRepo) GetByID(ctx context.Context, id string) (*domain.Worker, error) {
	var w domain.Worker
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&w)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &w, err
}

func (r *workerRepo) ListAll(ctx context.Context) ([]*domain.Worker, error) {
	cur, err := r.col.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "last_heartbeat", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var workers []*domain.Worker
	return workers, cur.All(ctx, &workers)
}

func (r *workerRepo) MarkOffline(ctx context.Context, id string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": domain.WorkerOffline}})
	return err
}

func (r *workerRepo) FindDeadWorkers(ctx context.Context, olderThan time.Duration) ([]*domain.Worker, error) {
	threshold := time.Now().UTC().Add(-olderThan)
	filter := bson.M{
		"status":         domain.WorkerOnline,
		"last_heartbeat": bson.M{"$lt": threshold},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var workers []*domain.Worker
	return workers, cur.All(ctx, &workers)
}

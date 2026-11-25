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

type jobRepo struct {
	col *mongo.Collection
}

// NewJobRepo returns a MongoDB-backed JobRepository.
func NewJobRepo(db *mongo.Database) repository.JobRepository {
	return &jobRepo{col: db.Collection("jobs")}
}

func (r *jobRepo) Create(ctx context.Context, job *domain.Job) error {
	_, err := r.col.InsertOne(ctx, job)
	return err
}

func (r *jobRepo) GetByID(ctx context.Context, id, ownerID string) (*domain.Job, error) {
	var job domain.Job
	err := r.col.FindOne(ctx, bson.M{"_id": id, "owner_id": ownerID}).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &job, err
}

func (r *jobRepo) List(ctx context.Context, f repository.ListJobsFilter) ([]*domain.Job, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filter := bson.M{"owner_id": f.OwnerID}
	if f.Status != "" {
		filter["status"] = f.Status
	}
	if f.Cursor != "" {
		filter["_id"] = bson.M{"$gt": f.Cursor}
	}
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit + 1))
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", err
	}
	defer cur.Close(ctx)

	var jobs []*domain.Job
	if err := cur.All(ctx, &jobs); err != nil {
		return nil, "", err
	}
	var nextCursor string
	if len(jobs) > limit {
		nextCursor = jobs[limit].ID
		jobs = jobs[:limit]
	}
	return jobs, nextCursor, nil
}

func (r *jobRepo) Update(ctx context.Context, job *domain.Job) error {
	job.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": job.ID, "owner_id": job.OwnerID}, job)
	return err
}

func (r *jobRepo) Delete(ctx context.Context, id, ownerID string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id, "owner_id": ownerID})
	return err
}

// ClaimDue atomically claims a job using the old next_run_at as a compare-and-swap.
// Returns nil if another scheduler won the race (NFR-02).
func (r *jobRepo) ClaimDue(ctx context.Context, jobID string, oldNextRunAt time.Time, newNextRunAt *time.Time) (*domain.Job, error) {
	filter := bson.M{
		"_id":         jobID,
		"status":      domain.JobStatusActive,
		"next_run_at": oldNextRunAt,
	}
	update := bson.M{"$set": bson.M{
		"next_run_at": newNextRunAt,
		"updated_at":  time.Now().UTC(),
	}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var job domain.Job
	err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil // another scheduler won
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// FindDue returns active jobs whose next_run_at is in the past.
func (r *jobRepo) FindDue(ctx context.Context, now time.Time, limit int) ([]*domain.Job, error) {
	filter := bson.M{
		"status":      domain.JobStatusActive,
		"next_run_at": bson.M{"$lte": now},
	}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "next_run_at", Value: 1}})
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var jobs []*domain.Job
	return jobs, cur.All(ctx, &jobs)
}

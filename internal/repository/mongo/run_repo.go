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

type runRepo struct {
	col *mongo.Collection
}

// NewRunRepo returns a MongoDB-backed RunRepository.
func NewRunRepo(db *mongo.Database) repository.RunRepository {
	return &runRepo{col: db.Collection("runs")}
}

func (r *runRepo) Create(ctx context.Context, run *domain.Run) error {
	_, err := r.col.InsertOne(ctx, run)
	return err
}

func (r *runRepo) GetByID(ctx context.Context, id, ownerID string) (*domain.Run, error) {
	var run domain.Run
	err := r.col.FindOne(ctx, bson.M{"_id": id, "owner_id": ownerID}).Decode(&run)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &run, err
}

func (r *runRepo) GetByIDOnly(ctx context.Context, id string) (*domain.Run, error) {
    var run domain.Run
    err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&run)
    if errors.Is(err, mongo.ErrNoDocuments) {
        return nil, nil
    }
    return &run, err
}

func (r *runRepo) List(ctx context.Context, f repository.ListRunsFilter) ([]*domain.Run, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filter := bson.M{"owner_id": f.OwnerID}
	if f.JobID != "" {
		filter["job_id"] = f.JobID
	}
	if f.State != "" {
		filter["state"] = f.State
	}
	if f.Cursor != "" {
		filter["_id"] = bson.M{"$lt": f.Cursor}
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "scheduled_for", Value: -1}}).
		SetLimit(int64(limit + 1))

	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", err
	}
	defer cur.Close(ctx)

	var runs []*domain.Run
	if err := cur.All(ctx, &runs); err != nil {
		return nil, "", err
	}
	var nextCursor string
	if len(runs) > limit {
		nextCursor = runs[limit].ID
		runs = runs[:limit]
	}
	return runs, nextCursor, nil
}

// MarkRunning does a conditional update: queued|retrying -> running.
func (r *runRepo) MarkRunning(ctx context.Context, runID, workerID string) error {
	now := time.Now().UTC()
	filter := bson.M{
		"_id":   runID,
		"state": bson.M{"$in": []domain.RunState{domain.RunStateQueued, domain.RunStateRetrying}},
	}
	update := bson.M{"$set": bson.M{
		"state":      domain.RunStateRunning,
		"worker_id":  workerID,
		"started_at": now,
	}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		// Already terminal or running (duplicate delivery). Safe to skip.
		return nil
	}
	return nil
}

// MarkFinished saves the terminal state, output and attempt log entry.
func (r *runRepo) MarkFinished(ctx context.Context, runID string, state domain.RunState,
	output, errMsg string, httpStatus int, durationMs int64, log domain.AttemptLog) error {

	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"state":       state,
			"finished_at": now,
			"duration_ms": durationMs,
			"output":      output,
			"error":       errMsg,
			"http_status": httpStatus,
		},
		"$push": bson.M{"attempts_log": log},
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": runID}, update)
	return err
}

// MarkRetrying transitions running -> retrying and increments attempt.
func (r *runRepo) MarkRetrying(ctx context.Context, runID string, attempt int, log domain.AttemptLog) error {
	update := bson.M{
		"$set":  bson.M{"state": domain.RunStateRetrying, "attempt": attempt},
		"$push": bson.M{"attempts_log": log},
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": runID}, update)
	return err
}

// FindStuckQueued returns queued runs older than the given duration.
func (r *runRepo) FindStuckQueued(ctx context.Context, olderThan time.Duration) ([]*domain.Run, error) {
	threshold := time.Now().UTC().Add(-olderThan)
	filter := bson.M{
		"state":        domain.RunStateQueued,
		"scheduled_for": bson.M{"$lt": threshold},
	}
	cur, err := r.col.Find(ctx, filter, options.Find().SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var runs []*domain.Run
	return runs, cur.All(ctx, &runs)
}

// FindStuckRunning returns running runs owned by the given offline worker IDs.
func (r *runRepo) FindStuckRunning(ctx context.Context, workerIDs []string) ([]*domain.Run, error) {
	if len(workerIDs) == 0 {
		return nil, nil
	}
	filter := bson.M{
		"state":     domain.RunStateRunning,
		"worker_id": bson.M{"$in": workerIDs},
	}
	cur, err := r.col.Find(ctx, filter, options.Find().SetLimit(200))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var runs []*domain.Run
	return runs, cur.All(ctx, &runs)
}

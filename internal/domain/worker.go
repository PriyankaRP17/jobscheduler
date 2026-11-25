package domain

import "time"

// WorkerStatus reflects the health of a worker instance.
type WorkerStatus string

const (
	WorkerOnline  WorkerStatus = "online"
	WorkerOffline WorkerStatus = "offline"
)

// Worker tracks a running worker instance.
type Worker struct {
	ID            string       `bson:"_id" json:"id"`
	Host          string       `bson:"host" json:"host"`
	Status        WorkerStatus `bson:"status" json:"status"`
	InFlight      int          `bson:"in_flight" json:"in_flight"`
	LastHeartbeat time.Time    `bson:"last_heartbeat" json:"last_heartbeat"`
	StartedAt     time.Time    `bson:"started_at" json:"started_at"`
	Version       string       `bson:"version" json:"version"`
}

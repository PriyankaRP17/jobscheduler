package queue

// Stream names
const (
	StreamJobs      = "JOBS"
	StreamJobsDead  = "JOBS_DEAD"
	StreamJobEvents = "JOB_EVENTS"
)

// Subjects for the JOBS stream
const (
	SubjectRunHigh    = "jobs.run.high"
	SubjectRunDefault = "jobs.run.default"
	SubjectRunAll     = "jobs.run.>"
)

// Subjects for the JOBS_DEAD stream
const (
	SubjectDead = "jobs.dead"
)

// Subjects for the JOB_EVENTS stream
const (
	SubjectEventAll       = "events.run.>"
	SubjectEventStarted   = "events.run.started"
	SubjectEventSucceeded = "events.run.succeeded"
	SubjectEventRetrying  = "events.run.retrying"
	SubjectEventFailed    = "events.run.failed"
	SubjectEventDead      = "events.run.dead"
)

// Consumer names
const (
	ConsumerWorkers = "workers"
)

// SubjectForPriority maps job priority to a NATS subject.
func SubjectForPriority(priority string) string {
	if priority == "high" {
		return SubjectRunHigh
	}
	return SubjectRunDefault
}

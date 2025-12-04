package storage

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// DBQueue provides safe concurrent access to SQLite database
type DBQueue struct {
	db         *sql.DB
	queryQueue chan *dbRequest
	done       chan struct{}
}

// dbRequest represents a database operation request
type dbRequest struct {
	query    func(*sql.DB) error
	response chan error
}

// NewDBQueue creates a new DBQueue instance
func NewDBQueue(db *sql.DB) *DBQueue {
	q := &DBQueue{
		db:         db,
		queryQueue: make(chan *dbRequest, 100),
		done:       make(chan struct{}),
	}
	go q.processQueue()
	return q
}

// processQueue processes database requests sequentially
func (q *DBQueue) processQueue() {
	for {
		select {
		case req := <-q.queryQueue:
			err := q.executeWithRetry(req.query)
			req.response <- err
		case <-q.done:
			return
		}
	}
}

// executeWithRetry executes a query with retry logic for SQLITE_BUSY errors
func (q *DBQueue) executeWithRetry(query func(*sql.DB) error) error {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := query(q.db)
		if err == nil {
			return nil
		}
		if isBusyError(err) {
			time.Sleep(time.Millisecond * time.Duration(100*(i+1)))
			continue
		}
		return err
	}
	return errors.New("max retries exceeded for SQLITE_BUSY")
}

// isBusyError checks if the error is a SQLITE_BUSY error
func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "SQLITE_BUSY")
}

// Execute executes a database operation through the queue
func (q *DBQueue) Execute(query func(*sql.DB) error) error {
	req := &dbRequest{
		query:    query,
		response: make(chan error, 1),
	}
	q.queryQueue <- req
	return <-req.response
}

// Close closes the DBQueue and stops processing
func (q *DBQueue) Close() {
	close(q.done)
}

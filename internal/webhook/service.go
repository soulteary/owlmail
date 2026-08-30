package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/types"
)

const (
	defaultMemoryQueueSize = 1024
	defaultShutdownTimeout = 15 * time.Second
	defaultLeaseRefresh    = redisClaimIdle / 3
)

// ServiceOptions configures queued webhook delivery.
type ServiceOptions struct {
	RedisURL        string
	RedisPrefix     string
	MaxConcurrency  int
	ShutdownTimeout time.Duration
	OnResults       func(string, []Result)
}

type deliveryJob struct {
	ID         string       `json:"id"`
	EnqueuedAt time.Time    `json:"enqueuedAt"`
	Email      *types.Email `json:"email"`
}

// Service owns webhook queue workers and their cancellation/drain lifecycle.
type Service struct {
	dispatcher      *Dispatcher
	queue           deliveryQueue
	memoryQueue     chan deliveryJob
	ctx             context.Context
	cancel          context.CancelFunc
	shutdownTimeout time.Duration
	onResults       func(string, []Result)
	workerCount     int
	leaseRefresh    time.Duration
	unlimited       bool
	accepting       bool
	handoffMutex    sync.RWMutex
	closeOnce       sync.Once
	handoffs        sync.WaitGroup
	workers         sync.WaitGroup
	deliveries      sync.WaitGroup
	closeErr        error
}

// NewService creates and starts a queued delivery service. Redis-backed mode
// uses a consumer-group stream and dead-letter stream; an empty Redis URL keeps
// a process-local queue while retaining cancellation and graceful drain.
func NewService(dispatcher *Dispatcher, options ServiceOptions) (*Service, error) {
	if dispatcher == nil {
		return nil, fmt.Errorf("webhook dispatcher is nil")
	}
	if options.MaxConcurrency < 0 {
		return nil, fmt.Errorf("webhook max concurrency must be zero or greater")
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = defaultShutdownTimeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	service := &Service{
		dispatcher: dispatcher.withMaxConcurrency(options.MaxConcurrency), ctx: ctx, cancel: cancel,
		shutdownTimeout: options.ShutdownTimeout, onResults: options.OnResults,
		workerCount: options.MaxConcurrency, unlimited: options.MaxConcurrency == 0,
		leaseRefresh: defaultLeaseRefresh,
	}
	if options.RedisURL != "" {
		consumer := fmt.Sprintf("%s-%s", runtime.GOOS, uuid.NewString())
		queue, err := newRedisDeliveryQueue(ctx, options.RedisURL, options.RedisPrefix, consumer)
		if err != nil {
			cancel()
			return nil, err
		}
		service.queue = queue
	} else {
		service.memoryQueue = make(chan deliveryJob, defaultMemoryQueueSize)
	}
	service.accepting = true
	service.start()
	return service, nil
}

func (service *Service) start() {
	if service.unlimited {
		service.workers.Add(1)
		go service.runPump()
		return
	}
	for i := 0; i < service.workerCount; i++ {
		service.workers.Add(1)
		go service.runWorker()
	}
}

// Enqueue durably hands an email to Redis when configured. Delivery remains
// at-least-once: receivers should deduplicate X-OwlMail-Delivery-ID.
func (service *Service) Enqueue(email *types.Email) error {
	if service == nil || email == nil {
		return fmt.Errorf("webhook email is nil")
	}
	service.handoffMutex.Lock()
	if !service.accepting {
		service.handoffMutex.Unlock()
		return fmt.Errorf("webhook service is shutting down")
	}
	service.handoffs.Add(1)
	service.handoffMutex.Unlock()
	defer service.handoffs.Done()
	emailSnapshot, err := cloneDeliveryEmail(email)
	if err != nil {
		return err
	}
	job := deliveryJob{ID: uuid.NewString(), EnqueuedAt: time.Now().UTC(), Email: emailSnapshot}
	if service.queue != nil {
		if err := service.queue.Enqueue(service.ctx, job); err != nil {
			return err
		}
		return nil
	}
	select {
	case service.memoryQueue <- job:
		return nil
	case <-service.ctx.Done():
		return service.ctx.Err()
	}
}

func cloneDeliveryEmail(email *types.Email) (*types.Email, error) {
	encoded, err := json.Marshal(email)
	if err != nil {
		return nil, fmt.Errorf("encode webhook email snapshot: %w", err)
	}
	var snapshot types.Email
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return nil, fmt.Errorf("decode webhook email snapshot: %w", err)
	}
	return &snapshot, nil
}

func (service *Service) runWorker() {
	defer service.workers.Done()
	for {
		job, receipt, ok := service.nextJob()
		if !ok {
			return
		}
		service.deliver(job, receipt)
	}
}

func (service *Service) runPump() {
	defer service.workers.Done()
	for {
		job, receipt, ok := service.nextJob()
		if !ok {
			return
		}
		service.deliveries.Add(1)
		go func() {
			defer service.deliveries.Done()
			service.deliver(job, receipt)
		}()
	}
}

func (service *Service) nextJob() (deliveryJob, *queueReceipt, bool) {
	if service.queue == nil {
		select {
		case job, open := <-service.memoryQueue:
			return job, nil, open
		case <-service.ctx.Done():
			return deliveryJob{}, nil, false
		}
	}
	for {
		if !service.isAccepting() {
			pending, err := service.queue.Pending(service.ctx)
			if err == nil && pending == 0 {
				return deliveryJob{}, nil, false
			}
		}
		receipt, err := service.queue.Claim(service.ctx)
		if err != nil {
			if service.ctx.Err() != nil {
				return deliveryJob{}, nil, false
			}
			service.report("", []Result{{Err: err}})
			continue
		}
		if receipt != nil {
			return receipt.job, receipt, true
		}
	}
}

func (service *Service) isAccepting() bool {
	service.handoffMutex.RLock()
	defer service.handoffMutex.RUnlock()
	return service.accepting
}

func (service *Service) deliver(job deliveryJob, receipt *queueReceipt) {
	stopLease := service.startLeaseHeartbeat(receipt)
	results := service.dispatcher.DispatchDelivery(service.ctx, job.Email, job.ID)
	stopLease()
	failed := false
	for _, result := range results {
		if result.Err != nil {
			failed = true
			break
		}
	}
	if service.queue != nil {
		var err error
		if failed {
			err = service.queue.DeadLetter(service.ctx, receipt, results)
		} else {
			err = service.queue.Ack(service.ctx, receipt)
		}
		if err != nil {
			results = append(results, Result{Err: err})
		}
	}
	service.report(job.ID, results)
}

func (service *Service) startLeaseHeartbeat(receipt *queueReceipt) func() {
	if service.queue == nil || receipt == nil || service.leaseRefresh <= 0 {
		return func() {}
	}
	ctx, cancel := context.WithCancel(service.ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(service.leaseRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := service.queue.Renew(ctx, receipt); err != nil && ctx.Err() == nil {
					service.report(receipt.job.ID, []Result{{Err: err}})
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (service *Service) report(jobID string, results []Result) {
	if service.onResults != nil {
		service.onResults(jobID, results)
	}
}

// Close stops accepting jobs, drains queued and active delivery until the
// configured deadline, then cancels remaining HTTP and Redis operations.
func (service *Service) Close() error {
	if service == nil {
		return nil
	}
	service.closeOnce.Do(func() {
		service.handoffMutex.Lock()
		service.accepting = false
		service.handoffMutex.Unlock()
		drained := make(chan struct{})
		go func() {
			service.handoffs.Wait()
			if service.memoryQueue != nil {
				close(service.memoryQueue)
			}
			service.workers.Wait()
			service.deliveries.Wait()
			close(drained)
		}()
		timer := time.NewTimer(service.shutdownTimeout)
		defer timer.Stop()
		select {
		case <-drained:
		case <-timer.C:
			service.closeErr = fmt.Errorf("webhook drain exceeded %s", service.shutdownTimeout)
			service.cancel()
			<-drained
		}
		service.cancel()
		if service.queue != nil {
			if err := service.queue.Close(); err != nil && !errors.Is(err, context.Canceled) && service.closeErr == nil {
				service.closeErr = err
			}
		}
	})
	return service.closeErr
}

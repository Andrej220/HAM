package workerpool

import (
	//"log"
	"github.com/andrej220/HAM/pkg/lg"
	"sync"
	"sync/atomic"
	"context"
	"time"
	"fmt"
)
const (
	TotalMaxWorkers = 10
	maxAttemps		= 3
)

type JobFunc[T any] func(T) error

type Job[T any] struct{
	Payload 	T
	Fn			JobFunc[T]
	Ctx			context.Context
	CleanupFunc func()
}

type Pool[T any] struct {
	Jobs         chan Job[T]      
	activeWorkers int32         
	wg           sync.WaitGroup 
	quit         chan struct{}  
	maxWorkers   int
}

func NewPool[T any](maxWorkers int) *Pool[T] {
	if maxWorkers <= 0 {
		maxWorkers = TotalMaxWorkers
	}
	pool := &Pool[T]{
		Jobs:  make(chan Job[T], maxWorkers), 
		quit:  make(chan struct{}),
		maxWorkers: maxWorkers,
	}
	go pool.dispatch() 
	return pool
}

func (p *Pool[T]) Stop() {
	close(p.quit)
	p.wg.Wait()
	close(p.Jobs)
}

func (p *Pool[T]) Submit( job Job[T]) {
	logger := lg.FromContext(job.Ctx)
	select {
	case p.Jobs <- job:
		//log.Printf("Job submitted with payload: %+v", job.Payload)
		logger.Info("Job submitted",lg.Any("job", job.Payload) )
	case <-p.quit:
		logger.Info("Worker pool is shutting down, job rejected")
		//log.Println("Worker pool is shutting down, job rejected")
	}
}

func (p *Pool[T]) dispatch() {
	for {
		select {
		case job := <-p.Jobs:
			p.wg.Add(1)
			atomic.AddInt32(&p.activeWorkers, 1)
			go p.worker(job)
		case <-p.quit:
			return
		}
	}
}

func (p *Pool[T]) worker(job Job[T]) {
	defer p.wg.Done()
	defer atomic.AddInt32(&p.activeWorkers, -1)
	defer func() {
		if job.CleanupFunc != nil {
			job.CleanupFunc()
		}
	}()
	logger := lg.FromContext(job.Ctx).With(lg.Any("job", job.Payload))
	//TODO: USE WITH
	logger.Info("Worker started with payload: %+v; # of workers: %d", 
							//lg.Any("job",job.Payload), 
							lg.Int32("workers", atomic.LoadInt32(&p.activeWorkers)))

	doneCh := make(chan error, 1)
	go func() {
		var err error
		for attempt := 1; attempt <= maxAttemps; attempt++ {
			err = job.Fn(job.Payload)
			if err == nil {
				doneCh <- nil
				return
			}
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		doneCh <- fmt.Errorf("failed after 3 attempts: %w", err)
	}()

	select {
	case <-job.Ctx.Done():
		//log.Printf("Job canceled with payload: %+v, reason: %v", job.Payload, job.Ctx.Err())
		logger.Info("Job canceled with payload: %+v, reason: %v", 
								//lg.Any("job",job.Payload), 
								lg.Any("ctx.error", job.Ctx.Err()))
	case err := <-doneCh:
		if err != nil {
			//log.Printf("Worker error with payload %+v: %v", job.Payload, err)
			logger.Info("Worker error with payload %+v: %v", 
								//lg.Any("job",job.Payload), 
								lg.Any("error", err))
		} else {
//			log.Printf("Worker finished job with payload: %+v; # of workers: %d",
//				job.Payload, atomic.LoadInt32(&p.activeWorkers))
			logger.Info(fmt.Sprintf("Worker finished for job %+v: %v ", 
								lg.Any("job",job.Payload), 
								lg.Int32("workers", atomic.LoadInt32(&p.activeWorkers))) )
		}
	}
}

func (p *Pool[T]) ActiveWorkers() int32 {
	return atomic.LoadInt32(&p.activeWorkers)
}
package workerpool

import (
	"log"
	"sync"
	"sync/atomic"
	"context"
)
const TotalMaxWorkers = 100

type JobFunc[T any] func(T) error

type Job[T any] struct{
	Payload T
	Fn		JobFunc[T]
	Ctx	context.Context
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

	select {
	case p.Jobs <- job:
		log.Printf("Job submitted with payload: %+v", job.Payload)
	case <-p.quit:
		log.Println("Worker pool is shutting down, job rejected")
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

	log.Printf("Worker started with payload: %+v; # of workers: %d", job.Payload, atomic.LoadInt32(&p.activeWorkers))

	doneCh := make(chan error, 1)

	go func() {
		doneCh <- job.Fn(job.Payload)
	}()

	select {
	case <-job.Ctx.Done():
		log.Printf("Job canceled with payload: %+v, reason: %v", job.Payload, job.Ctx.Err())
	case err := <-doneCh:
		if err != nil {
			log.Printf("Worker error with payload %+v: %v", job.Payload, err)
		} else {
			log.Printf("Worker finished for job with payload: %+v; # of workers: %d",
				job.Payload, atomic.LoadInt32(&p.activeWorkers))
		}
	}
}

func (p *Pool[T]) ActiveWorkers() int32 {
	return atomic.LoadInt32(&p.activeWorkers)
}
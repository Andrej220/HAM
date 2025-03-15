package main

import (
	"log"
	"sync"
	"sync/atomic"
)
const MAXWORKERS = 100

type workerFunction[T any] func(T) error

type WorkerPoolJob[T any] struct{
	Payload T
	fn		workerFunction[T]
}

type WorkerPool[T any] struct {
	Jobs         chan WorkerPoolJob[T]      
	activeWorkers int32         
	wg           sync.WaitGroup 
	quit         chan struct{}  
}

func NewWorkerPool[T any]() *WorkerPool[T] {
	pool := &WorkerPool[T]{
		Jobs:  make(chan WorkerPoolJob[T], MAXWORKERS), 
		quit:  make(chan struct{}),
	}
	go pool.dispatch() 
	return pool
}

func (wp *WorkerPool[T]) Stop() {
	close(wp.quit)
	wp.wg.Wait()
	close(wp.Jobs)
}

func (wp *WorkerPool[T]) Submit(job WorkerPoolJob[T]) {
	select {
	case wp.Jobs <- job:
		log.Printf("WorkerPoolJob submitted with payload: %+v", job.Payload)
	case <-wp.quit:
		log.Println("Worker pool is shutting down, job rejected")
	}
}

func (wp *WorkerPool[T]) dispatch() {
	for {
		select {
		case job := <-wp.Jobs:
			wp.wg.Add(1)
			atomic.AddInt32(&wp.activeWorkers, 1)
			go wp.worker(job)
		case <-wp.quit:
			return
		}
	}
}

func (wp *WorkerPool[T]) worker(job WorkerPoolJob[T]) {
	defer wp.wg.Done()
	defer atomic.AddInt32(&wp.activeWorkers, -1)

	log.Printf("Worker started with payload: %+v; # of workers: %d",job.Payload, atomic.LoadInt32(&wp.activeWorkers))

	// Execute the task
	err:=job.fn(job.Payload)
	if err!= nil{
		log.Printf("Worker started with payload %+v", job.Payload)
	}else{
		log.Printf("Worker finished for job with payload: %+v; # of workers: %d", job.Payload, atomic.LoadInt32(&wp.activeWorkers))
	}
}

func (wp *WorkerPool[T]) ActiveWorkers() int32 {
	return atomic.LoadInt32(&wp.activeWorkers)
}
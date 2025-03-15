package main

import (
	"log"
	"sync"
	"sync/atomic"
	"github.com/google/uuid"
)
const MAXWORKERS = 100

type workerFunction func(int, int, uuid.UUID) error

type Job struct {
	HostID   	int
	ScriptID 	int
	UUID   		uuid.UUID
	fn			workerFunction
}

type WorkerPool struct {
	Jobs         chan Job      
	activeWorkers int32         
	wg           sync.WaitGroup 
	quit         chan struct{}  
}

func NewWorkerPool() *WorkerPool {
	pool := &WorkerPool{
		Jobs:  make(chan Job, MAXWORKERS), 
		quit:  make(chan struct{}),
	}
	go pool.dispatch() 
	return pool
}

func (wp *WorkerPool) Stop() {
	close(wp.quit)
	wp.wg.Wait()
	close(wp.Jobs)
}

func (wp *WorkerPool) Submit(job Job) {
	select {
	case wp.Jobs <- job:
		log.Printf("Job submitted: HostID=%d, ScriptID=%d, UUID=%s", job.HostID, job.ScriptID, job.UUID)
	case <-wp.quit:
		log.Println("Worker pool is shutting down, job rejected")
	}
}

func (wp *WorkerPool) dispatch() {
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

func (wp *WorkerPool) worker(job Job) {
	defer wp.wg.Done()
	defer atomic.AddInt32(&wp.activeWorkers, -1)

	log.Printf("Worker started for job: HostID=%d, ScriptID=%d. Active workers: %d",
		job.HostID, job.ScriptID, atomic.LoadInt32(&wp.activeWorkers))

	// Execute the task
	err:=job.fn(job.HostID, job.ScriptID, job.UUID)
	if err!= nil{
		log.Printf("Worker started for job: HostID=%d, ScriptID=%d, UUID: %s",
		job.HostID, job.ScriptID, job.UUID)
	}else{
		log.Printf("Worker finished for job: HostID=%d, ScriptID=%d. Active workers: %d",
			job.HostID, job.ScriptID, atomic.LoadInt32(&wp.activeWorkers))
	}
}

func (wp *WorkerPool) ActiveWorkers() int32 {
	return atomic.LoadInt32(&wp.activeWorkers)
}
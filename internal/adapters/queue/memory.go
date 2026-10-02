package queue

import (
	"context"
	"sync"

	"gachaslop/internal/domain"
	"gachaslop/internal/ports"
)

type Memory struct {
	tasks chan ports.SyncTask
	once  sync.Once
}

func NewMemory(size int) *Memory {
	return &Memory{tasks: make(chan ports.SyncTask, size)}
}

func (q *Memory) Enqueue(task ports.SyncTask) error {
	select {
	case q.tasks <- task:
		return nil
	default:
		return domain.ErrQueueFull
	}
}

func (q *Memory) Start(ctx context.Context, workers int, handler func(context.Context, ports.SyncTask)) {
	q.once.Do(func() {
		for range workers {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case task := <-q.tasks:
						handler(ctx, task)
					}
				}
			}()
		}
	})
}

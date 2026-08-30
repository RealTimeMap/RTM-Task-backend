package task

import "context"

// Repository — порт хранилища задач. Домен знает только этот контракт,
// конкретный Postgres-адаптер живёт в infrastructure.
type Repository interface {
	Create(ctx context.Context, obj *Task) (*Task, error)
	GetByID(ctx context.Context, id uint) (*Task, error)
	List(ctx context.Context, filter Filter) ([]*Task, int64, error)
	Exists(ctx context.Context, id uint) (bool, error)

	// Update сохраняет задачу с проверкой оптимистичной блокировки:
	// запись обновляется только если её версия совпадает с expectedVersion.
	// При расхождении возвращается ErrVersionConflict.
	Update(ctx context.Context, obj *Task, expectedVersion int) (*Task, error)

	Delete(ctx context.Context, id uint) error

	// TodayCreated считает задачи, созданные сотрудником за сегодня —
	// нужно для суточного лимита.
	TodayCreated(ctx context.Context, creatorID uint) (int64, error)
}

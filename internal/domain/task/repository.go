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

// CommentRepository — порт хранилища комментариев.
//
// Отдельный порт, а не методы в Repository: комментарии не участвуют
// в оптимистичной блокировке задачи и живут своим циклом.
type CommentRepository interface {
	Create(ctx context.Context, obj *Comment) (*Comment, error)
	GetByID(ctx context.Context, id uint) (*Comment, error)

	// ListByTask возвращает обсуждение задачи в хронологическом порядке.
	ListByTask(ctx context.Context, taskID uint) ([]*Comment, error)

	// ListByTasks собирает комментарии сразу к набору задач — нужно,
	// чтобы список задач не превращался в N+1 запросов.
	ListByTasks(ctx context.Context, taskIDs []uint) (map[uint][]*Comment, error)

	Update(ctx context.Context, obj *Comment) (*Comment, error)
	Delete(ctx context.Context, id uint) error
}

// ChecklistRepository — порт хранилища пунктов чек-листа.
type ChecklistRepository interface {
	Create(ctx context.Context, obj *ChecklistItem) (*ChecklistItem, error)

	// CreateMany заводит пункты пачкой: при создании задачи чек-лист
	// приходит целиком, и построчная вставка была бы лишней.
	CreateMany(ctx context.Context, objs []*ChecklistItem) ([]*ChecklistItem, error)

	GetByID(ctx context.Context, id uint) (*ChecklistItem, error)
	ListByTask(ctx context.Context, taskID uint) ([]*ChecklistItem, error)
	ListByTasks(ctx context.Context, taskIDs []uint) (map[uint][]*ChecklistItem, error)

	// CountByTask нужен для проверки лимита пунктов.
	CountByTask(ctx context.Context, taskID uint) (int64, error)

	Update(ctx context.Context, obj *ChecklistItem) (*ChecklistItem, error)
	Delete(ctx context.Context, id uint) error

	// NextPosition возвращает позицию для нового пункта в конце списка.
	NextPosition(ctx context.Context, taskID uint) (int, error)
}

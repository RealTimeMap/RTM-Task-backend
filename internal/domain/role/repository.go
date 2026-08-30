package role

import "context"

// Repository — порт хранилища сотрудников. Реализация живёт в infrastructure.
type Repository interface {
	Create(ctx context.Context, staff *Staff) (*Staff, error)
	GetByID(ctx context.Context, id uint) (*Staff, error)
	GetByEmail(ctx context.Context, email string) (*Staff, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	Update(ctx context.Context, staff *Staff) (*Staff, error)
	List(ctx context.Context) ([]*Staff, error)
}

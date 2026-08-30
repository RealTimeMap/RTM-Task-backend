package task

import (
	"context"

	"go.uber.org/zap"
)

// fakeRepository — репозиторий в памяти для доменных тестов.
type fakeRepository struct {
	items       map[uint]*Task
	nextID      uint
	todayCount  int64
	createCalls int
	deleteCalls int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: make(map[uint]*Task), nextID: 1}
}

func (r *fakeRepository) put(obj *Task) *Task {
	if obj.ID == 0 {
		obj.ID = r.nextID
		r.nextID++
	}
	stored := *obj
	r.items[obj.ID] = &stored
	return obj
}

func (r *fakeRepository) Create(_ context.Context, obj *Task) (*Task, error) {
	r.createCalls++
	return r.put(obj), nil
}

func (r *fakeRepository) GetByID(_ context.Context, id uint) (*Task, error) {
	obj, ok := r.items[id]
	if !ok {
		return nil, ErrTaskNotFound(id)
	}
	copied := *obj
	return &copied, nil
}

func (r *fakeRepository) List(_ context.Context, _ Filter) ([]*Task, int64, error) {
	objs := make([]*Task, 0, len(r.items))
	for _, obj := range r.items {
		copied := *obj
		objs = append(objs, &copied)
	}
	return objs, int64(len(objs)), nil
}

func (r *fakeRepository) Exists(_ context.Context, id uint) (bool, error) {
	_, ok := r.items[id]
	return ok, nil
}

func (r *fakeRepository) Update(_ context.Context, obj *Task, expectedVersion int) (*Task, error) {
	stored, ok := r.items[obj.ID]
	if !ok {
		return nil, ErrTaskNotFound(obj.ID)
	}
	if stored.Version != expectedVersion {
		return nil, ErrVersionConflict(obj.ID)
	}
	copied := *obj
	r.items[obj.ID] = &copied
	result := copied
	return &result, nil
}

func (r *fakeRepository) Delete(_ context.Context, id uint) error {
	r.deleteCalls++
	if _, ok := r.items[id]; !ok {
		return ErrTaskNotFound(id)
	}
	delete(r.items, id)
	return nil
}

func (r *fakeRepository) TodayCreated(_ context.Context, _ uint) (int64, error) {
	return r.todayCount, nil
}

// fakeStaff — заглушка проверки сотрудников.
type fakeStaff struct {
	err error
}

func (s *fakeStaff) EnsureAssignable(_ context.Context, _ uint) error {
	return s.err
}

func newTestService(repo *fakeRepository, staff *fakeStaff) *Service {
	if staff == nil {
		staff = &fakeStaff{}
	}
	return NewService(repo, staff, zap.NewNop())
}

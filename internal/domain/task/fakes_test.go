package task

import (
	"context"
	"sort"

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

// fakeComments — хранилище комментариев в памяти.
type fakeComments struct {
	items  map[uint]*Comment
	nextID uint
}

func newFakeComments() *fakeComments {
	return &fakeComments{items: make(map[uint]*Comment), nextID: 1}
}

func (r *fakeComments) Create(_ context.Context, obj *Comment) (*Comment, error) {
	if obj.ID == 0 {
		obj.ID = r.nextID
		r.nextID++
	}
	stored := *obj
	r.items[obj.ID] = &stored
	return obj, nil
}

func (r *fakeComments) GetByID(_ context.Context, id uint) (*Comment, error) {
	obj, ok := r.items[id]
	if !ok {
		return nil, ErrCommentNotFound(id)
	}
	copied := *obj
	return &copied, nil
}

func (r *fakeComments) ListByTask(_ context.Context, taskID uint) ([]*Comment, error) {
	objs := make([]*Comment, 0)
	for _, obj := range r.items {
		if obj.TaskID == taskID {
			copied := *obj
			objs = append(objs, &copied)
		}
	}
	sortByID(objs, func(c *Comment) uint { return c.ID })
	return objs, nil
}

func (r *fakeComments) ListByTasks(ctx context.Context, taskIDs []uint) (map[uint][]*Comment, error) {
	grouped := make(map[uint][]*Comment, len(taskIDs))
	for _, id := range taskIDs {
		objs, err := r.ListByTask(ctx, id)
		if err != nil {
			return nil, err
		}
		if len(objs) > 0 {
			grouped[id] = objs
		}
	}
	return grouped, nil
}

func (r *fakeComments) Update(_ context.Context, obj *Comment) (*Comment, error) {
	if _, ok := r.items[obj.ID]; !ok {
		return nil, ErrCommentNotFound(obj.ID)
	}
	copied := *obj
	r.items[obj.ID] = &copied
	result := copied
	return &result, nil
}

func (r *fakeComments) Delete(_ context.Context, id uint) error {
	if _, ok := r.items[id]; !ok {
		return ErrCommentNotFound(id)
	}
	delete(r.items, id)
	return nil
}

// fakeChecklist — хранилище пунктов чек-листа в памяти.
type fakeChecklist struct {
	items  map[uint]*ChecklistItem
	nextID uint
}

func newFakeChecklist() *fakeChecklist {
	return &fakeChecklist{items: make(map[uint]*ChecklistItem), nextID: 1}
}

func (r *fakeChecklist) Create(_ context.Context, obj *ChecklistItem) (*ChecklistItem, error) {
	if obj.ID == 0 {
		obj.ID = r.nextID
		r.nextID++
	}
	stored := *obj
	r.items[obj.ID] = &stored
	return obj, nil
}

func (r *fakeChecklist) CreateMany(ctx context.Context, objs []*ChecklistItem) ([]*ChecklistItem, error) {
	for _, obj := range objs {
		if _, err := r.Create(ctx, obj); err != nil {
			return nil, err
		}
	}
	return objs, nil
}

func (r *fakeChecklist) GetByID(_ context.Context, id uint) (*ChecklistItem, error) {
	obj, ok := r.items[id]
	if !ok {
		return nil, ErrChecklistItemNotFound(id)
	}
	copied := *obj
	return &copied, nil
}

func (r *fakeChecklist) ListByTask(_ context.Context, taskID uint) ([]*ChecklistItem, error) {
	objs := make([]*ChecklistItem, 0)
	for _, obj := range r.items {
		if obj.TaskID == taskID {
			copied := *obj
			objs = append(objs, &copied)
		}
	}
	sortByID(objs, func(i *ChecklistItem) uint { return uint(i.Position) })
	return objs, nil
}

func (r *fakeChecklist) ListByTasks(ctx context.Context, taskIDs []uint) (map[uint][]*ChecklistItem, error) {
	grouped := make(map[uint][]*ChecklistItem, len(taskIDs))
	for _, id := range taskIDs {
		objs, err := r.ListByTask(ctx, id)
		if err != nil {
			return nil, err
		}
		if len(objs) > 0 {
			grouped[id] = objs
		}
	}
	return grouped, nil
}

func (r *fakeChecklist) CountByTask(ctx context.Context, taskID uint) (int64, error) {
	objs, err := r.ListByTask(ctx, taskID)
	if err != nil {
		return 0, err
	}
	return int64(len(objs)), nil
}

func (r *fakeChecklist) Update(_ context.Context, obj *ChecklistItem) (*ChecklistItem, error) {
	if _, ok := r.items[obj.ID]; !ok {
		return nil, ErrChecklistItemNotFound(obj.ID)
	}
	copied := *obj
	r.items[obj.ID] = &copied
	result := copied
	return &result, nil
}

func (r *fakeChecklist) Delete(_ context.Context, id uint) error {
	if _, ok := r.items[id]; !ok {
		return ErrChecklistItemNotFound(id)
	}
	delete(r.items, id)
	return nil
}

func (r *fakeChecklist) NextPosition(ctx context.Context, taskID uint) (int, error) {
	objs, err := r.ListByTask(ctx, taskID)
	if err != nil {
		return 0, err
	}
	next := 0
	for _, obj := range objs {
		if obj.Position >= next {
			next = obj.Position + 1
		}
	}
	return next, nil
}

// sortByID упорядочивает выборку из map: обход карты случаен, а тесты
// проверяют порядок записей.
func sortByID[T any](objs []T, key func(T) uint) {
	sort.Slice(objs, func(i, j int) bool { return key(objs[i]) < key(objs[j]) })
}

// testService — сервис со всеми хранилищами в памяти.
type testService struct {
	*Service

	repo      *fakeRepository
	comments  *fakeComments
	checklist *fakeChecklist
}

func newTestService(repo *fakeRepository, staff *fakeStaff) *Service {
	return newTestServiceFull(repo, staff).Service
}

func newTestServiceFull(repo *fakeRepository, staff *fakeStaff) testService {
	if staff == nil {
		staff = &fakeStaff{}
	}
	comments := newFakeComments()
	checklist := newFakeChecklist()

	return testService{
		Service:   NewService(repo, comments, checklist, staff, zap.NewNop()),
		repo:      repo,
		comments:  comments,
		checklist: checklist,
	}
}

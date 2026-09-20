package task_action

import (
	"context"
	"time"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

// BugReader — часть домена, отдающая перечень багов для привязки.
type BugReader interface {
	ListOpenBugs(ctx context.Context, filter task.BugFilter) ([]task.Bug, error)

	// GetBug отдаёт подробности бага, привязанного к задаче.
	GetBug(ctx context.Context, taskID uint) (task.Bug, error)
}

// BugLinker — часть домена, управляющая привязкой бага к задаче.
type BugLinker interface {
	AttachBug(ctx context.Context, actor role.Actor, id, bugID uint) (*task.Task, error)
	DetachBug(ctx context.Context, actor role.Actor, id uint) (*task.Task, error)
}

// BugResult — баг в перечне для привязки.
type BugResult struct {
	ID          uint
	Title       string
	Description string
	Tag         string
	Status      string
	Build       string
	HasLogs     bool
	CreatedAt   time.Time

	// Обстановка, в которой баг воспроизвёлся.
	Platform   string
	OS         string
	Resolution string
	Width      int
	Height     int
	Battery    *float64

	// Logs заполняется только при чтении одного бага.
	Logs []string

	// ReporterID — кто прислал отчёт. Пусто, если баг анонимный.
	ReporterID *uint
}

func toBugResult(obj task.Bug) BugResult {
	return BugResult{
		ID:          obj.ID,
		Title:       obj.Title,
		Description: obj.Description,
		Tag:         obj.Tag,
		Status:      obj.Status,
		Platform:    obj.Platform,
		Build:       obj.Build,
		HasLogs:     obj.HasLogs,
		CreatedAt:   obj.CreatedAt,
		OS:          obj.OS,
		Resolution:  obj.Resolution,
		Width:       obj.Width,
		Height:      obj.Height,
		Battery:     obj.Battery,
		Logs:        obj.Logs,
		ReporterID:  obj.ReporterID,
	}
}

func toBugResults(objs []task.Bug) []BugResult {
	results := make([]BugResult, 0, len(objs))
	for _, obj := range objs {
		results = append(results, toBugResult(obj))
	}
	return results
}

type ListBugsQuery struct {
	Tag   string
	Limit int
}

// GetBugQuery — запрос подробностей бага, привязанного к задаче.
type GetBugQuery struct {
	TaskID uint
}

type AttachBugCommand struct {
	Actor  role.Actor
	TaskID uint
	BugID  uint
}

func (c AttachBugCommand) Validate() error {
	if c.BugID == 0 {
		return apperror.NewRequiredError("bugId")
	}
	return nil
}

type DetachBugCommand struct {
	Actor  role.Actor
	TaskID uint
}

// BugHandler обслуживает работу с багами: перечень для привязки и саму
// привязку. Один обработчик на три операции — у них общий домен и общий
// смысл, а разносить их значило бы трижды повторить конструктор.
type BugHandler struct {
	bugs      BugReader
	tasks     BugLinker
	summaries TaskSummarizer
	publisher EventPublisher
	logger    *zap.Logger
}

func NewBugHandler(
	bugs BugReader,
	tasks BugLinker,
	summaries TaskSummarizer,
	publisher EventPublisher,
	logger *zap.Logger,
) *BugHandler {
	return &BugHandler{
		bugs:      bugs,
		tasks:     tasks,
		summaries: summaries,
		publisher: publisher,
		logger:    logger,
	}
}

// List отдаёт перечень багов, которые можно взять в работу.
func (h *BugHandler) List(ctx context.Context, query ListBugsQuery) ([]BugResult, error) {
	objs, err := h.bugs.ListOpenBugs(ctx, task.BugFilter{
		Tag:   query.Tag,
		Limit: query.Limit,
	})
	if err != nil {
		return nil, err
	}
	return toBugResults(objs), nil
}

// Get отдаёт подробности бага, над которым идёт работа в задаче.
func (h *BugHandler) Get(ctx context.Context, query GetBugQuery) (BugResult, error) {
	obj, err := h.bugs.GetBug(ctx, query.TaskID)
	if err != nil {
		return BugResult{}, err
	}
	return toBugResult(obj), nil
}

func (h *BugHandler) Attach(ctx context.Context, cmd AttachBugCommand) (TaskResult, error) {
	if err := cmd.Validate(); err != nil {
		return TaskResult{}, err
	}

	obj, err := h.tasks.AttachBug(ctx, cmd.Actor, cmd.TaskID, cmd.BugID)
	if err != nil {
		return TaskResult{}, err
	}

	result := h.withSummary(ctx, obj)
	publish(ctx, h.publisher, TaskEvent{Name: EventTaskUpdated, Task: result})
	// Баг ушёл из свободных — перечень у всех, кто его смотрит, устарел.
	publishBugsChanged(ctx, h.publisher)

	return result, nil
}

func (h *BugHandler) Detach(ctx context.Context, cmd DetachBugCommand) (TaskResult, error) {
	obj, err := h.tasks.DetachBug(ctx, cmd.Actor, cmd.TaskID)
	if err != nil {
		return TaskResult{}, err
	}

	result := h.withSummary(ctx, obj)
	publish(ctx, h.publisher, TaskEvent{Name: EventTaskUpdated, Task: result})
	// Освобождённый баг вернулся в разбор — перечень снова другой.
	publishBugsChanged(ctx, h.publisher)

	return result, nil
}

// withSummary дополняет задачу счётчиками вложенных записей: карточка
// на доске не должна терять прогресс чек-листа из-за привязки бага.
func (h *BugHandler) withSummary(ctx context.Context, obj *task.Task) TaskResult {
	result := toTaskResult(obj)
	if h.summaries == nil {
		return result
	}

	summaries, err := h.summaries.SummaryFor(ctx, []*task.Task{obj})
	if err != nil {
		h.logger.Warn("load task summary failed",
			zap.Uint("task_id", obj.ID),
			zap.Error(err),
		)
		return result
	}
	return result.withSummary(summaries[obj.ID])
}

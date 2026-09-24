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

// BugReviewer — часть домена, через которую разработчик выносит решение
// по отчёту: подтверждает, отклоняет или возвращает на проверку.
type BugReviewer interface {
	ConfirmBug(ctx context.Context, actor role.Actor, review task.BugReview) (task.Bug, error)
	RejectBug(ctx context.Context, actor role.Actor, review task.BugReview) (task.Bug, error)
	ReopenBug(ctx context.Context, actor role.Actor, bugID uint) (task.Bug, error)
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

	// Итог проверки разработчиком. Пусто, пока отчёт не проверен.
	ReviewedAt    *time.Time
	RejectReason  string
	ReviewComment string
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

		ReviewedAt:    obj.ReviewedAt,
		RejectReason:  string(obj.RejectReason),
		ReviewComment: obj.ReviewComment,
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
	Tag string
	// Status — очередь проверки (new) или отклонённые (rejected).
	// Пусто — подтверждённые баги, которые можно взять в задачу.
	Status string
	Limit  int
}

// ReviewBugCommand — решение по отчёту. Reason нужен только отклонению.
type ReviewBugCommand struct {
	Actor   role.Actor
	BugID   uint
	Reason  string
	Comment string
}

type ReopenBugCommand struct {
	Actor role.Actor
	BugID uint
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

// BugHandler обслуживает работу с багами: перечень, проверку отчётов и
// привязку к задачам. Один обработчик на все операции — у них общий
// домен и общий смысл, а разносить их значило бы повторять конструктор.
type BugHandler struct {
	bugs      BugReader
	reviewer  BugReviewer
	tasks     BugLinker
	summaries TaskSummarizer
	publisher EventPublisher
	logger    *zap.Logger
}

func NewBugHandler(
	bugs BugReader,
	reviewer BugReviewer,
	tasks BugLinker,
	summaries TaskSummarizer,
	publisher EventPublisher,
	logger *zap.Logger,
) *BugHandler {
	return &BugHandler{
		bugs:      bugs,
		reviewer:  reviewer,
		tasks:     tasks,
		summaries: summaries,
		publisher: publisher,
		logger:    logger,
	}
}

// List отдаёт перечень свободных багов: готовых к работе, ждущих
// проверки или отклонённых — по Status.
func (h *BugHandler) List(ctx context.Context, query ListBugsQuery) ([]BugResult, error) {
	objs, err := h.bugs.ListOpenBugs(ctx, task.BugFilter{
		Tag:    query.Tag,
		Status: query.Status,
		Limit:  query.Limit,
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

// Confirm фиксирует, что баг воспроизвёлся: он уходит из очереди
// проверки в перечень готовых к работе.
func (h *BugHandler) Confirm(ctx context.Context, cmd ReviewBugCommand) (BugResult, error) {
	obj, err := h.reviewer.ConfirmBug(ctx, cmd.Actor, task.BugReview{
		BugID:   cmd.BugID,
		Comment: cmd.Comment,
	})
	return h.reviewed(ctx, obj, err)
}

// Reject фиксирует, что проверка баг не подтвердила.
func (h *BugHandler) Reject(ctx context.Context, cmd ReviewBugCommand) (BugResult, error) {
	obj, err := h.reviewer.RejectBug(ctx, cmd.Actor, task.BugReview{
		BugID:   cmd.BugID,
		Reason:  task.BugRejectReason(cmd.Reason),
		Comment: cmd.Comment,
	})
	return h.reviewed(ctx, obj, err)
}

// Reopen возвращает баг на повторную проверку.
func (h *BugHandler) Reopen(ctx context.Context, cmd ReopenBugCommand) (BugResult, error) {
	obj, err := h.reviewer.ReopenBug(ctx, cmd.Actor, cmd.BugID)
	return h.reviewed(ctx, obj, err)
}

// reviewed завершает решение по отчёту: баг сменил перечень, и у всех,
// кто смотрит на очередь или на готовые к работе, картина устарела.
func (h *BugHandler) reviewed(ctx context.Context, obj task.Bug, err error) (BugResult, error) {
	if err != nil {
		return BugResult{}, err
	}
	publishBugsChanged(ctx, h.publisher)
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

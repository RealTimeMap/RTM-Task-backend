package task

import (
	"time"

	"RTM-Task/internal/app/use_cases/task_action"
)

// TaskResponse — представление задачи в API.
type TaskResponse struct {
	ID          uint       `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	Project     string     `json:"project"`
	CreatorID   uint       `json:"creatorId"`
	AssigneeID  *uint      `json:"assigneeId"`
	Version     int        `json:"version"`
	ClosedAt    *time.Time `json:"closedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`

	// Замечание к доработке: пусто, пока задачу не возвращали или пока
	// её не приняли заново.
	ReworkNote string     `json:"reworkNote,omitempty"`
	ReworkByID *uint      `json:"reworkById,omitempty"`
	ReworkAt   *time.Time `json:"reworkAt,omitempty"`

	// BugID — баг из feedback-service, над которым идёт работа.
	// Заполнен только у задач типа bug.
	BugID *uint `json:"bugId,omitempty"`

	// Сводка по вложенным записям: карточка показывает прогресс и число
	// реплик, не загружая их содержимое. Сами списки — отдельными ручками.
	//
	// omitempty вместе с указателем: у операций без сводки (смена
	// статуса, назначение) полей в ответе не будет вовсе, и клиент
	// оставит уже известные ему значения. Ноль здесь означал бы
	// «записей нет» и погасил бы счётчики на карточке.
	ChecklistTotal *int `json:"checklistTotal,omitempty"`
	ChecklistDone  *int `json:"checklistDone,omitempty"`
	CommentCount   *int `json:"commentCount,omitempty"`
}

func NewTaskResponse(result task_action.TaskResult) TaskResponse {
	return TaskResponse{
		ID:          result.ID,
		Title:       result.Title,
		Description: result.Description,
		Type:        result.Type,
		Status:      result.Status,
		Priority:    result.Priority,
		Project:     result.Project,
		CreatorID:   result.CreatorID,
		AssigneeID:  result.AssigneeID,
		Version:     result.Version,
		ClosedAt:    result.ClosedAt,
		CreatedAt:   result.CreatedAt,
		UpdatedAt:   result.UpdatedAt,
		ReworkNote:  result.ReworkNote,
		ReworkByID:  result.ReworkByID,
		ReworkAt:    result.ReworkAt,
		BugID:       result.BugID,

		ChecklistTotal: result.ChecklistTotal,
		ChecklistDone:  result.ChecklistDone,
		CommentCount:   result.CommentCount,
	}
}

// TaskListResponse — страница задач с метаданными пагинации.
type TaskListResponse struct {
	Items  []TaskResponse `json:"items"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func NewTaskListResponse(result task_action.TaskListResult) TaskListResponse {
	items := make([]TaskResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, NewTaskResponse(item))
	}
	return TaskListResponse{
		Items:  items,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	}
}

// BugResponse — баг в перечне: для привязки к задаче или для проверки.
type BugResponse struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Tag         string    `json:"tag"`
	Status      string    `json:"status"`
	Platform    string    `json:"platform,omitempty"`
	Build       string    `json:"build,omitempty"`
	HasLogs     bool      `json:"hasLogs"`
	CreatedAt   time.Time `json:"createdAt"`

	// Итог проверки разработчиком. Есть в перечне, а не только в
	// карточке: отклонённые без причины не разобрать, а пояснение к
	// подтверждённому нужно ещё до того, как баг возьмут в работу.
	ReviewedAt    *time.Time `json:"reviewedAt,omitempty"`
	RejectReason  string     `json:"rejectReason,omitempty"`
	ReviewComment string     `json:"reviewComment,omitempty"`
}

// BugDetailResponse — баг целиком: обстановка воспроизведения и логи.
//
// Отдельно от карточки перечня: эти поля нужны тому, кто уже открыл
// задачу и разбирается в отчёте, а в списке они были бы лишним весом.
type BugDetailResponse struct {
	BugResponse

	OS         string   `json:"os,omitempty"`
	Resolution string   `json:"resolution,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	Battery    *float64 `json:"battery,omitempty"`

	// ReporterID — кто прислал отчёт. Пусто, если баг анонимный.
	ReporterID *uint `json:"reporterId,omitempty"`

	// Logs — журнал приложения на момент отправки отчёта.
	Logs []string `json:"logs"`
}

func NewBugDetailResponse(result task_action.BugResult) BugDetailResponse {
	return BugDetailResponse{
		BugResponse: NewBugResponse(result),
		OS:          result.OS,
		Resolution:  result.Resolution,
		Width:       result.Width,
		Height:      result.Height,
		Battery:     result.Battery,
		ReporterID:  result.ReporterID,
		// Пустой срез вместо nil: клиенту не нужно отдельно разбирать
		// случай «логов нет» — он получает [] и рисует пустой список.
		Logs: append([]string{}, result.Logs...),
	}
}

func NewBugResponse(result task_action.BugResult) BugResponse {
	return BugResponse{
		ID:          result.ID,
		Title:       result.Title,
		Description: result.Description,
		Tag:         result.Tag,
		Status:      result.Status,
		Platform:    result.Platform,
		Build:       result.Build,
		HasLogs:     result.HasLogs,
		CreatedAt:   result.CreatedAt,

		ReviewedAt:    result.ReviewedAt,
		RejectReason:  result.RejectReason,
		ReviewComment: result.ReviewComment,
	}
}

// BugListResponse — перечень багов.
type BugListResponse struct {
	Items []BugResponse `json:"items"`
	Total int           `json:"total"`
}

func NewBugListResponse(results []task_action.BugResult) BugListResponse {
	items := make([]BugResponse, 0, len(results))
	for _, result := range results {
		items = append(items, NewBugResponse(result))
	}
	return BugListResponse{Items: items, Total: len(items)}
}

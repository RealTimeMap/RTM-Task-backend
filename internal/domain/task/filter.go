package task

import "RTM-Task/internal/utils/pagination"

// Filter — критерии выборки задач. Nil-поле означает «не фильтровать».
type Filter struct {
	Status     *Status
	Type       *Type
	Priority   *Priority
	CreatorID  *uint
	AssigneeID *uint

	// OnlyUnassigned отбирает задачи без исполнителя.
	OnlyUnassigned bool

	Pagination pagination.Params
}

// SortField — поле сортировки выборки.
type SortField string

const (
	SortByCreatedAt SortField = "created_at"
	SortByPriority  SortField = "priority"
	SortByStatus    SortField = "status"
)

func (f SortField) IsValid() bool {
	switch f {
	case SortByCreatedAt, SortByPriority, SortByStatus:
		return true
	default:
		return false
	}
}

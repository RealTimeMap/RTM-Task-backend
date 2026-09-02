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

	// Sort задаёт порядок выборки. Нулевое значение означает порядок
	// по умолчанию — сначала важные, внутри приоритета сначала свежие.
	Sort Sort

	Pagination pagination.Params
}

// SortField — поле сортировки выборки.
type SortField string

const (
	SortByCreatedAt SortField = "createdAt"
	SortByPriority  SortField = "priority"
	SortByStatus    SortField = "status"
	SortByType      SortField = "type"
)

func (f SortField) IsValid() bool {
	switch f {
	case SortByCreatedAt, SortByPriority, SortByStatus, SortByType:
		return true
	default:
		return false
	}
}

func (f SortField) String() string { return string(f) }

// SortOrder — направление сортировки.
type SortOrder string

const (
	AscOrder  SortOrder = "asc"
	DescOrder SortOrder = "desc"
)

func (o SortOrder) IsValid() bool {
	return o == AscOrder || o == DescOrder
}

func (o SortOrder) String() string { return string(o) }

// Sort — порядок выборки задач.
//
// Значение по умолчанию (нулевая структура) означает «как раньше»:
// сначала важные, внутри приоритета — сначала свежие. Это сохраняет
// поведение для клиентов, которые про сортировку ничего не знают.
type Sort struct {
	Field SortField
	Order SortOrder
}

// IsZero сообщает, что порядок не задан и нужен тот, что по умолчанию.
func (s Sort) IsZero() bool { return s.Field == "" }

// Normalize приводит порядок к пригодному для запроса виду.
// Пустое направление берётся из естественного для поля: даты читают
// от свежих к старым, а приоритет и статус — от начала шкалы.
func (s Sort) Normalize() Sort {
	if s.IsZero() {
		return Sort{}
	}
	if !s.Order.IsValid() {
		s.Order = s.Field.defaultOrder()
	}
	return s
}

// defaultOrder — направление, в котором поле читается естественно.
func (f SortField) defaultOrder() SortOrder {
	// Свежие задачи интереснее старых, поэтому дата по умолчанию убывает.
	// Приоритет, статус и тип — шкалы с осмысленным началом (важное,
	// новое, первый тип), их читают по возрастанию.
	if f == SortByCreatedAt {
		return DescOrder
	}
	return AscOrder
}

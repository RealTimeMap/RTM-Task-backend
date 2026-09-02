package task

// CreateTaskRequest — тело запроса на создание задачи.
type CreateTaskRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type" binding:"required,oneof=bug feature fix refactor update"`
	Priority    int    `json:"priority" binding:"omitempty,oneof=10 20 30 40"`
	AssigneeID  *uint  `json:"assigneeId"`

	// Checklist — заготовка списка дел прямо из формы создания.
	// Пустые строки сервер отбрасывает: незаполненное поле формы
	// не должно превращаться в пустой пункт.
	Checklist []string `json:"checklist" binding:"omitempty,max=50,dive,max=300"`
}

// UpdateTaskRequest — частичное обновление задачи: nil-поле означает
// «не менять», поэтому все поля указателями.
type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Type        *string `json:"type" binding:"omitempty,oneof=bug feature fix refactor update"`
	Priority    *int    `json:"priority" binding:"omitempty,oneof=10 20 30 40"`
}

// ChangeStatusRequest — тело запроса на смену статуса.
type ChangeStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=new working review complete"`
}

// SendToReworkRequest — тело запроса на возврат завершённой задачи
// в работу. Замечание обязательно: без него исполнитель не поймёт,
// что именно от него хотят.
type SendToReworkRequest struct {
	Note string `json:"note" binding:"required"`
}

// AssignTaskRequest — тело запроса на назначение исполнителя.
type AssignTaskRequest struct {
	AssigneeID uint `json:"assigneeId" binding:"required"`
}

// ListTasksQuery — параметры выборки задач из query string.
type ListTasksQuery struct {
	Status     *string `form:"status" binding:"omitempty,oneof=new working review complete"`
	Type       *string `form:"type" binding:"omitempty,oneof=bug feature fix refactor update"`
	Priority   *int    `form:"priority" binding:"omitempty,oneof=10 20 30 40"`
	CreatorID  *uint   `form:"creatorId"`
	AssigneeID *uint   `form:"assigneeId"`
	Unassigned bool    `form:"unassigned"`

	// Сортировка: поле и направление. Пустые — порядок по умолчанию.
	Sort  *string `form:"sort" binding:"omitempty,oneof=createdAt priority status type"`
	Order *string `form:"order" binding:"omitempty,oneof=asc desc"`

	Limit  int `form:"limit"`
	Offset int `form:"offset"`
}

package task

// CreateTaskRequest — тело запроса на создание задачи.
type CreateTaskRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type" binding:"required,oneof=bug feature fix"`
	Priority    int    `json:"priority" binding:"omitempty,oneof=10 20 30 40"`
	AssigneeID  *uint  `json:"assigneeId"`
}

// UpdateTaskRequest — частичное обновление задачи: nil-поле означает
// «не менять», поэтому все поля указателями.
type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Type        *string `json:"type" binding:"omitempty,oneof=bug feature fix"`
	Priority    *int    `json:"priority" binding:"omitempty,oneof=10 20 30 40"`
}

// ChangeStatusRequest — тело запроса на смену статуса.
type ChangeStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=new working review complete"`
}

// AssignTaskRequest — тело запроса на назначение исполнителя.
type AssignTaskRequest struct {
	AssigneeID uint `json:"assigneeId" binding:"required"`
}

// ListTasksQuery — параметры выборки задач из query string.
type ListTasksQuery struct {
	Status     *string `form:"status" binding:"omitempty,oneof=new working review complete"`
	Type       *string `form:"type" binding:"omitempty,oneof=bug feature fix"`
	Priority   *int    `form:"priority" binding:"omitempty,oneof=10 20 30 40"`
	CreatorID  *uint   `form:"creatorId"`
	AssigneeID *uint   `form:"assigneeId"`
	Unassigned bool    `form:"unassigned"`
	Limit      int     `form:"limit"`
	Offset     int     `form:"offset"`
}

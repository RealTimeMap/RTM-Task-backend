package task_action

// Application собирает use case'ы работы с задачами в одну точку входа
// для транспортного слоя.
type Application struct {
	CreateTask   *CreateTaskHandler
	GetTask      *GetTaskHandler
	ListTasks    *ListTasksHandler
	UpdateTask   *UpdateTaskHandler
	ChangeStatus *ChangeStatusHandler
	SendToRework *SendToReworkHandler
	AssignTask   *AssignTaskHandler
	UnassignTask *UnassignTaskHandler
	DeleteTask   *DeleteTaskHandler
}

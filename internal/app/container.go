package app

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"RTM-Task/internal/app/use_cases/idea_action"
	"RTM-Task/internal/app/use_cases/staff_action"
	"RTM-Task/internal/app/use_cases/task_action"
	"RTM-Task/internal/config"
	"RTM-Task/internal/domain/idea"
	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/infrastructure/feedback"
	"RTM-Task/internal/infrastructure/notify"
	"RTM-Task/internal/infrastructure/persistence/postgres"
	"RTM-Task/internal/transport/socket"
)

// Container — точка сборки зависимостей: инфраструктура → домен → use cases.
type Container struct {
	Config *config.Config
	DB     *gorm.DB
	Logger *zap.Logger

	// Доменный сервис сотрудников нужен транспорту напрямую:
	// по нему auth разрешает участника операции и в HTTP, и в сокете.
	StaffService *role.Service

	TaskUseCases  *task_action.Application
	StaffUseCases *staff_action.Application
	IdeaUseCases  *idea_action.Application

	Socket *socket.Server
}

func MustContainer(cfg *config.Config, db *gorm.DB, log *zap.Logger) *Container {
	// Инфраструктура: адаптеры портов домена.
	staffRepo := postgres.NewStaffRepository(db, log)
	taskRepo := postgres.NewTaskRepository(db, log)
	commentRepo := postgres.NewCommentRepository(db, log)
	checklistRepo := postgres.NewChecklistRepository(db, log)
	ideaRepo := postgres.NewIdeaRepository(db, log)
	ideaCommentRepo := postgres.NewIdeaCommentRepository(db, log)

	// Каталог багов feedback-service. Порт остаётся nil, если интеграция
	// не настроена: домен это допускает — перечень багов пуст, а привязка
	// отклоняется, задачи же работают как обычно.
	var bugCatalog task.BugCatalog
	if cfg.Feedback.Enabled() {
		bugCatalog = feedback.NewClient(feedback.Config{
			BaseURL: cfg.Feedback.BaseURL,
			ApiKey:  cfg.Feedback.ApiKey,
			Timeout: cfg.Feedback.Timeout,
		}, log)
		log.Info("bug catalog enabled", zap.String("base_url", cfg.Feedback.BaseURL))
	} else {
		log.Info("bug catalog disabled: base_url or api_key is empty")
	}

	// Домен: сервисы, знающие только о своих портах.
	staffService := role.NewService(staffRepo, log)
	taskService := task.NewService(taskRepo, commentRepo, checklistRepo, staffService, bugCatalog, log)

	// Socket-сервер и publisher замкнуты друг на друга: use case'ы публикуют
	// события через publisher, а publisher рассылает их сокетам, которые
	// обслуживает сервер, собранный поверх тех же use case'ов.
	// Цикл разрывается поздним связыванием: publisher получает ссылку на
	// сервер, а сервер — готовые use case'ы уже после их сборки.
	socketServer := socket.NewServer(socket.Deps{
		Staff:  staffService,
		Logger: log,
	})
	publisher := socket.NewPublisher(socketServer, log)

	// Уведомления по почте. Порт остаётся nil, если сервис не настроен:
	// use case это допускает и просто не отправляет письма.
	var notifier task_action.Notifier
	if cfg.SMTP.Enabled() {
		notifier = notify.NewSMTPNotifier(notify.Config{
			BaseURL: cfg.SMTP.BaseURL,
			ApiKey:  cfg.SMTP.ApiKey,
			Timeout: cfg.SMTP.Timeout,
			AppURL:  cfg.SMTP.AppURL,
		}, log)
		log.Info("smtp notifier enabled", zap.String("base_url", cfg.SMTP.BaseURL))
	} else {
		log.Info("smtp notifier disabled: base_url or api_key is empty")
	}

	// Application: use case'ы поверх доменных сервисов.
	taskUseCases := &task_action.Application{
		CreateTask:   task_action.NewCreateTaskHandler(taskService, staffService, taskService, publisher, notifier, log),
		GetTask:      task_action.NewGetTaskHandler(taskService, taskService, log),
		ListTasks:    task_action.NewListTasksHandler(taskService, taskService, log),
		UpdateTask:   task_action.NewUpdateTaskHandler(taskService, publisher, log),
		ChangeStatus: task_action.NewChangeStatusHandler(taskService, publisher, log),
		SendToRework: task_action.NewSendToReworkHandler(taskService, staffService, publisher, notifier, log),
		AssignTask:   task_action.NewAssignTaskHandler(taskService, staffService, publisher, notifier, log),
		UnassignTask: task_action.NewUnassignTaskHandler(taskService, publisher, log),
		DeleteTask:   task_action.NewDeleteTaskHandler(taskService, publisher, log),

		Bugs: task_action.NewBugHandler(taskService, taskService, taskService, taskService, publisher, log),

		Comments:  task_action.NewCommentHandler(taskService, publisher, log),
		Checklist: task_action.NewChecklistHandler(taskService, publisher, log),
	}

	// Идеи не участвуют в realtime и уведомлениях: копилка замыслов
	// не требует, чтобы о ней узнавали в ту же секунду.
	ideaService := idea.NewService(ideaRepo, ideaCommentRepo, log)
	ideaUseCases := idea_action.NewApplication(ideaService, publisher, log)

	staffUseCases := &staff_action.Application{
		GetStaff:   staff_action.NewGetStaffHandler(staffService, log),
		ListStaff:  staff_action.NewListStaffHandler(staffService, log),
		ChangeRole: staff_action.NewChangeRoleHandler(staffService, log),
		Deactivate: staff_action.NewDeactivateStaffHandler(staffService, log),
	}

	// Теперь, когда use case'ы собраны, сервер сокетов может их обслуживать.
	socketServer.Attach(taskUseCases)

	return &Container{
		Config:        cfg,
		DB:            db,
		Logger:        log,
		StaffService:  staffService,
		TaskUseCases:  taskUseCases,
		StaffUseCases: staffUseCases,
		IdeaUseCases:  ideaUseCases,
		Socket:        socketServer,
	}
}

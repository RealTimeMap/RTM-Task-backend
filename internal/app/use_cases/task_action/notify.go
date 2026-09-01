package task_action

import (
	"context"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

// AssignmentNotice — письмо о том, что задача назначена на сотрудника.
//
// Тип принадлежит application-слою: он описывает повод для уведомления,
// а не способ доставки. Как именно письмо уйдёт — забота адаптера.
type AssignmentNotice struct {
	Recipient role.Staff
	Task      task.Task
}

// ReworkNotice — письмо о том, что завершённую задачу вернули в работу.
// Замечание передаётся отдельным полем: получателю важно не то, что
// статус сменился, а что именно нужно доделать.
type ReworkNotice struct {
	Recipient role.Staff
	Task      task.Task
	Note      string
}

// Notifier — порт уведомлений.
//
// Реализуется инфраструктурой (сейчас — клиентом smtp-service).
// nil допустим: тогда уведомления просто не отправляются, и это не мешает
// работать с задачами.
type Notifier interface {
	NotifyAssignment(ctx context.Context, notice AssignmentNotice)
	NotifyRework(ctx context.Context, notice ReworkNotice)
}

// notifyAssignment отправляет уведомление, если notifier подключён.
//
// Доставка не влияет на результат операции: задача уже назначена, и отказ
// почтового сервиса не повод возвращать ошибку пользователю.
func notifyAssignment(ctx context.Context, notifier Notifier, notice AssignmentNotice) {
	if notifier == nil {
		return
	}
	notifier.NotifyAssignment(ctx, notice)
}

// notifyRework отправляет уведомление о доработке, если notifier подключён.
func notifyRework(ctx context.Context, notifier Notifier, notice ReworkNotice) {
	if notifier == nil {
		return
	}
	notifier.NotifyRework(ctx, notice)
}

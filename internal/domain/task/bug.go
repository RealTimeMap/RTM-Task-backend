package task

import (
	"context"
	"time"
)

// Bug — баг из feedback-service в объёме, нужном таск-менеджеру.
//
// Собственный тип, а не структура чужого сервиса: домен задач знает о
// баге ровно то, что показывает в перечне и подставляет в задачу, и не
// должен меняться вслед за полями чужой модели.
type Bug struct {
	ID          uint
	Title       string
	Description string
	Tag         string
	Status      string
	Build       string
	HasLogs     bool
	CreatedAt   time.Time

	// Обстановка, в которой баг воспроизвёлся. Без неё отчёт часто
	// невоспроизводим: одна и та же ошибка живёт не на всех версиях ОС
	// и не на всех разрешениях.
	Platform   string
	OS         string
	Resolution string
	Width      int
	Height     int
	Battery    *float64

	// Logs — журнал приложения. Заполняется только при чтении одного
	// бага: в перечне логи были бы лишним весом.
	Logs []string

	// ReporterID — пользователь, приславший отчёт. Пусто, если баг
	// отправлен анонимно.
	ReporterID *uint

	// TaskID — задача, в которой баг уже ведут. Пусто, пока баг свободен.
	TaskID *uint
}

// BugSync — статус бага, соответствующий состоянию задачи.
//
// Значения принадлежат feedback-service, поэтому это строки, а не
// доменный тип: единственный, кто их толкует, — адаптер на той стороне.
type BugSync string

const (
	// BugInWork — над багом идёт работа.
	BugInWork BugSync = "in work"
	// BugClosed — работа завершена.
	BugClosed BugSync = "closed"
	// BugNew — баг снова свободен и ждёт, когда его возьмут.
	BugNew BugSync = "new"
)

// BugCatalog — порт каталога багов.
//
// Реализуется клиентом feedback-service. nil допустим: тогда перечень
// багов пуст, а привязка отклоняется — сервис задач продолжает работать
// без интеграции.
type BugCatalog interface {
	// ListOpen возвращает баги, которые можно взять в задачу:
	// незакрытые и ещё не занятые другой задачей.
	ListOpen(ctx context.Context, filter BugFilter) ([]Bug, error)

	// Get отдаёт один баг целиком — с логами и обстановкой
	// воспроизведения. Их показывает карточка задачи.
	Get(ctx context.Context, bugID uint) (Bug, error)

	// Link отмечает баг как взятый в работу указанной задачей.
	Link(ctx context.Context, bugID, taskID uint) error

	// Unlink снимает привязку и возвращает баг в перечень свободных.
	Unlink(ctx context.Context, taskID uint) error

	// UnlinkBug освобождает конкретный баг. Нужен, когда задача меняет
	// баг: прежний уже не найти по идентификатору задачи — её привязка
	// указывает на новый.
	UnlinkBug(ctx context.Context, bugID uint) error

	// SyncStatus переносит состояние задачи на привязанный к ней баг.
	// Задача без бага — не ошибка: адаптер просто ничего не меняет.
	SyncStatus(ctx context.Context, taskID uint, status BugSync) error
}

// BugFilter — параметры перечня багов.
type BugFilter struct {
	Tag string

	// Limit ограничивает выборку. Ноль означает размер по умолчанию,
	// заданный на стороне каталога.
	Limit int
}

// BugStatusFor переводит статус задачи в статус привязанного бага.
//
// Соответствие одностороннее: задача — источник истины для бага,
// который в ней ведут. Пока работа не начата, баг остаётся тем, чем
// был, поэтому new и review сюда не попадают — второй ничего не
// меняет для того, кто баг завёл, а первый означает, что задачу ещё
// не взяли.
func BugStatusFor(status Status) (BugSync, bool) {
	switch status {
	case WorkingStatus:
		return BugInWork, true
	case CompleteStatus:
		return BugClosed, true
	default:
		return "", false
	}
}

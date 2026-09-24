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

	// Итог проверки разработчиком. Пусто, пока отчёт не проверен или
	// возвращён на повторную проверку.
	ReviewedAt    *time.Time
	RejectReason  BugRejectReason
	ReviewComment string
}

// Состояния бага, по которым таск-менеджер отбирает перечень.
//
// Значения принадлежат feedback-service. Здесь только те, что нужны
// экранам: очередь проверки, готовые к работе и отклонённые.
const (
	// BugStatusNew — отчёт пришёл и ждёт проверки разработчиком.
	BugStatusNew = "new"
	// BugStatusConfirmed — баг воспроизвели; только такой берут в задачу.
	BugStatusConfirmed = "confirmed"
	// BugStatusRejected — проверка баг не подтвердила.
	BugStatusRejected = "rejected"
)

// BugRejectReason — почему проверка не подтвердила баг.
//
// Коды фиксированы feedback-service: по ним там считают, какие отчёты
// чаще всего оказываются пустыми. Домен повторяет набор, чтобы отказать
// в неверной причине сразу, а не после похода в чужой сервис.
type BugRejectReason string

const (
	RejectNotReproducible  BugRejectReason = "not_reproducible"
	RejectNotABug          BugRejectReason = "not_a_bug"
	RejectDuplicate        BugRejectReason = "duplicate"
	RejectInsufficientInfo BugRejectReason = "insufficient_info"
	RejectSpam             BugRejectReason = "spam"
)

func (r BugRejectReason) IsValid() bool {
	switch r {
	case RejectNotReproducible, RejectNotABug, RejectDuplicate, RejectInsufficientInfo, RejectSpam:
		return true
	}
	return false
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
)

// BugCatalog — порт каталога багов.
//
// Реализуется клиентом feedback-service. nil допустим: тогда перечень
// багов пуст, а привязка отклоняется — сервис задач продолжает работать
// без интеграции.
type BugCatalog interface {
	// ListOpen возвращает свободные баги: по умолчанию — подтверждённые,
	// которые можно взять в задачу, а с BugFilter.Status — очередь
	// проверки или отклонённые.
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

	// Confirm фиксирует, что разработчик воспроизвёл баг: с этого
	// момента его можно брать в задачу.
	Confirm(ctx context.Context, params BugReview) (Bug, error)

	// Reject фиксирует, что проверка баг не подтвердила.
	Reject(ctx context.Context, params BugReview) (Bug, error)

	// Reopen возвращает баг на повторную проверку и стирает решение.
	Reopen(ctx context.Context, bugID uint) (Bug, error)
}

// BugReview — решение разработчика по отчёту.
type BugReview struct {
	BugID uint
	// Reason заполняется только при отклонении.
	Reason BugRejectReason
	// Comment — как баг воспроизвёлся или почему отклонён.
	Comment string
}

// BugFilter — параметры перечня багов.
type BugFilter struct {
	Tag string

	// Status заменяет набор по умолчанию (подтверждённые баги) одним
	// состоянием: BugStatusNew — очередь проверки, BugStatusRejected —
	// отклонённые. Пусто — перечень для привязки.
	Status string

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
// не взяли. Статусы проверки (confirmed, rejected) задача тоже не
// выставляет: это отдельное решение разработчика.
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

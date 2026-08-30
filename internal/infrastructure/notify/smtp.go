// Package notify содержит адаптеры доставки уведомлений.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/task_action"
	"RTM-Task/internal/domain/task"
)

// Заголовок и путь берутся из контракта smtp-service.
const (
	apiKeyHeader = "X-Api-Key"

	apiPrefix = "/api/v2"

	// Межсервисный путь: аутентификация по API-ключу.
	// Соседний /api/v2/emails/admin рассчитан на админку и проверяет
	// X-User-Admin, поэтому нам не подходит.
	sendPath = apiPrefix + "/service/emails"

	// Шаблон письма о назначении задачи. Заведён в smtp-service.
	assignmentTemplate = "newTask"
)

// Config — настройки клиента почтового сервиса.
type Config struct {
	BaseURL string
	ApiKey  string
	Timeout time.Duration
	AppURL  string
}

// SMTPNotifier отправляет письма через smtp-service платформы.
// Реализует task_action.Notifier.
type SMTPNotifier struct {
	cfg    Config
	client *http.Client
	logger *zap.Logger
}

func NewSMTPNotifier(cfg Config, logger *zap.Logger) *SMTPNotifier {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	cfg.BaseURL = normalizeBaseURL(cfg.BaseURL)

	return &SMTPNotifier{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
		logger: logger.Named("smtp_notifier"),
	}
}

// normalizeBaseURL приводит адрес к виду «схема + хост».
//
// В настройках адрес пишут и как http://smtp:8080, и как
// http://smtp:8080/api/v2 — во втором случае префикс пути удвоился бы
// и запрос ушёл бы в никуда.
func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimRight(raw, "/")
	return strings.TrimSuffix(trimmed, apiPrefix)
}

// sendEmailRequest — тело запроса к smtp-service.
type sendEmailRequest struct {
	TemplateID     string         `json:"templateId"`
	To             string         `json:"to"`
	Data           map[string]any `json:"data,omitempty"`
	IdempotencyKey string         `json:"idempotencyKey,omitempty"`
}

// sendEmailResponse — ответ smtp-service на постановку письма в очередь.
type sendEmailResponse struct {
	EmailID   string `json:"emailId"`
	Duplicate bool   `json:"duplicate"`
}

// NotifyAssignment сообщает исполнителю, что на него назначена задача.
//
// Доставка не влияет на исход операции: задача уже назначена, поэтому
// любая проблема с почтой только логируется.
func (n *SMTPNotifier) NotifyAssignment(ctx context.Context, notice task_action.AssignmentNotice) {
	// Адрес известен не всегда: сотрудник заводится по данным шлюза,
	// а email тот передаёт не в каждом контуре.
	if notice.Recipient.Email == nil || *notice.Recipient.Email == "" {
		n.logger.Debug("assignment notice skipped: recipient has no email",
			zap.Uint("staff_id", notice.Recipient.ID),
			zap.Uint("task_id", notice.Task.ID),
		)
		return
	}

	payload := sendEmailRequest{
		TemplateID: assignmentTemplate,
		To:         *notice.Recipient.Email,
		Data:       n.assignmentData(notice),
		// Ключ идемпотентности привязан к версии задачи: повторный запрос
		// того же назначения не создаст второе письмо, а следующее
		// назначение — создаст.
		IdempotencyKey: fmt.Sprintf(
			"rtm-task:assign:%d:%d:%d",
			notice.Task.ID, notice.Recipient.ID, notice.Task.Version,
		),
	}

	if err := n.send(ctx, payload); err != nil {
		n.logger.Warn("assignment notice not sent",
			zap.Uint("task_id", notice.Task.ID),
			zap.Uint("staff_id", notice.Recipient.ID),
			zap.Error(err),
		)
	}
}

// assignmentData собирает переменные шаблона.
//
// Шаблон подставляет значения как есть, поэтому сюда идут подписи для
// человека («В работе», «Баг»), а не коды домена.
func (n *SMTPNotifier) assignmentData(notice task_action.AssignmentNotice) map[string]any {
	obj := notice.Task

	data := map[string]any{
		"username":    notice.Recipient.FullName,
		"assignee":    notice.Recipient.FullName,
		"taskId":      taskCode(obj.ID),
		"title":       obj.Title,
		"description": obj.Description,
		"status":      statusTitle(obj.Status),
		"type":        typeTitle(obj.Type),
		"priority":    priorityTitle(obj.Priority),
	}

	if n.cfg.AppURL != "" {
		data["url"] = n.cfg.AppURL + "/tasks/" + strconv.FormatUint(uint64(obj.ID), 10)
	}

	return data
}

// send выполняет запрос к smtp-service.
func (n *SMTPNotifier) send(ctx context.Context, payload sendEmailRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, n.cfg.BaseURL+sendPath, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(apiKeyHeader, n.cfg.ApiKey)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("call smtp-service: %w", err)
	}
	defer resp.Body.Close()

	// Сервис отвечает 202: письмо принято в очередь, отправка асинхронная.
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("smtp-service returned %d", resp.StatusCode)
	}

	var result sendEmailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Письмо принято, а разобрать ответ не вышло — это не отказ.
		n.logger.Debug("decode smtp-service response failed", zap.Error(err))
		return nil
	}

	n.logger.Info("assignment notice queued",
		zap.String("email_id", result.EmailID),
		zap.Bool("duplicate", result.Duplicate),
	)
	return nil
}

func taskCode(id uint) string {
	return "RTM-" + strconv.FormatUint(uint64(id), 10)
}

// Подписи для писем: получатель читает их глазами, а не разбирает коды.
func statusTitle(status task.Status) string {
	switch status {
	case task.NewStatus:
		return "Новые"
	case task.WorkingStatus:
		return "В работе"
	case task.ReviewStatus:
		return "На проверке"
	case task.CompleteStatus:
		return "Завершены"
	default:
		return string(status)
	}
}

func typeTitle(taskType task.Type) string {
	switch taskType {
	case task.BugType:
		return "Баг"
	case task.FeatureType:
		return "Фича"
	case task.FixType:
		return "Фикс"
	default:
		return string(taskType)
	}
}

func priorityTitle(priority task.Priority) string {
	switch priority {
	case task.BasePriority:
		return "Критичный"
	case task.HighPriority:
		return "Высокий"
	case task.MediumPriority:
		return "Средний"
	case task.LowPriority:
		return "Низкий"
	default:
		return strconv.Itoa(priority.Int())
	}
}

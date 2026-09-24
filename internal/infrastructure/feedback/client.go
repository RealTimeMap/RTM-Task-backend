// Package feedback содержит адаптер каталога багов feedback-service.
package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

// Контракт межсервисных маршрутов feedback-service.
const (
	apiKeyHeader = "X-Api-Key"

	// Межсервисный префикс: аутентификация по ключу сервиса.
	// Соседний /api/v2/bug/list рассчитан на админку и проверяет
	// заголовки пользователя, которых у фонового вызова нет.
	servicePrefix = "/api/v2/service"

	bugsPath = servicePrefix + "/bugs"

	// defaultLimit ограничивает перечень: он показывается списком в
	// форме создания задачи, и отдавать туда всю базу багов незачем.
	defaultLimit = 50

	// maxLimit совпадает с потолком пагинации feedback-service:
	// просить больше бессмысленно, ответ всё равно будет урезан.
	maxLimit = 100
)

// Config — настройки клиента каталога багов.
type Config struct {
	BaseURL string
	ApiKey  string
	Timeout time.Duration
}

// Enabled сообщает, настроена ли интеграция.
func (c Config) Enabled() bool {
	return c.BaseURL != "" && c.ApiKey != ""
}

// Client — каталог багов поверх feedback-service.
// Реализует task.BugCatalog.
type Client struct {
	cfg    Config
	client *http.Client
	logger *zap.Logger
}

func NewClient(cfg Config, logger *zap.Logger) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	cfg.BaseURL = normalizeBaseURL(cfg.BaseURL)

	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
		logger: logger.Named("feedback_client"),
	}
}

// normalizeBaseURL приводит адрес к виду «схема + хост».
//
// В настройках адрес пишут и как http://feedback:8086, и вместе с
// префиксом пути — во втором случае префикс удвоился бы и запрос ушёл
// бы в никуда.
func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimRight(raw, "/")
	for _, suffix := range []string{servicePrefix, "/api/v2"} {
		trimmed = strings.TrimSuffix(trimmed, suffix)
	}
	return trimmed
}

// bugResponse — представление бага в ответе feedback-service.
type bugResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Title     string    `json:"title"`
	Desc      string    `json:"desc"`
	Tag       string    `json:"tag"`
	Status    string    `json:"status"`
	Platform  string    `json:"platform"`
	Build     string    `json:"build"`
	HasLogs   bool      `json:"hasLogs"`
	UserID    *uint     `json:"userId"`
	TaskID    *uint     `json:"taskId"`

	OS         string   `json:"os"`
	Resolution string   `json:"resolution"`
	Battery    *float64 `json:"battery"`

	// Итог проверки разработчиком. Пусто, пока баг не проверен.
	ReviewedAt    *time.Time `json:"reviewedAt"`
	RejectReason  string     `json:"rejectReason"`
	ReviewComment string     `json:"reviewComment"`
}

// bugDetailResponse — баг целиком: та же карточка плюс логи.
type bugDetailResponse struct {
	bugResponse

	Logs   []string `json:"logs"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
}

type bugListResponse struct {
	Items []bugResponse `json:"items"`
}

func (b bugResponse) toDomain() task.Bug {
	return task.Bug{
		ID:          b.ID,
		Title:       b.Title,
		Description: b.Desc,
		Tag:         b.Tag,
		Status:      b.Status,
		Platform:    b.Platform,
		Build:       b.Build,
		HasLogs:     b.HasLogs,
		CreatedAt:   b.CreatedAt,
		ReporterID:  b.UserID,
		TaskID:      b.TaskID,
		OS:          b.OS,
		Resolution:  b.Resolution,
		Battery:     b.Battery,

		ReviewedAt:    b.ReviewedAt,
		RejectReason:  task.BugRejectReason(b.RejectReason),
		ReviewComment: b.ReviewComment,
	}
}

func (b bugDetailResponse) toDomain() task.Bug {
	obj := b.bugResponse.toDomain()
	obj.Logs = b.Logs
	obj.Width = b.Width
	obj.Height = b.Height
	return obj
}

// ListOpen возвращает свободные баги: подтверждённые или, с явным
// статусом, очередь проверки и отклонённые.
func (c *Client) ListOpen(ctx context.Context, filter task.BugFilter) ([]task.Bug, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	query := url.Values{}
	query.Set("pageSize", strconv.Itoa(limit))
	if filter.Tag != "" {
		query.Set("tag", filter.Tag)
	}
	// Явный статус заменяет на той стороне набор открытых: без него
	// приходят подтверждённые баги, с ним — ровно запрошенное состояние.
	if filter.Status != "" {
		query.Set("status", filter.Status)
	}

	var body bugListResponse
	if err := c.do(ctx, http.MethodGet, bugsPath+"?"+query.Encode(), nil, &body); err != nil {
		return nil, translate(err, 0)
	}

	bugs := make([]task.Bug, 0, len(body.Items))
	for _, item := range body.Items {
		bugs = append(bugs, item.toDomain())
	}
	return bugs, nil
}

// Get отдаёт баг целиком — с логами и обстановкой воспроизведения.
func (c *Client) Get(ctx context.Context, bugID uint) (task.Bug, error) {
	path := fmt.Sprintf("%s/%d", bugsPath, bugID)

	var body bugDetailResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &body); err != nil {
		return task.Bug{}, translate(err, bugID)
	}
	return body.toDomain(), nil
}

// Link отмечает баг занятым задачей.
func (c *Client) Link(ctx context.Context, bugID, taskID uint) error {
	path := fmt.Sprintf("%s/%d/task", bugsPath, bugID)
	payload := map[string]any{"taskId": taskID}

	if err := c.do(ctx, http.MethodPut, path, payload, nil); err != nil {
		return translate(err, bugID)
	}
	return nil
}

// Unlink снимает привязку бага, который вела задача.
func (c *Client) Unlink(ctx context.Context, taskID uint) error {
	path := fmt.Sprintf("%s/task/%d", bugsPath, taskID)

	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return translate(err, 0)
	}
	return nil
}

// UnlinkBug освобождает конкретный баг.
func (c *Client) UnlinkBug(ctx context.Context, bugID uint) error {
	path := fmt.Sprintf("%s/%d/task", bugsPath, bugID)

	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return translate(err, bugID)
	}
	return nil
}

// SyncStatus переносит состояние задачи на привязанный к ней баг.
func (c *Client) SyncStatus(ctx context.Context, taskID uint, status task.BugSync) error {
	path := fmt.Sprintf("%s/task/%d/status", bugsPath, taskID)
	payload := map[string]any{"status": string(status)}

	if err := c.do(ctx, http.MethodPatch, path, payload, nil); err != nil {
		return translate(err, 0)
	}
	return nil
}

// Confirm фиксирует, что разработчик воспроизвёл баг.
func (c *Client) Confirm(ctx context.Context, review task.BugReview) (task.Bug, error) {
	path := fmt.Sprintf("%s/%d/confirm", bugsPath, review.BugID)
	payload := map[string]any{"comment": review.Comment}

	var body bugResponse
	if err := c.do(ctx, http.MethodPost, path, payload, &body); err != nil {
		return task.Bug{}, translate(err, review.BugID)
	}
	return body.toDomain(), nil
}

// Reject фиксирует, что проверка баг не подтвердила.
func (c *Client) Reject(ctx context.Context, review task.BugReview) (task.Bug, error) {
	path := fmt.Sprintf("%s/%d/reject", bugsPath, review.BugID)
	payload := map[string]any{
		"reason":  string(review.Reason),
		"comment": review.Comment,
	}

	var body bugResponse
	if err := c.do(ctx, http.MethodPost, path, payload, &body); err != nil {
		return task.Bug{}, translate(err, review.BugID)
	}
	return body.toDomain(), nil
}

// Reopen возвращает баг на повторную проверку.
func (c *Client) Reopen(ctx context.Context, bugID uint) (task.Bug, error) {
	path := fmt.Sprintf("%s/%d/reopen", bugsPath, bugID)

	var body bugResponse
	if err := c.do(ctx, http.MethodPost, path, nil, &body); err != nil {
		return task.Bug{}, translate(err, bugID)
	}
	return body.toDomain(), nil
}

// statusError — ответ feedback-service с кодом вне 2xx.
//
// Отдельный тип, а не строка: по коду решается, чья это ошибка. Отказ
// по состоянию бага (404, 409, 422) — ответ на запрос, и пользователь
// должен увидеть его как есть. Всё прочее — сбой соседа.
type statusError struct {
	Status int
	// Message — первое пояснение из тела ответа, если оно разобралось.
	Message string
	Body    string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("feedback-service returned %d: %s", e.Status, e.Body)
}

// errorBody — формат ошибки feedback-service: либо перечень пояснений
// по полям, либо одна строка.
type errorBody struct {
	Detail []struct {
		Msg string `json:"msg"`
	} `json:"detail"`
	Error string `json:"error"`
}

func newStatusError(status int, raw []byte) *statusError {
	err := &statusError{Status: status, Body: strings.TrimSpace(string(raw))}

	var body errorBody
	if json.Unmarshal(raw, &body) == nil {
		if len(body.Detail) > 0 {
			err.Message = body.Detail[0].Msg
		} else {
			err.Message = body.Error
		}
	}
	if err.Message == "" {
		err.Message = err.Body
	}
	return err
}

// translate приводит отказ feedback-service к доменной ошибке.
//
// Раньше любой отказ считался недоступностью, и это было честно, пока
// каталог отказывал только по сбою. Теперь он отказывает и по правилам —
// «баг ещё не подтверждён», «уже отклонён», — и выдавать такой ответ за
// «сервис временно недоступен» значило бы звать человека повторять то,
// что не пройдёт никогда.
//
// 401 и 403 остаются недоступностью: это расхождение ключей между
// сервисами, а не ошибка пользователя.
func translate(err error, bugID uint) error {
	var status *statusError
	if !errors.As(err, &status) {
		return task.ErrBugUnavailable(err)
	}

	switch status.Status {
	case http.StatusNotFound:
		return task.ErrBugNotFound(bugID)
	case http.StatusConflict:
		return task.ErrBugConflict(status.Message)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return apperror.NewValidationError("bug", status.Message, "value_error", nil)
	default:
		return task.ErrBugUnavailable(err)
	}
}

// do выполняет запрос к feedback-service и разбирает ответ в out.
//
// out может быть nil: часть маршрутов отвечает 204 без тела, и разбирать
// там нечего.
func (c *Client) do(ctx context.Context, method, path string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(apiKeyHeader, c.cfg.ApiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call feedback-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Тело ошибки читаем ограниченно: пользователю из него уходит
		// одна строка пояснения, и качать оттуда мегабайты незачем.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return newStatusError(resp.StatusCode, detail)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

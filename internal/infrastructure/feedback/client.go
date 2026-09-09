// Package feedback содержит адаптер каталога багов feedback-service.
package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/task"
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
	}
}

func (b bugDetailResponse) toDomain() task.Bug {
	obj := b.bugResponse.toDomain()
	obj.Logs = b.Logs
	obj.Width = b.Width
	obj.Height = b.Height
	return obj
}

// ListOpen возвращает баги, которые можно взять в задачу.
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

	var body bugListResponse
	if err := c.do(ctx, http.MethodGet, bugsPath+"?"+query.Encode(), nil, &body); err != nil {
		return nil, task.ErrBugUnavailable(err)
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
		return task.Bug{}, task.ErrBugUnavailable(err)
	}
	return body.toDomain(), nil
}

// Link отмечает баг занятым задачей.
func (c *Client) Link(ctx context.Context, bugID, taskID uint) error {
	path := fmt.Sprintf("%s/%d/task", bugsPath, bugID)
	payload := map[string]any{"taskId": taskID}

	if err := c.do(ctx, http.MethodPut, path, payload, nil); err != nil {
		return task.ErrBugUnavailable(err)
	}
	return nil
}

// Unlink снимает привязку бага, который вела задача.
func (c *Client) Unlink(ctx context.Context, taskID uint) error {
	path := fmt.Sprintf("%s/task/%d", bugsPath, taskID)

	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return task.ErrBugUnavailable(err)
	}
	return nil
}

// UnlinkBug освобождает конкретный баг.
func (c *Client) UnlinkBug(ctx context.Context, bugID uint) error {
	path := fmt.Sprintf("%s/%d/task", bugsPath, bugID)

	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return task.ErrBugUnavailable(err)
	}
	return nil
}

// SyncStatus переносит состояние задачи на привязанный к ней баг.
func (c *Client) SyncStatus(ctx context.Context, taskID uint, status task.BugSync) error {
	path := fmt.Sprintf("%s/task/%d/status", bugsPath, taskID)
	payload := map[string]any{"status": string(status)}

	if err := c.do(ctx, http.MethodPatch, path, payload, nil); err != nil {
		return task.ErrBugUnavailable(err)
	}
	return nil
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
		// Тело ошибки читаем ограниченно: оно идёт в лог, а не
		// пользователю, и качать оттуда мегабайты незачем.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf(
			"feedback-service returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(detail)),
		)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

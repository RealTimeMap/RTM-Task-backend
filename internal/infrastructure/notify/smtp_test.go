package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"RTM-Task/internal/app/use_cases/task_action"
	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

// captured — то, что заглушка smtp-service увидела в запросе.
type captured struct {
	path   string
	apiKey string
	body   map[string]any
	calls  int
}

// newStubService поднимает заглушку smtp-service, отвечающую заданным кодом.
func newStubService(t *testing.T, status int) (*httptest.Server, *captured) {
	t.Helper()

	seen := &captured{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls++
		seen.path = r.URL.Path
		seen.apiKey = r.Header.Get(apiKeyHeader)

		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &seen.body)

		w.WriteHeader(status)
		if status == http.StatusAccepted {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"emailId":   "eml_1",
				"duplicate": false,
			})
		}
	}))
	t.Cleanup(server.Close)

	return server, seen
}

func newNotice(email *string) task_action.AssignmentNotice {
	recipient := role.Staff{FullName: "Мира Соколова", Email: email}
	recipient.ID = 2

	assignee := uint(2)
	obj := task.Task{
		Title:       "Маркер не обновляется",
		Description: "Шаги воспроизведения",
		Type:        task.BugType,
		Status:      task.NewStatus,
		Priority:    task.HighPriority,
		AssigneeID:  &assignee,
		Version:     3,
	}
	obj.ID = 31

	return task_action.AssignmentNotice{Recipient: recipient, Task: obj}
}

func email(value string) *string { return &value }

func TestNotifyAssignmentSendsRequest(t *testing.T) {
	server, seen := newStubService(t, http.StatusAccepted)

	notifier := NewSMTPNotifier(Config{
		BaseURL: server.URL,
		ApiKey:  "key_secret",
		AppURL:  "https://tasks.example.com",
	}, zap.NewNop())

	notifier.NotifyAssignment(context.Background(), newNotice(email("mira@example.com")))

	if seen.calls != 1 {
		t.Fatalf("запросов = %d, ожидали 1", seen.calls)
	}
	if seen.path != sendPath {
		t.Fatalf("путь = %q, ожидали %q", seen.path, sendPath)
	}
	if seen.apiKey != "key_secret" {
		t.Fatalf("ключ = %q, ожидали key_secret", seen.apiKey)
	}
	if seen.body["templateId"] != assignmentTemplate {
		t.Fatalf("шаблон = %v, ожидали %q", seen.body["templateId"], assignmentTemplate)
	}
	if seen.body["to"] != "mira@example.com" {
		t.Fatalf("адрес = %v", seen.body["to"])
	}
	if seen.body["idempotencyKey"] == "" || seen.body["idempotencyKey"] == nil {
		t.Fatal("ключ идемпотентности не передан")
	}
}

func TestNotifyAssignmentFillsTemplateData(t *testing.T) {
	server, seen := newStubService(t, http.StatusAccepted)

	notifier := NewSMTPNotifier(Config{
		BaseURL: server.URL,
		ApiKey:  "key",
		AppURL:  "https://tasks.example.com",
	}, zap.NewNop())

	notifier.NotifyAssignment(context.Background(), newNotice(email("mira@example.com")))

	data, ok := seen.body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data не объект: %v", seen.body["data"])
	}

	// Шаблон подставляет значения как есть, поэтому проверяем подписи,
	// а не коды домена.
	expected := map[string]string{
		"username": "Мира Соколова",
		"assignee": "Мира Соколова",
		"taskId":   "RTM-31",
		"title":    "Маркер не обновляется",
		"status":   "Новые",
		"type":     "Баг",
		"priority": "Высокий",
		"url":      "https://tasks.example.com/tasks/31",
	}

	for key, want := range expected {
		if got := data[key]; got != want {
			t.Fatalf("data[%q] = %v, ожидали %q", key, got, want)
		}
	}
}

func TestNotifyAssignmentSkipsWithoutEmail(t *testing.T) {
	server, seen := newStubService(t, http.StatusAccepted)

	notifier := NewSMTPNotifier(Config{BaseURL: server.URL, ApiKey: "key"}, zap.NewNop())

	// Ни nil, ни пустая строка не должны приводить к запросу:
	// у сотрудника, заведённого по данным шлюза, адреса может не быть.
	notifier.NotifyAssignment(context.Background(), newNotice(nil))
	notifier.NotifyAssignment(context.Background(), newNotice(email("")))

	if seen.calls != 0 {
		t.Fatalf("запросов = %d, ожидали 0", seen.calls)
	}
}

func TestNotifyAssignmentToleratesServiceError(t *testing.T) {
	server, seen := newStubService(t, http.StatusInternalServerError)

	notifier := NewSMTPNotifier(Config{BaseURL: server.URL, ApiKey: "key"}, zap.NewNop())

	// Отказ почты не должен ронять вызывающий код: он ничего не возвращает
	// и не паникует — задача уже назначена.
	notifier.NotifyAssignment(context.Background(), newNotice(email("mira@example.com")))

	if seen.calls != 1 {
		t.Fatalf("запросов = %d, ожидали 1", seen.calls)
	}
}

func TestNotifyAssignmentToleratesUnreachableService(t *testing.T) {
	// Адрес, на котором никто не слушает.
	notifier := NewSMTPNotifier(Config{
		BaseURL: "http://127.0.0.1:1",
		ApiKey:  "key",
	}, zap.NewNop())

	notifier.NotifyAssignment(context.Background(), newNotice(email("mira@example.com")))
}

func TestBaseURLNormalization(t *testing.T) {
	// Адрес пишут по-разному; путь к ручке не должен от этого удваиваться.
	cases := map[string]string{
		"http://smtp:8080":             "http://smtp:8080",
		"http://smtp:8080/":            "http://smtp:8080",
		"http://smtp:8080/api/v2":      "http://smtp:8080",
		"http://smtp:8080/api/v2/":     "http://smtp:8080",
		"http://127.0.0.1:8087/api/v2": "http://127.0.0.1:8087",
	}

	for raw, want := range cases {
		if got := normalizeBaseURL(raw); got != want {
			t.Fatalf("normalizeBaseURL(%q) = %q, ожидали %q", raw, got, want)
		}
	}
}

func TestNotifyUsesCorrectPathWithApiPrefixInBaseURL(t *testing.T) {
	server, seen := newStubService(t, http.StatusAccepted)

	// Адрес с уже включённым префиксом — так он записан в конфиге.
	notifier := NewSMTPNotifier(Config{
		BaseURL: server.URL + apiPrefix,
		ApiKey:  "key",
	}, zap.NewNop())

	notifier.NotifyAssignment(context.Background(), newNotice(email("mira@example.com")))

	if seen.path != sendPath {
		t.Fatalf("путь = %q, ожидали %q", seen.path, sendPath)
	}
}

func TestIdempotencyKeyChangesWithVersion(t *testing.T) {
	server, seen := newStubService(t, http.StatusAccepted)
	notifier := NewSMTPNotifier(Config{BaseURL: server.URL, ApiKey: "key"}, zap.NewNop())

	first := newNotice(email("mira@example.com"))
	notifier.NotifyAssignment(context.Background(), first)
	keyBefore := seen.body["idempotencyKey"]

	// Повтор того же назначения — ключ обязан совпасть, иначе письмо
	// продублируется при ретрае.
	notifier.NotifyAssignment(context.Background(), first)
	if seen.body["idempotencyKey"] != keyBefore {
		t.Fatalf("ключ изменился при повторе: %v -> %v", keyBefore, seen.body["idempotencyKey"])
	}

	// Следующее назначение той же задачи — версия другая, ключ должен
	// отличаться, иначе письмо потеряется в дедупликации.
	next := newNotice(email("mira@example.com"))
	next.Task.Version = 4
	notifier.NotifyAssignment(context.Background(), next)

	if seen.body["idempotencyKey"] == keyBefore {
		t.Fatalf("ключ не изменился при новой версии задачи: %v", keyBefore)
	}
}

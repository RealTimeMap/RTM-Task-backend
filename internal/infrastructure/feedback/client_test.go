package feedback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/task"
	"RTM-Task/internal/utils/apperror"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return NewClient(Config{BaseURL: server.URL, ApiKey: "secret"}, zap.NewNop())
}

// Отказ по состоянию бага — ответ на запрос, а не сбой соседа: человеку
// нужно увидеть «баг ещё не подтверждён», а не «попробуйте позже».
func TestTranslateStatusErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   apperror.Kind
		msg    string
	}{
		{
			name:   "conflict keeps catalog message",
			status: http.StatusConflict,
			body:   `{"detail":[{"loc":["body","status"],"msg":"bug 7 is not confirmed yet","type":"value_error.conflict"}]}`,
			want:   apperror.KindConflict,
			msg:    "bug 7 is not confirmed yet",
		},
		{
			name:   "not found",
			status: http.StatusNotFound,
			body:   `{"detail":[{"msg":"bug not found"}]}`,
			want:   apperror.KindNotFound,
		},
		{
			name:   "validation",
			status: http.StatusUnprocessableEntity,
			body:   `{"detail":[{"msg":"reason is not allowed"}]}`,
			want:   apperror.KindValidation,
			msg:    "reason is not allowed",
		},
		{
			name:   "wrong api key is still unavailability",
			status: http.StatusUnauthorized,
			body:   `{"error":"unauthorized"}`,
			want:   apperror.KindUnavailable,
		},
		{
			name:   "server failure",
			status: http.StatusInternalServerError,
			body:   `{"error":"internal server error"}`,
			want:   apperror.KindUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			_, err := client.Confirm(context.Background(), task.BugReview{BugID: 7})

			appErr, ok := apperror.As(err)
			if !ok || appErr.Kind != tc.want {
				t.Fatalf("err = %v, want kind %q", err, tc.want)
			}
			if tc.msg != "" && appErr.Message != tc.msg {
				t.Fatalf("message = %q, want %q", appErr.Message, tc.msg)
			}
		})
	}
}

func TestRejectSendsReasonAndParsesReview(t *testing.T) {
	var got map[string]string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/service/bugs/7/reject" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get(apiKeyHeader) != "secret" {
			t.Errorf("api key header missing")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)

		_, _ = w.Write([]byte(`{
			"id": 7, "title": "Карта", "status": "rejected", "tag": "ui",
			"reviewedAt": "2026-09-24T10:00:00Z",
			"rejectReason": "duplicate", "reviewComment": "см. #5"
		}`))
	})

	obj, err := client.Reject(context.Background(), task.BugReview{
		BugID:   7,
		Reason:  task.RejectDuplicate,
		Comment: "см. #5",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}

	if got["reason"] != "duplicate" || got["comment"] != "см. #5" {
		t.Fatalf("payload = %v", got)
	}
	if obj.Status != task.BugStatusRejected || obj.RejectReason != task.RejectDuplicate {
		t.Fatalf("bug = %+v", obj)
	}
	if obj.ReviewedAt == nil || obj.ReviewComment != "см. #5" {
		t.Fatalf("review not parsed: %+v", obj)
	}
}

func TestListOpenPassesStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "new" {
			t.Errorf("status query = %q, want new", r.URL.Query().Get("status"))
		}
		_, _ = w.Write([]byte(`{"items":[{"id":1,"status":"new"}]}`))
	})

	bugs, err := client.ListOpen(context.Background(), task.BugFilter{Status: task.BugStatusNew})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(bugs) != 1 || bugs[0].Status != task.BugStatusNew {
		t.Fatalf("bugs = %+v", bugs)
	}
}

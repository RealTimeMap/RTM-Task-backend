package task

import (
	"context"
	"strings"
	"testing"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

func TestConfirmBugPassesTrimmedComment(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7, Status: BugStatusNew})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj, err := svc.ConfirmBug(context.Background(), actor(1, role.DeveloperRole), BugReview{
		BugID:   7,
		Comment: "  воспроизвёлся на Android 14  ",
		// Причина у подтверждения не имеет смысла и не должна уйти в каталог.
		Reason: RejectSpam,
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if obj.Status != BugStatusConfirmed {
		t.Fatalf("status = %q, want %q", obj.Status, BugStatusConfirmed)
	}

	got := bugs.reviews[0]
	if got.Comment != "воспроизвёлся на Android 14" {
		t.Fatalf("comment = %q, want trimmed", got.Comment)
	}
	if got.Reason != "" {
		t.Fatalf("reason = %q, want empty for confirm", got.Reason)
	}
}

// Наблюдатель читает отчёты, но решений по ним не принимает.
func TestReviewBugForbiddenForViewer(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7, Status: BugStatusNew})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)
	viewer := actor(1, role.ViewerRole)
	ctx := context.Background()

	calls := map[string]func() error{
		"confirm": func() error {
			_, err := svc.ConfirmBug(ctx, viewer, BugReview{BugID: 7})
			return err
		},
		"reject": func() error {
			_, err := svc.RejectBug(ctx, viewer, BugReview{BugID: 7, Reason: RejectSpam})
			return err
		},
		"reopen": func() error {
			_, err := svc.ReopenBug(ctx, viewer, 7)
			return err
		},
	}

	for name, call := range calls {
		err := call()
		appErr, ok := apperror.As(err)
		if !ok || appErr.Kind != apperror.KindForbidden {
			t.Fatalf("%s: err = %v, want forbidden", name, err)
		}
	}
	if len(bugs.reviews) != 0 {
		t.Fatalf("catalog got %d reviews, want none", len(bugs.reviews))
	}
}

// Неверную причину отклоняем сами: ходить с ней в каталог незачем.
func TestRejectBugValidatesReasonBeforeCatalog(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7, Status: BugStatusNew})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	_, err := svc.RejectBug(context.Background(), actor(1, role.DeveloperRole), BugReview{
		BugID:  7,
		Reason: "boring",
	})

	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation || appErr.Field != "reason" {
		t.Fatalf("err = %v, want validation error on reason", err)
	}
	if len(bugs.reviews) != 0 {
		t.Fatalf("catalog got %d reviews, want none", len(bugs.reviews))
	}
}

func TestRejectBugStoresReason(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7, Status: BugStatusNew})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj, err := svc.RejectBug(context.Background(), actor(1, role.ManagerRole), BugReview{
		BugID:  7,
		Reason: RejectDuplicate,
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if obj.Status != BugStatusRejected || obj.RejectReason != RejectDuplicate {
		t.Fatalf("bug = %+v, want rejected as duplicate", obj)
	}
}

func TestReviewBugRejectsTooLongComment(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7, Status: BugStatusNew})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	_, err := svc.ConfirmBug(context.Background(), actor(1, role.DeveloperRole), BugReview{
		BugID:   7,
		Comment: strings.Repeat("я", maxBugReviewCommentLength+1),
	})

	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation || appErr.Field != "comment" {
		t.Fatalf("err = %v, want validation error on comment", err)
	}
}

// Отказ каталога по правилам (например, «баг уже в работе») должен
// дойти до пользователя как есть, а не превратиться в «сервис недоступен».
func TestReviewBugKeepsCatalogConflict(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	bugs.err = ErrBugConflict("bug 7 is in work and cannot be rejected")
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	_, err := svc.RejectBug(context.Background(), actor(1, role.DeveloperRole), BugReview{
		BugID:  7,
		Reason: RejectNotABug,
	})

	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindConflict {
		t.Fatalf("err = %v, want conflict", err)
	}
}

func TestReviewBugWithoutCatalogIsUnavailable(t *testing.T) {
	svc := newTestServiceWithBugs(newFakeRepository(), nil, newFakeBugs())
	svc.Service.bugs = nil

	_, err := svc.ReopenBug(context.Background(), actor(1, role.DeveloperRole), 7)

	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindUnavailable {
		t.Fatalf("err = %v, want unavailable", err)
	}
}

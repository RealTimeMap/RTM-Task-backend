package task

import (
	"context"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	"RTM-Task/internal/domain/role"
)

// maxBugReviewCommentLength совпадает с потолком feedback-service:
// длиннее он не примет, и лучше отказать здесь понятной ошибкой.
const maxBugReviewCommentLength = 2000

// ConfirmBug фиксирует, что разработчик воспроизвёл баг.
//
// Проверка отделена от заведения задачи: большинство отчётов не
// подтверждается, и решение по ним принимают до того, как по багу
// заводят работу. В задачу берут только подтверждённые баги.
func (s *Service) ConfirmBug(ctx context.Context, actor role.Actor, review BugReview) (Bug, error) {
	review.Reason = ""
	if err := s.ensureCanReview(actor, &review); err != nil {
		return Bug{}, err
	}

	obj, err := s.bugs.Confirm(ctx, review)
	if err != nil {
		return Bug{}, asBugUnavailable(err)
	}

	s.logger.Info("bug confirmed",
		zap.Uint("bug_id", review.BugID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return obj, nil
}

// RejectBug фиксирует, что проверка баг не подтвердила.
//
// Причина обязательна: по ней feedback-service считает, какие отчёты
// оказываются пустыми.
func (s *Service) RejectBug(ctx context.Context, actor role.Actor, review BugReview) (Bug, error) {
	if !review.Reason.IsValid() {
		return Bug{}, ErrBugRejectReasonInvalid(string(review.Reason))
	}
	if err := s.ensureCanReview(actor, &review); err != nil {
		return Bug{}, err
	}

	obj, err := s.bugs.Reject(ctx, review)
	if err != nil {
		return Bug{}, asBugUnavailable(err)
	}

	s.logger.Info("bug rejected",
		zap.Uint("bug_id", review.BugID),
		zap.String("reason", string(review.Reason)),
		zap.Uint("actor_id", actor.StaffID),
	)
	return obj, nil
}

// ReopenBug возвращает баг на повторную проверку.
//
// Нужен, когда решение оказалось ошибочным: отклонённый баг всё-таки
// воспроизвёлся или подтверждение было поспешным.
func (s *Service) ReopenBug(ctx context.Context, actor role.Actor, bugID uint) (Bug, error) {
	review := BugReview{BugID: bugID}
	if err := s.ensureCanReview(actor, &review); err != nil {
		return Bug{}, err
	}

	obj, err := s.bugs.Reopen(ctx, bugID)
	if err != nil {
		return Bug{}, asBugUnavailable(err)
	}

	s.logger.Info("bug reopened for review",
		zap.Uint("bug_id", bugID),
		zap.Uint("actor_id", actor.StaffID),
	)
	return obj, nil
}

// ensureCanReview проверяет право на решение и приводит пояснение к
// виду, который примет каталог.
func (s *Service) ensureCanReview(actor role.Actor, review *BugReview) error {
	if !actor.Role.CanReviewBug() {
		return ErrBugReviewForbidden(actor.Role.String())
	}
	if s.bugs == nil {
		return ErrBugUnavailable(nil)
	}

	review.Comment = strings.TrimSpace(review.Comment)
	if utf8.RuneCountInString(review.Comment) > maxBugReviewCommentLength {
		return ErrBugReviewCommentTooLong(review.Comment)
	}
	return nil
}

package task

import (
	"context"
	"testing"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

func actor(id uint, r role.Role) role.Actor {
	return role.Actor{StaffID: id, Role: r}
}

func requireKind(t *testing.T, err error, kind apperror.Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", kind)
	}
	appErr, ok := apperror.As(err)
	if !ok {
		t.Fatalf("expected *apperror.AppError, got %T: %v", err, err)
	}
	if appErr.Kind != kind {
		t.Fatalf("error kind = %s, want %s (%v)", appErr.Kind, kind, err)
	}
}

func validCreateParams() CreateTaskParams {
	return CreateTaskParams{
		Title: "Fix the broken import",
		Type:  BugType,
	}
}

func TestCreateRejectsViewerRole(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	_, err := svc.Create(context.Background(), actor(1, role.ViewerRole), validCreateParams())

	requireKind(t, err, apperror.KindForbidden)
}

func TestCreateSetsDefaults(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	obj, err := svc.Create(context.Background(), actor(1, role.DeveloperRole), validCreateParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if obj.Status != NewStatus {
		t.Fatalf("status = %s, want %s", obj.Status, NewStatus)
	}
	if obj.Priority != HighPriority {
		t.Fatalf("priority = %d, want %d", obj.Priority, HighPriority)
	}
	if obj.CreatorID != 1 {
		t.Fatalf("creatorID = %d, want 1", obj.CreatorID)
	}
	if obj.Version != 1 {
		t.Fatalf("version = %d, want 1", obj.Version)
	}
}

func TestCreateRejectsUnknownType(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	params := validCreateParams()
	params.Type = Type("epic")

	_, err := svc.Create(context.Background(), actor(1, role.ManagerRole), params)

	requireKind(t, err, apperror.KindValidation)
}

func TestCreateEnforcesDailyLimit(t *testing.T) {
	repo := newFakeRepository()
	repo.todayCount = maxTasksPerDay
	svc := newTestService(repo, nil)

	_, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())

	requireKind(t, err, apperror.KindConflict)
	if repo.createCalls != 0 {
		t.Fatal("task must not be persisted when the daily limit is reached")
	}
}

func TestDeveloperCannotAssignTaskToSomeoneElseOnCreate(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	other := uint(42)
	params := validCreateParams()
	params.AssigneeID = &other

	_, err := svc.Create(context.Background(), actor(1, role.DeveloperRole), params)

	requireKind(t, err, apperror.KindForbidden)
}

func TestDeveloperCanAssignTaskToSelfOnCreate(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	self := uint(1)
	params := validCreateParams()
	params.AssigneeID = &self

	obj, err := svc.Create(context.Background(), actor(1, role.DeveloperRole), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !obj.IsAssignedTo(1) {
		t.Fatal("task must be assigned to its creator")
	}
}

func TestManagerCanAssignAnyone(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, nil)

	created, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := svc.Assign(context.Background(), actor(1, role.ManagerRole), created.ID, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.IsAssignedTo(42) {
		t.Fatal("task must be assigned to staff 42")
	}
	if updated.Version != 2 {
		t.Fatalf("version = %d, want 2 after update", updated.Version)
	}
}

func TestAssignPropagatesInactiveStaffError(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, &fakeStaff{err: role.ErrStaffInactive(42)})

	created, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Assign(context.Background(), actor(1, role.ManagerRole), created.ID, 42)

	requireKind(t, err, apperror.KindForbidden)
}

func TestUpdateForbiddenForUnrelatedDeveloper(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, nil)

	created, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	title := "Hijacked title"
	_, err = svc.Update(context.Background(), actor(99, role.DeveloperRole), created.ID, UpdateTaskParams{Title: &title})

	requireKind(t, err, apperror.KindForbidden)
}

func TestUpdateAllowedForCreator(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, nil)

	created, err := svc.Create(context.Background(), actor(1, role.DeveloperRole), validCreateParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	title := "Refined title"
	updated, err := svc.Update(context.Background(), actor(1, role.DeveloperRole), created.ID, UpdateTaskParams{Title: &title})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != title {
		t.Fatalf("title = %q, want %q", updated.Title, title)
	}
}

func TestCloseForbiddenForNonAssigneeDeveloper(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, nil)

	// Задача создана разработчиком 1 и назначена на разработчика 2.
	self := uint(1)
	params := validCreateParams()
	params.AssigneeID = &self
	created, err := svc.Create(context.Background(), actor(1, role.DeveloperRole), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.ChangeStatus(context.Background(), actor(1, role.DeveloperRole), created.ID, WorkingStatus); err != nil {
		t.Fatalf("unexpected error moving to work: %v", err)
	}
	if _, err := svc.ChangeStatus(context.Background(), actor(1, role.DeveloperRole), created.ID, ReviewStatus); err != nil {
		t.Fatalf("unexpected error moving to review: %v", err)
	}

	// Переназначаем на другого сотрудника, автор перестаёт быть исполнителем.
	if _, err := svc.Assign(context.Background(), actor(5, role.ManagerRole), created.ID, 2); err != nil {
		t.Fatalf("unexpected error reassigning: %v", err)
	}

	_, err = svc.ChangeStatus(context.Background(), actor(1, role.DeveloperRole), created.ID, CompleteStatus)

	requireKind(t, err, apperror.KindForbidden)
}

func TestDeleteForbiddenForForeignDeveloper(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, nil)

	created, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.Delete(context.Background(), actor(99, role.DeveloperRole), created.ID)

	requireKind(t, err, apperror.KindForbidden)
	if repo.deleteCalls != 0 {
		t.Fatal("repository delete must not be called when access is denied")
	}
}

func TestDeleteAllowedForManager(t *testing.T) {
	repo := newFakeRepository()
	svc := newTestService(repo, nil)

	created, err := svc.Create(context.Background(), actor(1, role.DeveloperRole), validCreateParams())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Delete(context.Background(), actor(5, role.ManagerRole), created.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.GetByID(context.Background(), created.ID); err == nil {
		t.Fatal("task must be gone after delete")
	}
}

func TestGetByIDReturnsNotFound(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	_, err := svc.GetByID(context.Background(), 404)

	requireKind(t, err, apperror.KindNotFound)
}

func TestListRejectsInvalidStatusFilter(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	status := Status("archived")
	_, _, err := svc.List(context.Background(), Filter{Status: &status})

	requireKind(t, err, apperror.KindValidation)
}

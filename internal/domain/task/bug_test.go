package task

import (
	"context"
	"errors"
	"testing"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/utils/apperror"
)

// takeIntoWork доводит задачу до статуса «в работе»: только оттуда
// видна синхронизация бага, и повторять эти два шага в каждом тесте
// было бы шумом.
func takeIntoWork(t *testing.T, svc testService, id, staffID uint) *Task {
	t.Helper()
	ctx := context.Background()
	manager := actor(staffID, role.ManagerRole)

	if _, err := svc.Assign(ctx, manager, id, staffID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	obj, err := svc.ChangeStatus(ctx, manager, id, WorkingStatus)
	if err != nil {
		t.Fatalf("change status: %v", err)
	}
	return obj
}

func createWithBug(t *testing.T, svc testService, bugID uint) *Task {
	t.Helper()

	params := validCreateParams()
	params.BugID = &bugID

	obj, err := svc.Create(context.Background(), actor(1, role.ManagerRole), params)
	if err != nil {
		t.Fatalf("create task with bug: %v", err)
	}
	return obj
}

func TestCreateAttachesBugAndMarksItTaken(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7, Title: "Карта не грузится"})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)

	if obj.BugID == nil || *obj.BugID != 7 {
		t.Fatalf("bugID = %v, want 7", obj.BugID)
	}
	if bugs.linked[7] != obj.ID {
		t.Fatalf("bug 7 linked to task %d, want %d", bugs.linked[7], obj.ID)
	}
}

// Занятый баг не должен предлагаться второй раз: иначе две задачи
// вели бы один и тот же баг, и обратная синхронизация конфликтовала бы.
func TestListOpenBugsHidesTakenOnes(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7}, Bug{ID: 8})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	createWithBug(t, svc, 7)

	free, err := svc.ListOpenBugs(context.Background(), BugFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(free) != 1 || free[0].ID != 8 {
		t.Fatalf("open bugs = %+v, want only bug 8", free)
	}
}

func TestCreateRejectsBugOnNonBugType(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	params := validCreateParams()
	params.Type = FeatureType
	bugID := uint(7)
	params.BugID = &bugID

	_, err := svc.Create(context.Background(), actor(1, role.ManagerRole), params)

	requireKind(t, err, apperror.KindConflict)
}

// Каталог мог отказать: баг успели забрать или сервис недоступен.
// Задача при этом остаётся — она уже заведена и полезна, — но ссылку
// на баг держать нельзя: каталог считает его свободным.
func TestCreateRollsBackBugLinkWhenCatalogFails(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	bugs.err = errors.New("feedback-service is down")
	repo := newFakeRepository()
	svc := newTestServiceWithBugs(repo, nil, bugs)

	params := validCreateParams()
	bugID := uint(7)
	params.BugID = &bugID

	_, err := svc.Create(context.Background(), actor(1, role.ManagerRole), params)
	if err == nil {
		t.Fatal("expected error when catalog rejects the link")
	}

	stored, _, listErr := repo.List(context.Background(), Filter{})
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	if len(stored) != 1 {
		t.Fatalf("stored tasks = %d, want 1 - the task itself must survive", len(stored))
	}
	if stored[0].HasBug() {
		t.Fatalf("task kept bug %v after the catalog refused the link", stored[0].BugID)
	}
}

func TestWorkingStatusPutsBugInWork(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)
	takeIntoWork(t, svc, obj.ID, 1)

	if got := bugs.synced[obj.ID]; got != BugInWork {
		t.Fatalf("synced status = %q, want %q", got, BugInWork)
	}
}

func TestCompletingTaskClosesBug(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)
	ctx := context.Background()
	manager := actor(1, role.ManagerRole)

	obj := createWithBug(t, svc, 7)
	takeIntoWork(t, svc, obj.ID, 1)

	if _, err := svc.ChangeStatus(ctx, manager, obj.ID, ReviewStatus); err != nil {
		t.Fatalf("review: %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, manager, obj.ID, CompleteStatus); err != nil {
		t.Fatalf("complete: %v", err)
	}

	if got := bugs.synced[obj.ID]; got != BugClosed {
		t.Fatalf("synced status = %q, want %q", got, BugClosed)
	}
}

// Недоступный каталог не должен мешать работать с задачей: статус
// меняется, расхождение чинится следующей синхронизацией.
func TestStatusChangeSurvivesCatalogFailure(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)
	bugs.err = errors.New("feedback-service is down")

	updated := takeIntoWork(t, svc, obj.ID, 1)

	if updated.Status != WorkingStatus {
		t.Fatalf("status = %s, want %s", updated.Status, WorkingStatus)
	}
}

func TestDeleteReleasesBug(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)

	if err := svc.Delete(context.Background(), actor(1, role.ManagerRole), obj.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, taken := bugs.linked[7]; taken {
		t.Fatal("bug 7 is still linked after its task was deleted")
	}
}

func TestDetachBugReleasesIt(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)

	updated, err := svc.DetachBug(context.Background(), actor(1, role.ManagerRole), obj.ID)
	if err != nil {
		t.Fatalf("detach: %v", err)
	}

	if updated.HasBug() {
		t.Fatalf("task still has bug %v", updated.BugID)
	}
	if _, taken := bugs.linked[7]; taken {
		t.Fatal("bug 7 is still linked after detach")
	}
}

// Смена бага должна освободить прежний: иначе он навсегда остался бы
// числиться за этой задачей и не вернулся бы в перечень.
func TestAttachAnotherBugReleasesPrevious(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7}, Bug{ID: 8})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)

	updated, err := svc.AttachBug(context.Background(), actor(1, role.ManagerRole), obj.ID, 8)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if updated.BugID == nil || *updated.BugID != 8 {
		t.Fatalf("bugID = %v, want 8", updated.BugID)
	}
	if _, taken := bugs.linked[7]; taken {
		t.Fatal("previous bug 7 is still linked")
	}
	if bugs.linked[8] != obj.ID {
		t.Fatalf("bug 8 linked to task %d, want %d", bugs.linked[8], obj.ID)
	}
}

// Тип задачи и привязка бага связаны: увести задачу из типа bug, не
// отвязав баг, значило бы закрыть его по завершении посторонней работы.
func TestChangingTypeWithAttachedBugIsRejected(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)

	featureType := FeatureType
	_, err := svc.Update(context.Background(), actor(1, role.ManagerRole), obj.ID, UpdateTaskParams{
		Type: &featureType,
	})

	requireKind(t, err, apperror.KindConflict)
}

func TestAttachBugRejectsNonBugTask(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	params := validCreateParams()
	params.Type = RefactorType
	obj, err := svc.Create(context.Background(), actor(1, role.ManagerRole), params)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = svc.AttachBug(context.Background(), actor(1, role.ManagerRole), obj.ID, 7)

	requireKind(t, err, apperror.KindConflict)
}

// Без настроенной интеграции сервис задач работает как раньше:
// перечень пуст, а не ошибочен.
func TestListOpenBugsWithoutCatalogIsEmpty(t *testing.T) {
	svc := newTestServiceWithBugs(newFakeRepository(), nil, nil)

	bugs, err := svc.ListOpenBugs(context.Background(), BugFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bugs) != 0 {
		t.Fatalf("bugs = %d, want 0", len(bugs))
	}
}

// Проект по умолчанию должен проставляться сам: старые клиенты его не
// передают, и задача без проекта не должна попадать в пустую колонку.
func TestCreateSetsDefaultProject(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	obj, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if obj.Project != DefaultProject {
		t.Fatalf("project = %q, want %q", obj.Project, DefaultProject)
	}
}

func TestCreateRejectsUnknownProject(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	params := validCreateParams()
	params.Project = Project("rtm-web")

	_, err := svc.Create(context.Background(), actor(1, role.ManagerRole), params)

	requireKind(t, err, apperror.KindValidation)
}

func TestUpdateChangesProject(t *testing.T) {
	svc := newTestService(newFakeRepository(), nil)

	obj, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	target := AppProject
	updated, err := svc.Update(context.Background(), actor(1, role.ManagerRole), obj.ID, UpdateTaskParams{
		Project: &target,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Project != AppProject {
		t.Fatalf("project = %q, want %q", updated.Project, AppProject)
	}
}

// Доработка возвращает задачу в работу, а с ней и баг: закрытый баг
// снова становится открытым, иначе он остался бы закрытым при том,
// что работа над ним продолжается.
func TestReworkReopensBug(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)
	ctx := context.Background()
	manager := actor(1, role.ManagerRole)

	obj := createWithBug(t, svc, 7)
	takeIntoWork(t, svc, obj.ID, 1)

	if _, err := svc.ChangeStatus(ctx, manager, obj.ID, ReviewStatus); err != nil {
		t.Fatalf("review: %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, manager, obj.ID, CompleteStatus); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got := bugs.synced[obj.ID]; got != BugClosed {
		t.Fatalf("synced status before rework = %q, want %q", got, BugClosed)
	}

	if _, err := svc.SendToRework(ctx, manager, obj.ID, ReworkParams{
		Note: "Маркер всё ещё дублируется на зуме",
	}); err != nil {
		t.Fatalf("rework: %v", err)
	}

	if got := bugs.synced[obj.ID]; got != BugInWork {
		t.Fatalf("synced status after rework = %q, want %q", got, BugInWork)
	}
}

// Подробности бага читаются через задачу: идентификатор берётся из неё,
// поэтому чужой отчёт в свою карточку не подтянуть.
func TestGetBugReturnsDetails(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7, Title: "Карта не грузится", OS: "Android 13"})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)

	details, err := svc.GetBug(context.Background(), obj.ID)
	if err != nil {
		t.Fatalf("get bug: %v", err)
	}

	if details.ID != 7 {
		t.Fatalf("bug id = %d, want 7", details.ID)
	}
	if details.OS != "Android 13" {
		t.Fatalf("os = %q, want %q", details.OS, "Android 13")
	}
	// Логи приезжают только при чтении одного бага — в перечне их нет.
	if len(details.Logs) == 0 {
		t.Fatal("logs are empty, want them loaded with the details")
	}
}

func TestGetBugWithoutAttachedBugFails(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj, err := svc.Create(context.Background(), actor(1, role.ManagerRole), validCreateParams())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = svc.GetBug(context.Background(), obj.ID)

	requireKind(t, err, apperror.KindConflict)
}

// Недоступность feedback-service должна доезжать до клиента как
// «сервис временно недоступен» (503), а не как внутренняя ошибка:
// виноваты не мы и не запрос, и повтор имеет смысл.
func TestBugCatalogFailureIsUnavailable(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	bugs.err = errors.New("connection refused")
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	_, err := svc.ListOpenBugs(context.Background(), BugFilter{})

	requireKind(t, err, apperror.KindUnavailable)
}

// То же для чтения отчёта: привязка бага цела, недоступен только сервис.
func TestGetBugFailureIsUnavailable(t *testing.T) {
	bugs := newFakeBugs(Bug{ID: 7})
	svc := newTestServiceWithBugs(newFakeRepository(), nil, bugs)

	obj := createWithBug(t, svc, 7)
	bugs.err = errors.New("connection refused")

	_, err := svc.GetBug(context.Background(), obj.ID)

	requireKind(t, err, apperror.KindUnavailable)
}

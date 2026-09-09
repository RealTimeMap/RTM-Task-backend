package task

import "testing"

func assignedTask(status Status) *Task {
	assignee := uint(7)
	return &Task{Status: status, AssigneeID: &assignee}
}

func TestChangeStatusFollowsLifecycle(t *testing.T) {
	tests := []struct {
		name    string
		from    Status
		to      Status
		wantErr bool
	}{
		{"new -> working", NewStatus, WorkingStatus, false},
		{"working -> review", WorkingStatus, ReviewStatus, false},
		{"review -> complete", ReviewStatus, CompleteStatus, false},
		{"review -> working", ReviewStatus, WorkingStatus, false},
		{"working -> new", WorkingStatus, NewStatus, false},
		{"new -> complete is skipping review", NewStatus, CompleteStatus, true},
		{"new -> review is skipping work", NewStatus, ReviewStatus, true},
		{"complete is final", CompleteStatus, WorkingStatus, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := assignedTask(tt.from)

			err := obj.ChangeStatus(tt.to)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s -> %s, got nil", tt.from, tt.to)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %s -> %s: %v", tt.from, tt.to, err)
			}
			if obj.Status != tt.to {
				t.Fatalf("status = %s, want %s", obj.Status, tt.to)
			}
		})
	}
}

func TestChangeStatusToSameStatusIsRejected(t *testing.T) {
	obj := assignedTask(WorkingStatus)

	if err := obj.ChangeStatus(WorkingStatus); err == nil {
		t.Fatal("expected error when target status equals current")
	}
}

func TestChangeStatusToInvalidValueIsRejected(t *testing.T) {
	obj := assignedTask(NewStatus)

	if err := obj.ChangeStatus(Status("archived")); err == nil {
		t.Fatal("expected error for unknown status")
	}
}

func TestMoveToWorkRequiresAssignee(t *testing.T) {
	obj := &Task{Status: NewStatus}

	if err := obj.ChangeStatus(WorkingStatus); err == nil {
		t.Fatal("expected error when moving unassigned task to work")
	}
}

func TestClosingSetsClosedAt(t *testing.T) {
	obj := assignedTask(ReviewStatus)

	if err := obj.ChangeStatus(CompleteStatus); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.ClosedAt == nil {
		t.Fatal("ClosedAt must be set when task is closed")
	}
}

func TestReopeningClearsClosedAt(t *testing.T) {
	obj := assignedTask(ReviewStatus)
	if err := obj.ChangeStatus(CompleteStatus); err != nil {
		t.Fatalf("unexpected error closing task: %v", err)
	}

	// Задача закрыта — вернуть её в работу нельзя, ClosedAt остаётся.
	if err := obj.ChangeStatus(WorkingStatus); err == nil {
		t.Fatal("closed task must not be reopened")
	}
	if obj.ClosedAt == nil {
		t.Fatal("ClosedAt must survive a rejected transition")
	}
}

func TestAssignClosedTaskIsRejected(t *testing.T) {
	obj := &Task{Status: CompleteStatus}

	if err := obj.Assign(5); err == nil {
		t.Fatal("expected error when assigning closed task")
	}
}

func TestAssignSameStaffTwiceIsRejected(t *testing.T) {
	obj := assignedTask(NewStatus)

	if err := obj.Assign(7); err == nil {
		t.Fatal("expected error when assigning the same staff again")
	}
}

func TestUnassignActiveTaskIsRejected(t *testing.T) {
	obj := assignedTask(WorkingStatus)

	if err := obj.Unassign(); err == nil {
		t.Fatal("expected error when unassigning task in work")
	}
}

func TestUnassignNewTaskSucceeds(t *testing.T) {
	obj := assignedTask(NewStatus)

	if err := obj.Unassign(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.IsAssigned() {
		t.Fatal("assignee must be cleared")
	}
}

func TestApplyDetailsOnClosedTaskIsRejected(t *testing.T) {
	obj := &Task{Status: CompleteStatus, Title: "old"}
	title := "new title"

	if err := obj.ApplyDetails(&title, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error when editing closed task")
	}
	if obj.Title != "old" {
		t.Fatalf("title must not change, got %q", obj.Title)
	}
}

func TestApplyDetailsRejectsShortTitle(t *testing.T) {
	obj := &Task{Status: NewStatus, Title: "valid title"}
	title := "ab"

	if err := obj.ApplyDetails(&title, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for too short title")
	}
}

func TestApplyDetailsRejectsUnknownPriority(t *testing.T) {
	obj := &Task{Status: NewStatus, Title: "valid title"}
	priority := Priority(99)

	if err := obj.ApplyDetails(nil, nil, &priority, nil, nil); err == nil {
		t.Fatal("expected error for unknown priority")
	}
}

func closedTask() *Task {
	obj := assignedTask(ReviewStatus)
	if err := obj.ChangeStatus(CompleteStatus); err != nil {
		panic(err)
	}
	return obj
}

func TestSendToReworkReturnsClosedTaskToWork(t *testing.T) {
	obj := closedTask()

	if err := obj.SendToRework("Не учтён пустой список", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.Status != WorkingStatus {
		t.Fatalf("status = %q, want %q", obj.Status, WorkingStatus)
	}
	if obj.ClosedAt != nil {
		t.Fatal("ClosedAt must be cleared when task returns to work")
	}
	if !obj.IsInRework() {
		t.Fatal("task must be marked as being in rework")
	}
	if obj.ReworkNote != "Не учтён пустой список" {
		t.Fatalf("ReworkNote = %q", obj.ReworkNote)
	}
	if obj.ReworkByID == nil || *obj.ReworkByID != 3 {
		t.Fatal("ReworkByID must point at the author of the note")
	}
}

func TestSendToReworkWithoutAssigneeGoesToNew(t *testing.T) {
	obj := closedTask()
	obj.AssigneeID = nil

	if err := obj.SendToRework("Переделать вёрстку", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Работать некому — задача ждёт назначения, а не висит «в работе».
	if obj.Status != NewStatus {
		t.Fatalf("status = %q, want %q", obj.Status, NewStatus)
	}
}

func TestSendToReworkRejectsOpenTask(t *testing.T) {
	for _, status := range []Status{NewStatus, WorkingStatus, ReviewStatus} {
		obj := assignedTask(status)
		if err := obj.SendToRework("Дополнить тесты", 3); err == nil {
			t.Fatalf("status %q: open task must not be sent to rework", status)
		}
	}
}

func TestSendToReworkRequiresNote(t *testing.T) {
	for _, note := range []string{"", "   ", "нет"} {
		obj := closedTask()
		if err := obj.SendToRework(note, 3); err == nil {
			t.Fatalf("note %q: must be rejected", note)
		}
		// Отказ не должен менять состояние задачи.
		if obj.Status != CompleteStatus {
			t.Fatalf("note %q: status changed to %q on error", note, obj.Status)
		}
	}
}

func TestClosingAgainClearsReworkNote(t *testing.T) {
	obj := closedTask()
	if err := obj.SendToRework("Поправить отступы", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := obj.ChangeStatus(ReviewStatus); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := obj.ChangeStatus(CompleteStatus); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Замечание относилось к прошлому кругу — оно закрыто вместе с задачей.
	if obj.IsInRework() || obj.ReworkNote != "" || obj.ReworkByID != nil {
		t.Fatal("rework note must be cleared once the task is accepted")
	}
}

func TestNewTypesAreValid(t *testing.T) {
	for _, value := range []Type{BugType, FeatureType, FixType, RefactorType, UpdateType} {
		if !value.IsValid() {
			t.Fatalf("type %q must be valid", value)
		}
	}
	if Type("chore").IsValid() {
		t.Fatal("unknown type must be rejected")
	}
}

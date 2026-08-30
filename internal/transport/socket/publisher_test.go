package socket

import (
	"testing"

	sio "github.com/zishang520/socket.io/v2/socket"

	"RTM-Task/internal/app/use_cases/task_action"
)

func staffPtr(id uint) *uint { return &id }

func TestAssigneeRoomsIncludesCurrentAssignee(t *testing.T) {
	p := &Publisher{}

	rooms := p.assigneeRooms(task_action.TaskEvent{
		Task: task_action.TaskResult{AssigneeID: staffPtr(7)},
	})

	if len(rooms) != 1 {
		t.Fatalf("rooms = %v, want exactly one room", rooms)
	}
	if rooms[0] != sio.Room(assigneeRoom(7)) {
		t.Fatalf("room = %q, want %q", rooms[0], assigneeRoom(7))
	}
}

func TestAssigneeRoomsIncludesPreviousOnReassign(t *testing.T) {
	p := &Publisher{}

	rooms := p.assigneeRooms(task_action.TaskEvent{
		Task:               task_action.TaskResult{AssigneeID: staffPtr(2)},
		PreviousAssigneeID: staffPtr(1),
	})

	if len(rooms) != 2 {
		t.Fatalf("rooms = %v, want both current and previous assignee", rooms)
	}
	if rooms[0] != sio.Room(assigneeRoom(2)) || rooms[1] != sio.Room(assigneeRoom(1)) {
		t.Fatalf("rooms = %v, want [%s %s]", rooms, assigneeRoom(2), assigneeRoom(1))
	}
}

func TestAssigneeRoomsSkipsDuplicateWhenAssigneeUnchanged(t *testing.T) {
	p := &Publisher{}

	rooms := p.assigneeRooms(task_action.TaskEvent{
		Task:               task_action.TaskResult{AssigneeID: staffPtr(5)},
		PreviousAssigneeID: staffPtr(5),
	})

	if len(rooms) != 1 {
		t.Fatalf("rooms = %v, want a single room without duplicates", rooms)
	}
}

func TestAssigneeRoomsOnUnassignNotifiesPrevious(t *testing.T) {
	p := &Publisher{}

	rooms := p.assigneeRooms(task_action.TaskEvent{
		Task:               task_action.TaskResult{AssigneeID: nil},
		PreviousAssigneeID: staffPtr(3),
	})

	if len(rooms) != 1 || rooms[0] != sio.Room(assigneeRoom(3)) {
		t.Fatalf("rooms = %v, want [%s]", rooms, assigneeRoom(3))
	}
}

func TestAssigneeRoomsEmptyForUnassignedTask(t *testing.T) {
	p := &Publisher{}

	rooms := p.assigneeRooms(task_action.TaskEvent{
		Task: task_action.TaskResult{AssigneeID: nil},
	})

	if len(rooms) != 0 {
		t.Fatalf("rooms = %v, want none", rooms)
	}
}

func TestPublishTaskOnNilPublisherIsNoop(t *testing.T) {
	var p *Publisher

	// Не должно паниковать: publisher может быть не подключён.
	p.PublishTask(nil, task_action.TaskEvent{Name: task_action.EventTaskCreated})
}

func TestAssigneeRoomNaming(t *testing.T) {
	if got := assigneeRoom(42); got != "tasks:assignee:42" {
		t.Fatalf("assigneeRoom(42) = %q, want %q", got, "tasks:assignee:42")
	}
}

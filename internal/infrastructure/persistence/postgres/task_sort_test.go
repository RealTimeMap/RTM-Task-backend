package postgres

import (
	"strings"
	"testing"

	"RTM-Task/internal/domain/task"
)

func TestOrderClauseDefault(t *testing.T) {
	// Клиент без сортировки должен получать прежний порядок:
	// сначала важные, внутри приоритета — свежие.
	if got := orderClause(task.Sort{}); got != "priority ASC, created_at DESC" {
		t.Fatalf("clause = %q", got)
	}
}

func TestOrderClauseUnknownFieldFallsBack(t *testing.T) {
	// Домен такое не пропустит, но репозиторий не должен собирать
	// запрос из неизвестного поля даже если его позвали напрямую.
	if got := orderClause(task.Sort{Field: "title"}); got != "priority ASC, created_at DESC" {
		t.Fatalf("clause = %q", got)
	}
}

func TestOrderClauseDirections(t *testing.T) {
	cases := []struct {
		sort task.Sort
		want string
	}{
		{task.Sort{Field: task.SortByCreatedAt}, "created_at DESC, id DESC"},
		{task.Sort{Field: task.SortByCreatedAt, Order: task.AscOrder}, "created_at ASC, id DESC"},
		{task.Sort{Field: task.SortByPriority}, "priority ASC, id DESC"},
		{task.Sort{Field: task.SortByPriority, Order: task.DescOrder}, "priority DESC, id DESC"},
	}

	for _, c := range cases {
		if got := orderClause(c.sort); got != c.want {
			t.Fatalf("sort %+v: clause = %q, want %q", c.sort, got, c.want)
		}
	}
}

func TestOrderClauseUsesLifecycleOrderForStatus(t *testing.T) {
	got := orderClause(task.Sort{Field: task.SortByStatus})

	// По алфавиту complete оказался бы первым, а нужен порядок
	// жизненного цикла — поэтому в запросе CASE, а не имя колонки.
	if !strings.Contains(got, "CASE status") {
		t.Fatalf("clause = %q, want a CASE expression", got)
	}

	for i, status := range []string{"'new'", "'working'", "'review'", "'complete'"} {
		if !strings.Contains(got, status) {
			t.Fatalf("clause is missing %s: %q", status, got)
		}
		// Позиции в выражении должны идти в том же порядке, что и цикл.
		if i > 0 {
			previous := []string{"'new'", "'working'", "'review'", "'complete'"}[i-1]
			if strings.Index(got, previous) > strings.Index(got, status) {
				t.Fatalf("status %s stands before %s: %q", status, previous, got)
			}
		}
	}
}

func TestOrderClauseUsesDomainOrderForType(t *testing.T) {
	got := orderClause(task.Sort{Field: task.SortByType})
	if !strings.Contains(got, "CASE type") {
		t.Fatalf("clause = %q, want a CASE expression", got)
	}

	types := []string{"'bug'", "'feature'", "'fix'", "'refactor'", "'update'"}
	for i := 1; i < len(types); i++ {
		if strings.Index(got, types[i-1]) > strings.Index(got, types[i]) {
			t.Fatalf("type %s stands before %s: %q", types[i], types[i-1], got)
		}
	}
}

func TestOrderClauseAlwaysBreaksTiesByID(t *testing.T) {
	// Без второго ключа страницы с одинаковыми значениями поля могли бы
	// перемешиваться между запросами и дублировать задачи.
	for _, field := range []task.SortField{
		task.SortByCreatedAt,
		task.SortByPriority,
		task.SortByStatus,
		task.SortByType,
	} {
		got := orderClause(task.Sort{Field: field})
		if !strings.HasSuffix(got, ", id DESC") {
			t.Fatalf("field %q: clause = %q, want id tiebreaker", field, got)
		}
	}
}

func TestOrderClauseNeverEmbedsRawInput(t *testing.T) {
	// Поле и направление в запрос не подставляются: выражение
	// собирается из констант по switch.
	got := orderClause(task.Sort{Field: "id; DROP TABLE tasks", Order: "; DROP TABLE tasks"})
	if strings.Contains(got, "DROP") {
		t.Fatalf("clause leaked input: %q", got)
	}
}

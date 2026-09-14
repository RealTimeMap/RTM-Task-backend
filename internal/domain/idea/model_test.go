package idea

import (
	"testing"
	"time"
)

func TestNewValidatesTitle(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"пустой заголовок отклоняется", "", true},
		{"пробелы — тот же пустой заголовок", "   ", true},
		{"слишком короткий отклоняется", "ид", true},
		{"минимально допустимый принимается", "иде", false},
		{"обычный принимается", "Кэшировать список задач", false},
		{"слишком длинный отклоняется", string(make([]rune, MaxTitleLength+1)), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(1, tc.title, "")
			if tc.wantErr && err == nil {
				t.Fatalf("ожидали ошибку для %q, получили nil", tc.title)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("неожиданная ошибка для %q: %v", tc.title, err)
			}
		})
	}
}

// Заголовок и описание чистятся от краевых пробелов: иначе идея
// «  Кэш  » и «Кэш» выглядели бы в списке одинаково, но не совпадали
// при поиске.
func TestNewTrimsText(t *testing.T) {
	obj, err := New(1, "  Кэшировать список  ", "  подробности  ")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if obj.Title != "Кэшировать список" {
		t.Errorf("заголовок не очищен: %q", obj.Title)
	}
	if obj.Description != "подробности" {
		t.Errorf("описание не очищено: %q", obj.Description)
	}
	if obj.Done {
		t.Error("новая идея не может быть выполненной")
	}
}

func TestMarkDoneRecordsWhoAndWhen(t *testing.T) {
	obj, err := New(1, "Кэшировать список", "")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	before := time.Now()
	obj.MarkDone(7)

	if !obj.Done {
		t.Fatal("идея должна была стать выполненной")
	}
	if obj.DoneByID == nil || *obj.DoneByID != 7 {
		t.Errorf("не записан тот, кто закрыл идею: %v", obj.DoneByID)
	}
	if obj.DoneAt == nil || obj.DoneAt.Before(before.Add(-time.Second)) {
		t.Errorf("не записано время закрытия: %v", obj.DoneAt)
	}
}

// Повторная отметка не переписывает автора закрытия: первым закрыл —
// он и остаётся в записи, иначе подпись под выполненной идеей врала бы.
func TestMarkDoneKeepsFirstCloser(t *testing.T) {
	obj, _ := New(1, "Кэшировать список", "")
	obj.MarkDone(7)
	first := *obj.DoneAt

	obj.MarkDone(9)

	if *obj.DoneByID != 7 {
		t.Errorf("автор закрытия переписан: %d", *obj.DoneByID)
	}
	if !obj.DoneAt.Equal(first) {
		t.Error("время закрытия переписано")
	}
}

// Возврат в работу стирает отметку целиком: оставшаяся подпись
// говорила бы, что идею кто-то закрыл, хотя она снова открыта.
func TestReopenClearsMark(t *testing.T) {
	obj, _ := New(1, "Кэшировать список", "")
	obj.MarkDone(7)
	obj.Reopen()

	if obj.Done {
		t.Error("идея должна была вернуться в работу")
	}
	if obj.DoneAt != nil || obj.DoneByID != nil {
		t.Errorf("отметка не стёрта: at=%v by=%v", obj.DoneAt, obj.DoneByID)
	}
}

// Повторный возврат ничего не ломает: кнопку могли нажать дважды.
func TestReopenIsIdempotent(t *testing.T) {
	obj, _ := New(1, "Кэшировать список", "")
	obj.Reopen()

	if obj.Done || obj.DoneAt != nil || obj.DoneByID != nil {
		t.Error("возврат уже открытой идеи изменил её состояние")
	}
}

// Пустое описание — осмысленное действие: идею могли записать наспех,
// и стереть подробности должно быть можно.
func TestDescribeAcceptsEmpty(t *testing.T) {
	obj, _ := New(1, "Кэшировать список", "было описание")

	if err := obj.Describe(""); err != nil {
		t.Fatalf("стирание описания отклонено: %v", err)
	}
	if obj.Description != "" {
		t.Errorf("описание не стёрто: %q", obj.Description)
	}
}

func TestRenameValidates(t *testing.T) {
	obj, _ := New(1, "Кэшировать список", "")

	if err := obj.Rename(""); err == nil {
		t.Error("пустой заголовок должен отклоняться")
	}
	if obj.Title != "Кэшировать список" {
		t.Errorf("отклонённая правка изменила заголовок: %q", obj.Title)
	}

	if err := obj.Rename("Новый заголовок"); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if obj.Title != "Новый заголовок" {
		t.Errorf("заголовок не изменён: %q", obj.Title)
	}
}

func TestCommentRewriteMarksEdit(t *testing.T) {
	c, err := NewComment(1, 2, "первая мысль")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if c.IsEdited() {
		t.Error("новая реплика не может быть правленой")
	}

	if err := c.Rewrite("вторая мысль"); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !c.IsEdited() {
		t.Error("правка не отмечена")
	}
}

// Тот же текст правкой не считается: иначе «отредактировано» появлялось
// бы после случайного повторного сохранения без единого изменения.
func TestCommentRewriteSameTextIsNotEdit(t *testing.T) {
	c, _ := NewComment(1, 2, "мысль")

	if err := c.Rewrite("мысль"); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if c.IsEdited() {
		t.Error("сохранение того же текста отмечено как правка")
	}
}

func TestCommentRequiresBody(t *testing.T) {
	if _, err := NewComment(1, 2, "   "); err == nil {
		t.Error("пустая реплика должна отклоняться")
	}
}

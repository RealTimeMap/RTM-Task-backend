package task

import (
	"context"
	"testing"

	"RTM-Task/internal/utils/apperror"
	"RTM-Task/internal/utils/pagination"
)

func TestSortFieldValidity(t *testing.T) {
	valid := []SortField{SortByCreatedAt, SortByPriority, SortByStatus, SortByType}
	for _, field := range valid {
		if !field.IsValid() {
			t.Fatalf("field %q must be valid", field)
		}
	}

	// Имя колонки БД не должно проходить как поле сортировки: контракт
	// API — camelCase, и совпадение с колонкой здесь случайно.
	for _, field := range []SortField{"", "created_at", "title", "id; DROP TABLE tasks"} {
		if SortField(field).IsValid() {
			t.Fatalf("field %q must be rejected", field)
		}
	}
}

func TestSortOrderValidity(t *testing.T) {
	if !AscOrder.IsValid() || !DescOrder.IsValid() {
		t.Fatal("asc and desc must be valid")
	}
	for _, order := range []SortOrder{"", "ASC", "up", "random()"} {
		if order.IsValid() {
			t.Fatalf("order %q must be rejected", order)
		}
	}
}

func TestSortNormalizePicksNaturalOrder(t *testing.T) {
	// Дата читается от свежих к старым, остальные шкалы — от начала.
	cases := map[SortField]SortOrder{
		SortByCreatedAt: DescOrder,
		SortByPriority:  AscOrder,
		SortByStatus:    AscOrder,
		SortByType:      AscOrder,
	}

	for field, expected := range cases {
		got := Sort{Field: field}.Normalize()
		if got.Order != expected {
			t.Fatalf("field %q: order = %q, want %q", field, got.Order, expected)
		}
	}
}

func TestSortNormalizeKeepsExplicitOrder(t *testing.T) {
	got := Sort{Field: SortByCreatedAt, Order: AscOrder}.Normalize()
	if got.Order != AscOrder {
		t.Fatalf("order = %q, want asc", got.Order)
	}
}

func TestSortNormalizeRepairsBrokenOrder(t *testing.T) {
	// Мусор в направлении не должен доезжать до запроса: подставляем
	// естественное для поля.
	got := Sort{Field: SortByPriority, Order: "sideways"}.Normalize()
	if got.Order != AscOrder {
		t.Fatalf("order = %q, want asc", got.Order)
	}
}

func TestZeroSortMeansDefault(t *testing.T) {
	if !(Sort{}).IsZero() {
		t.Fatal("zero sort must report itself as zero")
	}
	if (Sort{Field: SortByStatus}).IsZero() {
		t.Fatal("sort with a field must not be zero")
	}
	// Направление без поля бессмысленно и тоже считается пустым:
	// сортировать «по возрастанию чего-то» нельзя.
	if !(Sort{Order: AscOrder}).IsZero() {
		t.Fatal("order without a field must be treated as zero")
	}
}

func TestListRejectsUnknownSortField(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)

	_, _, err := svc.List(context.Background(), Filter{
		Sort:       Sort{Field: "title"},
		Pagination: pagination.New(20, 0),
	})
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestListRejectsUnknownSortOrder(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)

	_, _, err := svc.List(context.Background(), Filter{
		Sort:       Sort{Field: SortByPriority, Order: "sideways"},
		Pagination: pagination.New(20, 0),
	})
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindValidation {
		t.Fatalf("err = %v, want validation error", err)
	}
}

func TestListAcceptsValidSort(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)
	seedTask(svc, 1, nil)

	for _, field := range []SortField{SortByCreatedAt, SortByPriority, SortByStatus, SortByType} {
		if _, _, err := svc.List(context.Background(), Filter{
			Sort:       Sort{Field: field},
			Pagination: pagination.New(20, 0),
		}); err != nil {
			t.Fatalf("field %q: unexpected error: %v", field, err)
		}
	}
}

func TestListAcceptsEmptySort(t *testing.T) {
	svc := newTestServiceFull(newFakeRepository(), nil)

	// Клиент, ничего не знающий о сортировке, должен получать список
	// в порядке по умолчанию, а не ошибку.
	if _, _, err := svc.List(context.Background(), Filter{
		Pagination: pagination.New(20, 0),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

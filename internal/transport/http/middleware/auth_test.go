package middleware

import (
	"net/http"
	"testing"

	"RTM-Task/internal/utils/apperror"
)

func headers(values map[string]string) http.Header {
	h := http.Header{}
	for name, value := range values {
		h.Set(name, value)
	}
	return h
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

func TestIdentityFromHeadersAcceptsAdmin(t *testing.T) {
	identity, err := IdentityFromHeaders(headers(map[string]string{
		HeaderUserID:    "42",
		HeaderUserName:  "Mira",
		HeaderUserAdmin: "true",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", identity.UserID)
	}
	if identity.UserName != "Mira" {
		t.Fatalf("UserName = %q, want %q", identity.UserName, "Mira")
	}
	if !identity.IsAdmin {
		t.Fatal("IsAdmin must be true")
	}
}

func TestIdentityFromHeadersDecodesPercentEncodedName(t *testing.T) {
	// Кириллица в заголовке приходит percent-encoded: HTTP-заголовки
	// не переносят не-ASCII.
	identity, err := IdentityFromHeaders(headers(map[string]string{
		HeaderUserID:    "2",
		HeaderUserName:  "%D0%9C%D0%B8%D1%80%D0%B0",
		HeaderUserAdmin: "true",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.UserName != "Мира" {
		t.Fatalf("UserName = %q, want %q", identity.UserName, "Мира")
	}
}

func TestIdentityFromHeadersKeepsPlainNameWithPercent(t *testing.T) {
	// Значение с процентом, но не являющееся корректной кодировкой,
	// должно остаться как есть, а не потеряться.
	identity, err := IdentityFromHeaders(headers(map[string]string{
		HeaderUserID:    "2",
		HeaderUserName:  "100% Admin",
		HeaderUserAdmin: "true",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.UserName != "100% Admin" {
		t.Fatalf("UserName = %q, want %q", identity.UserName, "100% Admin")
	}
}

func TestIdentityFromHeadersRejectsMissingID(t *testing.T) {
	_, err := IdentityFromHeaders(headers(map[string]string{
		HeaderUserName:  "Mira",
		HeaderUserAdmin: "true",
	}))

	requireKind(t, err, apperror.KindUnauthorized)
}

func TestIdentityFromHeadersRejectsInvalidID(t *testing.T) {
	for _, value := range []string{"0", "-1", "abc"} {
		_, err := IdentityFromHeaders(headers(map[string]string{
			HeaderUserID:    value,
			HeaderUserName:  "Mira",
			HeaderUserAdmin: "true",
		}))

		requireKind(t, err, apperror.KindUnauthorized)
	}
}

func TestIdentityFromHeadersRejectsMissingName(t *testing.T) {
	_, err := IdentityFromHeaders(headers(map[string]string{
		HeaderUserID:    "2",
		HeaderUserAdmin: "true",
	}))

	requireKind(t, err, apperror.KindUnauthorized)
}

func TestIdentityFromHeadersRejectsNonAdmin(t *testing.T) {
	for _, value := range []string{"false", "", "yes"} {
		_, err := IdentityFromHeaders(headers(map[string]string{
			HeaderUserID:    "2",
			HeaderUserName:  "Mira",
			HeaderUserAdmin: value,
		}))

		requireKind(t, err, apperror.KindForbidden)
	}
}

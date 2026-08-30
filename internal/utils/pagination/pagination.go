package pagination

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Params — параметры постраничной выборки, нормализованные к безопасным значениям.
type Params struct {
	Limit  int
	Offset int
}

// New нормализует пришедшие извне значения: отрицательные и нулевые
// заменяются дефолтами, слишком большой limit урезается.
func New(limit, offset int) Params {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return Params{Limit: limit, Offset: offset}
}

// FromPage строит параметры из номера страницы (1-based).
func FromPage(page, size int) Params {
	if page <= 0 {
		page = 1
	}
	p := New(size, 0)
	p.Offset = (page - 1) * p.Limit
	return p
}

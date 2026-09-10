package middleware

import (
	"github.com/gin-gonic/gin"
)

// Заголовки управления кэшем.
const (
	headerCacheControl = "Cache-Control"
	headerPragma       = "Pragma"
	headerExpires      = "Expires"
	headerVary         = "Vary"
)

// noStore запрещает хранить ответ где бы то ни было.
//
// private важен отдельно от no-store: без него общий кэш (прокси
// провайдера, корпоративный шлюз) вправе счесть ответ общим для всех.
// Ответы этого API зависят от того, кто спрашивает, и такая находка
// означала бы показ чужих задач.
const noStore = "no-store, no-cache, must-revalidate, private"

// NoCache запрещает кэшировать ответы API.
//
// Данные задач меняются в реальном времени и зависят от пользователя, а
// сервис не отдавал ни Cache-Control, ни ETag. Без явных указаний
// браузер применяет эвристику из RFC 9111 и вправе считать ответ свежим
// столько, сколько сочтёт нужным, — отсюда и расходящаяся картина у
// разных людей, которая лечилась только полной очисткой кэша.
//
// Заголовок ставится до обработчика: после записи тела менять их поздно,
// а gin отдаёт ответ потоком.
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(headerCacheControl, noStore)
		// Pragma и Expires — для старых промежуточных узлов, которые
		// Cache-Control не понимают: они кэшируют по HTTP/1.0 правилам.
		c.Header(headerPragma, "no-cache")
		c.Header(headerExpires, "0")

		// Ответ зависит от того, чей токен пришёл. Vary заставляет любой
		// кэш на пути различать пользователей, а не отдавать первому
		// попавшемуся то, что положил туда другой.
		//
		// Add, а не Set: CORS-слой шлюза уже проставляет Vary: Origin,
		// и затирать его нельзя.
		c.Writer.Header().Add(headerVary, "Authorization")
		c.Writer.Header().Add(headerVary, "X-User-ID")

		c.Next()
	}
}

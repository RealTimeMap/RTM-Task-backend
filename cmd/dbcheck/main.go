package main

import (
	"fmt"

	"RTM-Task/internal/config"
	"RTM-Task/internal/utils/database"
	"RTM-Task/internal/utils/logger"
)

func main() {
	cfg := config.MustLoad()
	log := logger.New(cfg.Env)
	db := database.MustNew(cfg.Database, log)

	var tasksTotal, tasksAlive int64
	db.Raw("SELECT COUNT(*) FROM tasks").Scan(&tasksTotal)
	db.Raw("SELECT COUNT(*) FROM tasks WHERE deleted_at IS NULL").Scan(&tasksAlive)
	fmt.Println("RESULT tasks: rows=", tasksTotal, "alive=", tasksAlive)

	for _, table := range []string{"task_comments", "task_checklist_items"} {
		var total, alive, orphans, dangling int64

		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&total)
		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", table)).Scan(&alive)
		db.Raw(fmt.Sprintf(
			"SELECT COUNT(*) FROM %s c WHERE NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = c.task_id)",
			table,
		)).Scan(&orphans)
		// Живые дочерние записи, чья задача уже помечена удалённой.
		db.Raw(fmt.Sprintf(
			"SELECT COUNT(*) FROM %s c WHERE c.deleted_at IS NULL AND EXISTS "+
				"(SELECT 1 FROM tasks t WHERE t.id = c.task_id AND t.deleted_at IS NOT NULL)",
			table,
		)).Scan(&dangling)

		fmt.Println("RESULT", table, "rows=", total, "alive=", alive, "orphans=", orphans, "dangling=", dangling)
	}
}

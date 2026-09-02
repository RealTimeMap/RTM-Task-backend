package postgres

import (
	"fmt"

	"gorm.io/gorm"

	"RTM-Task/internal/domain/role"
	"RTM-Task/internal/domain/task"
)

// AutoMigrate приводит схему БД в соответствие доменным моделям.
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&role.Staff{},
		&task.Task{},
		&task.Comment{},
		&task.ChecklistItem{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

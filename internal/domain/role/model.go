package role

import "gorm.io/gorm"

// Role — роль сотрудника в системе. Value object: сравнивается по значению
// и сама по себе несёт все правила доступа.
type Role string

const (
	AdminRole     Role = "admin"
	ManagerRole   Role = "manager"
	DeveloperRole Role = "developer"
	ViewerRole    Role = "viewer"
)

// IsValid сообщает, входит ли значение в список известных ролей.
func (r Role) IsValid() bool {
	switch r {
	case AdminRole, ManagerRole, DeveloperRole, ViewerRole:
		return true
	default:
		return false
	}
}

func (r Role) String() string { return string(r) }

// CanCreateTask — кто вправе заводить задачи.
func (r Role) CanCreateTask() bool {
	return r == AdminRole || r == ManagerRole || r == DeveloperRole
}

// CanAssignTask — кто вправе назначать исполнителя.
// Разработчик может взять задачу себе, но не раздавать её другим,
// поэтому проверка «на себя» остаётся на уровне доменного сервиса task.
func (r Role) CanAssignAnyone() bool {
	return r == AdminRole || r == ManagerRole
}

// CanDeleteTask — удаление доступно только administrative-ролям.
func (r Role) CanDeleteTask() bool {
	return r == AdminRole || r == ManagerRole
}

// CanCloseForeignTask — закрывать чужую задачу может только управляющая роль.
func (r Role) CanCloseForeignTask() bool {
	return r == AdminRole || r == ManagerRole
}

// CanEditForeignTask — редактировать чужую задачу может только управляющая роль.
func (r Role) CanEditForeignTask() bool {
	return r == AdminRole || r == ManagerRole
}

// Identity — пользователь платформы, пришедший из auth-service через шлюз.
// Это то, что известно о нём до обращения к таблице сотрудников.
type Identity struct {
	// UserID — глобальный идентификатор пользователя платформы.
	// Он же является идентификатором сотрудника: staff.id = user_id.
	UserID   uint
	UserName string
	IsAdmin  bool
}

// Staff — сотрудник, участник рабочего процесса. Корень агрегата:
// вне него роль не живёт и не меняется.
//
// ID сотрудника не генерируется базой: он равен идентификатору пользователя
// платформы, что позволяет связывать задачи с людьми без отдельной таблицы
// соответствий.
type Staff struct {
	gorm.Model

	// Email приходит не всегда: сотрудник заводится по данным шлюза,
	// а там его может не быть. Пустое значение хранится как NULL —
	// иначе уникальный индекс считал бы все такие записи одинаковыми
	// и пустил бы в систему только первого сотрудника без почты.
	Email    *string `gorm:"type:varchar(255);index"`
	FullName string  `gorm:"type:varchar(200);not null"`
	Role     Role    `gorm:"type:varchar(20);not null;default:'viewer';index"`
	IsActive bool    `gorm:"not null;default:true;index"`
}

func (Staff) TableName() string { return "staff" }

// Actor — снимок сотрудника, выполняющего операцию. Передаётся в доменные
// сервисы вместо полной сущности: домену для проверки прав нужны только
// идентификатор и роль, а не состояние из базы.
type Actor struct {
	StaffID uint
	Role    Role
}

// AsActor превращает сотрудника в участника операции.
func (s *Staff) AsActor() Actor {
	return Actor{StaffID: s.ID, Role: s.Role}
}

// Is сообщает, совпадает ли участник с указанным идентификатором сотрудника.
func (a Actor) Is(staffID uint) bool { return a.StaffID == staffID }

package staff_action

// Application собирает use case'ы работы с сотрудниками.
//
// Создания здесь нет: сотрудник заводится доменным сервисом при первом
// обращении пользователя платформы (см. role.Service.ResolveActor).
type Application struct {
	GetStaff   *GetStaffHandler
	ListStaff  *ListStaffHandler
	ChangeRole *ChangeRoleHandler
	Deactivate *DeactivateStaffHandler
}

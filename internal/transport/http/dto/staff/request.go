package staff

// ChangeRoleRequest — тело запроса на смену роли сотрудника.
type ChangeRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=admin manager developer viewer"`
}

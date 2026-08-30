package staff_action

import (
	"time"

	"RTM-Task/internal/domain/role"
)

type StaffResult struct {
	ID        uint
	Email     string
	FullName  string
	Role      string
	IsActive  bool
	CreatedAt time.Time
}

func toStaffResult(obj *role.Staff) StaffResult {
	if obj == nil {
		return StaffResult{}
	}
	email := ""
	if obj.Email != nil {
		email = *obj.Email
	}

	return StaffResult{
		ID:        obj.ID,
		Email:     email,
		FullName:  obj.FullName,
		Role:      obj.Role.String(),
		IsActive:  obj.IsActive,
		CreatedAt: obj.CreatedAt,
	}
}

func toStaffResults(objs []*role.Staff) []StaffResult {
	results := make([]StaffResult, 0, len(objs))
	for _, obj := range objs {
		results = append(results, toStaffResult(obj))
	}
	return results
}

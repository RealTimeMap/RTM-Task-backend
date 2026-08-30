package staff

import (
	"time"

	"RTM-Task/internal/app/use_cases/staff_action"
)

// StaffResponse — представление сотрудника в API.
type StaffResponse struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"fullName"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewStaffResponse(result staff_action.StaffResult) StaffResponse {
	return StaffResponse{
		ID:        result.ID,
		Email:     result.Email,
		FullName:  result.FullName,
		Role:      result.Role,
		IsActive:  result.IsActive,
		CreatedAt: result.CreatedAt,
	}
}

type StaffListResponse struct {
	Items []StaffResponse `json:"items"`
}

func NewStaffListResponse(results []staff_action.StaffResult) StaffListResponse {
	items := make([]StaffResponse, 0, len(results))
	for _, result := range results {
		items = append(items, NewStaffResponse(result))
	}
	return StaffListResponse{Items: items}
}

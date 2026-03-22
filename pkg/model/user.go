package model

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`    // never expose
	Role     string `json:"role"` // "admin", "manager", "employee"
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

type Permission struct {
	Resource  string `json:"resource"`
	CanView   bool   `json:"can_view"`
	CanAdd    bool   `json:"can_add"`
	CanEdit   bool   `json:"can_edit"`
	CanDelete bool   `json:"can_delete"`
}

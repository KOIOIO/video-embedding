package adminauth

import "time"

const (
	UserTypeNormal int16 = 2
	UserTypeAdmin  int16 = 3
)

type Admin struct {
	ID           uint64 `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	RealName     string `json:"real_name"`
	UserType     int16  `json:"user_type"`
	Status       int16  `json:"-"`
	Deleted      int16  `json:"-"`
}

type LoginResult struct {
	Token     string    `json:"access_token"`
	ExpiresAt time.Time `json:"expires_at"`
	Admin     Admin     `json:"admin"`
}

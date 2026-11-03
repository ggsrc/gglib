package mctx

import (
	"fmt"
)

// NewUser returns a User with the given app, user and device IDs.
func NewUser(appID int32, userID string, deviceID string) User {
	return User{
		AppID:    appID,
		UserID:   userID,
		DeviceID: deviceID,
	}
}

// User identifies the caller of a request.
type User struct {
	AppID    int32  `json:"app_id"`
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`

	Address     string `json:"address"`
	SignAt      string `json:"sign_at"`
	Signature   string `json:"signature"`
	AccessToken string `json:"access_token"`

	JwtString string `json:"-"`
}

// String returns the user as "app<AppID>/u<UserID>/d<DeviceID>".
func (u User) String() string {
	return fmt.Sprintf("app%d/u%s/d%s", u.AppID, u.UserID, u.DeviceID)
}

// GetAppID returns the user's app ID.
func (u User) GetAppID() int32 {
	return u.AppID
}

// IsLogin reports whether the user is signed in, that is, has a user ID.
func (u User) IsLogin() bool {
	return u.UserID != ""
}

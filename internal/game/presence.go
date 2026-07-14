package game

type ConnectedUser struct {
	UserID   string
	Username string
}

type PresenceSnapshot struct {
	ConnectedUsers int
	Users          []ConnectedUser
}

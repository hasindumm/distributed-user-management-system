package userclient

const (
	SubjectCreateUser     = "users.v1.create"
	SubjectGetUserByID    = "users.v1.get.by_id"
	SubjectGetUserByEmail = "users.v1.get.by_email"
	SubjectListUsers      = "users.v1.list"
	SubjectListAllUsers   = "users.v1.list.all"
	SubjectUpdateUser     = "users.v1.update"
	SubjectDeleteUser     = "users.v1.delete"

	SubjectUserCreated = "users.v1.events.created"
	SubjectUserUpdated = "users.v1.events.updated"
	SubjectUserDeleted = "users.v1.events.deleted"
)

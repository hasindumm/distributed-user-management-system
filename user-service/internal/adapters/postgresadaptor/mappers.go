package postgresadaptor

import (
	"database/sql"
	"time"
	"user-service/internal/adapters/postgresadaptor/db"
	"user-service/internal/domain"
)

func toNullString(s *string) sql.NullString {

	if s == nil {
		return sql.NullString{
			String: "",
			Valid:  false,
		}
	}

	return sql.NullString{
		String: *s,
		Valid:  true,
	}
}

func toNullInt32(i *int) sql.NullInt32 {
	if i == nil {
		return sql.NullInt32{
			Valid: false,
		}
	}
	return sql.NullInt32{
		Int32: int32(*i),
		Valid: true,
	}
}

func fromNullString(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func fromNullInt32(i sql.NullInt32) *int {
	if !i.Valid {
		return nil
	}
	v := int(i.Int32)
	return &v
}

func fromNullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func toDomainUser(u db.User) domain.User {
	return domain.User{
		UserId:    u.UserID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		Phone:     fromNullString(u.Phone),
		Age:       fromNullInt32(u.Age),
		Status:    domain.UserStatus(u.Status),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		DeletedAt: fromNullTime(u.DeletedAt),
	}

}

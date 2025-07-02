package db

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	LoginID   string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"login_id"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"`
	Email     string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`

	Roles  []UserRoleType  `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	Scopes []UserScopeType `gorm:"many2many:user_scopes;" json:"scopes,omitempty"`
}

func (User) TableName() string {
	return "users"
}

type UserRepository interface {
	Create(user *User) error
	GetByID(id uint) (*User, error)
	GetByLoginID(loginID string) (*User, error)
	GetByEmail(email string) (*User, error)
	Update(user *User) error
	Delete(id uint) error
	List(limit, offset int) ([]User, error)
	GetWithRoles(id uint) (*User, error)
	GetWithScopes(id uint) (*User, error)
	GetWithRolesAndScopes(id uint) (*User, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) GetByID(id uint) (*User, error) {
	var user User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByLoginID(loginID string) (*User, error) {
	var user User
	err := r.db.Where("login_id = ?", loginID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(email string) (*User, error) {
	var user User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(user *User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) Delete(id uint) error {
	return r.db.Delete(&User{}, id).Error
}

func (r *userRepository) List(limit, offset int) ([]User, error) {
	var users []User
	err := r.db.Limit(limit).Offset(offset).Find(&users).Error
	return users, err
}

func (r *userRepository) GetWithRoles(id uint) (*User, error) {
	var user User
	err := r.db.Preload("Roles").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetWithScopes(id uint) (*User, error) {
	var user User
	err := r.db.Preload("Scopes").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetWithRolesAndScopes(id uint) (*User, error) {
	var user User
	err := r.db.Preload("Roles").Preload("Scopes").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

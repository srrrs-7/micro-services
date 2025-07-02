package db

import (
	"time"

	"gorm.io/gorm"
)

type UserRoleType struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Role      string    `gorm:"type:varchar(16);uniqueIndex;not null" json:"role"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`

	Users []User `gorm:"many2many:user_roles;" json:"users,omitempty"`
}

func (UserRoleType) TableName() string {
	return "user_role_types"
}

type UserRoleTypeRepository interface {
	Create(roleType *UserRoleType) error
	GetByID(id uint) (*UserRoleType, error)
	GetByRole(role string) (*UserRoleType, error)
	Update(roleType *UserRoleType) error
	Delete(id uint) error
	List() ([]UserRoleType, error)
	GetWithUsers(id uint) (*UserRoleType, error)
}

type userRoleTypeRepository struct {
	db *gorm.DB
}

func NewUserRoleTypeRepository(db *gorm.DB) UserRoleTypeRepository {
	return &userRoleTypeRepository{db: db}
}

func (r *userRoleTypeRepository) Create(roleType *UserRoleType) error {
	return r.db.Create(roleType).Error
}

func (r *userRoleTypeRepository) GetByID(id uint) (*UserRoleType, error) {
	var roleType UserRoleType
	err := r.db.First(&roleType, id).Error
	if err != nil {
		return nil, err
	}
	return &roleType, nil
}

func (r *userRoleTypeRepository) GetByRole(role string) (*UserRoleType, error) {
	var roleType UserRoleType
	err := r.db.Where("role = ?", role).First(&roleType).Error
	if err != nil {
		return nil, err
	}
	return &roleType, nil
}

func (r *userRoleTypeRepository) Update(roleType *UserRoleType) error {
	return r.db.Save(roleType).Error
}

func (r *userRoleTypeRepository) Delete(id uint) error {
	return r.db.Delete(&UserRoleType{}, id).Error
}

func (r *userRoleTypeRepository) List() ([]UserRoleType, error) {
	var roleTypes []UserRoleType
	err := r.db.Find(&roleTypes).Error
	return roleTypes, err
}

func (r *userRoleTypeRepository) GetWithUsers(id uint) (*UserRoleType, error) {
	var roleType UserRoleType
	err := r.db.Preload("Users").First(&roleType, id).Error
	if err != nil {
		return nil, err
	}
	return &roleType, nil
}

package db

import (
	"time"

	"gorm.io/gorm"
)

type UserRole struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	RoleID    uint      `gorm:"primaryKey" json:"role_id"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`

	User User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role UserRoleType `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

func (UserRole) TableName() string {
	return "user_roles"
}

type UserRoleRepository interface {
	Create(userRole *UserRole) error
	GetByUserID(userID uint) ([]UserRole, error)
	GetByRoleID(roleID uint) ([]UserRole, error)
	GetByUserAndRole(userID, roleID uint) (*UserRole, error)
	Delete(userID, roleID uint) error
	DeleteByUserID(userID uint) error
	DeleteByRoleID(roleID uint) error
	List() ([]UserRole, error)
	GetWithUserAndRole(userID, roleID uint) (*UserRole, error)
}

type userRoleRepository struct {
	db *gorm.DB
}

func NewUserRoleRepository(db *gorm.DB) UserRoleRepository {
	return &userRoleRepository{db: db}
}

func (r *userRoleRepository) Create(userRole *UserRole) error {
	return r.db.Create(userRole).Error
}

func (r *userRoleRepository) GetByUserID(userID uint) ([]UserRole, error) {
	var userRoles []UserRole
	err := r.db.Where("user_id = ?", userID).Find(&userRoles).Error
	return userRoles, err
}

func (r *userRoleRepository) GetByRoleID(roleID uint) ([]UserRole, error) {
	var userRoles []UserRole
	err := r.db.Where("role_id = ?", roleID).Find(&userRoles).Error
	return userRoles, err
}

func (r *userRoleRepository) GetByUserAndRole(userID, roleID uint) (*UserRole, error) {
	var userRole UserRole
	err := r.db.Where("user_id = ? AND role_id = ?", userID, roleID).First(&userRole).Error
	if err != nil {
		return nil, err
	}
	return &userRole, nil
}

func (r *userRoleRepository) Delete(userID, roleID uint) error {
	return r.db.Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&UserRole{}).Error
}

func (r *userRoleRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&UserRole{}).Error
}

func (r *userRoleRepository) DeleteByRoleID(roleID uint) error {
	return r.db.Where("role_id = ?", roleID).Delete(&UserRole{}).Error
}

func (r *userRoleRepository) List() ([]UserRole, error) {
	var userRoles []UserRole
	err := r.db.Find(&userRoles).Error
	return userRoles, err
}

func (r *userRoleRepository) GetWithUserAndRole(userID, roleID uint) (*UserRole, error) {
	var userRole UserRole
	err := r.db.Preload("User").Preload("Role").Where("user_id = ? AND role_id = ?", userID, roleID).First(&userRole).Error
	if err != nil {
		return nil, err
	}
	return &userRole, nil
}

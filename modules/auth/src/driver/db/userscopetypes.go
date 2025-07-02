package db

import (
	"time"

	"gorm.io/gorm"
)

type UserScopeType struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Scope     string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"scope"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`

	Users []User `gorm:"many2many:user_scopes;" json:"users,omitempty"`
}

func (UserScopeType) TableName() string {
	return "user_scope_types"
}

type UserScopeTypeRepository interface {
	Create(scopeType *UserScopeType) error
	GetByID(id uint) (*UserScopeType, error)
	GetByScope(scope string) (*UserScopeType, error)
	Update(scopeType *UserScopeType) error
	Delete(id uint) error
	List() ([]UserScopeType, error)
	GetWithUsers(id uint) (*UserScopeType, error)
}

type userScopeTypeRepository struct {
	db *gorm.DB
}

func NewUserScopeTypeRepository(db *gorm.DB) UserScopeTypeRepository {
	return &userScopeTypeRepository{db: db}
}

func (r *userScopeTypeRepository) Create(scopeType *UserScopeType) error {
	return r.db.Create(scopeType).Error
}

func (r *userScopeTypeRepository) GetByID(id uint) (*UserScopeType, error) {
	var scopeType UserScopeType
	err := r.db.First(&scopeType, id).Error
	if err != nil {
		return nil, err
	}
	return &scopeType, nil
}

func (r *userScopeTypeRepository) GetByScope(scope string) (*UserScopeType, error) {
	var scopeType UserScopeType
	err := r.db.Where("scope = ?", scope).First(&scopeType).Error
	if err != nil {
		return nil, err
	}
	return &scopeType, nil
}

func (r *userScopeTypeRepository) Update(scopeType *UserScopeType) error {
	return r.db.Save(scopeType).Error
}

func (r *userScopeTypeRepository) Delete(id uint) error {
	return r.db.Delete(&UserScopeType{}, id).Error
}

func (r *userScopeTypeRepository) List() ([]UserScopeType, error) {
	var scopeTypes []UserScopeType
	err := r.db.Find(&scopeTypes).Error
	return scopeTypes, err
}

func (r *userScopeTypeRepository) GetWithUsers(id uint) (*UserScopeType, error) {
	var scopeType UserScopeType
	err := r.db.Preload("Users").First(&scopeType, id).Error
	if err != nil {
		return nil, err
	}
	return &scopeType, nil
}

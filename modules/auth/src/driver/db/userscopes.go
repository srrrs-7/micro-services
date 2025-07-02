package db

import (
	"time"

	"gorm.io/gorm"
)

type UserScope struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	ScopeID   uint      `gorm:"primaryKey" json:"scope_id"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`

	User  User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Scope UserScopeType `gorm:"foreignKey:ScopeID" json:"scope,omitempty"`
}

func (UserScope) TableName() string {
	return "user_scopes"
}

type UserScopeRepository interface {
	Create(userScope *UserScope) error
	GetByUserID(userID uint) ([]UserScope, error)
	GetByScopeID(scopeID uint) ([]UserScope, error)
	GetByUserAndScope(userID, scopeID uint) (*UserScope, error)
	Delete(userID, scopeID uint) error
	DeleteByUserID(userID uint) error
	DeleteByScopeID(scopeID uint) error
	List() ([]UserScope, error)
	GetWithUserAndScope(userID, scopeID uint) (*UserScope, error)
}

type userScopeRepository struct {
	db *gorm.DB
}

func NewUserScopeRepository(db *gorm.DB) UserScopeRepository {
	return &userScopeRepository{db: db}
}

func (r *userScopeRepository) Create(userScope *UserScope) error {
	return r.db.Create(userScope).Error
}

func (r *userScopeRepository) GetByUserID(userID uint) ([]UserScope, error) {
	var userScopes []UserScope
	err := r.db.Where("user_id = ?", userID).Find(&userScopes).Error
	return userScopes, err
}

func (r *userScopeRepository) GetByScopeID(scopeID uint) ([]UserScope, error) {
	var userScopes []UserScope
	err := r.db.Where("scope_id = ?", scopeID).Find(&userScopes).Error
	return userScopes, err
}

func (r *userScopeRepository) GetByUserAndScope(userID, scopeID uint) (*UserScope, error) {
	var userScope UserScope
	err := r.db.Where("user_id = ? AND scope_id = ?", userID, scopeID).First(&userScope).Error
	if err != nil {
		return nil, err
	}
	return &userScope, nil
}

func (r *userScopeRepository) Delete(userID, scopeID uint) error {
	return r.db.Where("user_id = ? AND scope_id = ?", userID, scopeID).Delete(&UserScope{}).Error
}

func (r *userScopeRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&UserScope{}).Error
}

func (r *userScopeRepository) DeleteByScopeID(scopeID uint) error {
	return r.db.Where("scope_id = ?", scopeID).Delete(&UserScope{}).Error
}

func (r *userScopeRepository) List() ([]UserScope, error) {
	var userScopes []UserScope
	err := r.db.Find(&userScopes).Error
	return userScopes, err
}

func (r *userScopeRepository) GetWithUserAndScope(userID, scopeID uint) (*UserScope, error) {
	var userScope UserScope
	err := r.db.Preload("User").Preload("Scope").Where("user_id = ? AND scope_id = ?", userID, scopeID).First(&userScope).Error
	if err != nil {
		return nil, err
	}
	return &userScope, nil
}

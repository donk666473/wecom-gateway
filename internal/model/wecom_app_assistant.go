// Package model 提供应用-智能体绑定表 (im_app_assistant) 的数据模型和 CRUD 操作。
// 参照 DATRIX 设计文档 5.2 节。
package model

import (
	"errors"
	"time"
)

// AppAssistant 应用-智能体绑定表，对应设计文档中的 im_app_assistant 表。
// 一个 IM 应用可绑定多个智能体，其中一个为默认智能体。
type AppAssistant struct {
	ID          int64     `gorm:"primary_key;auto_increment" json:"id"`
	AppID       string    `gorm:"type:varchar(64);uniqueIndex:uk_app_assistant;not null;comment:IM应用ID" json:"app_id"`
	AssistantID string    `gorm:"type:varchar(100);uniqueIndex:uk_app_assistant;not null;comment:智能体ID" json:"assistant_id"`
	BindingName string    `gorm:"type:varchar(200);comment:绑定名称" json:"binding_name"`
	IsDefault   int       `gorm:"type:smallint;default:0;comment:是否默认智能体 0=否 1=是" json:"is_default"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (AppAssistant) TableName() string {
	return "app_assistant"
}

// GetDefaultAssistant 获取应用的默认智能体
func GetDefaultAssistant(appID string) (*AppAssistant, error) {
	var binding AppAssistant
	err := DB.Where("app_id = ? AND is_default = 1", appID).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetAppAssistants 获取应用绑定的所有智能体
func GetAppAssistants(appID string) ([]AppAssistant, error) {
	var bindings []AppAssistant
	err := DB.Where("app_id = ?", appID).Find(&bindings).Error
	return bindings, err
}

// Create 创建绑定关系
func (b *AppAssistant) Create() error {
	return DB.Create(b).Error
}

// Delete 删除绑定关系
func (b *AppAssistant) Delete() error {
	result := DB.Where("app_id = ? AND assistant_id = ?", b.AppID, b.AssistantID).Delete(&AppAssistant{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("record not found")
	}
	return nil
}

// SetDefault 将指定绑定设为默认智能体（事务处理）
func SetDefault(appID, assistantID string) error {
	tx := DB.Begin()
	// 取消所有默认
	if err := tx.Model(&AppAssistant{}).Where("app_id = ?", appID).Update("is_default", 0).Error; err != nil {
		tx.Rollback()
		return err
	}
	// 设置新的默认
	if err := tx.Model(&AppAssistant{}).Where("app_id = ? AND assistant_id = ?", appID, assistantID).
		Update("is_default", 1).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

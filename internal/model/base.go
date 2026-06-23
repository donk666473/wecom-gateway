// Package model 提供数据模型定义和数据库操作。
// 参照 DATRIX 设计文档第五节数据库设计和钉钉对接项目中的 model 模块。
package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型，包含通用字段
type BaseModel struct {
	ID        int64          `gorm:"primary_key;auto_increment" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// DB 全局数据库实例（由 main 注入）
var DB *gorm.DB

// SetDB 设置全局数据库实例
func SetDB(db *gorm.DB) {
	DB = db
}

// InitAllTables 自动迁移所有数据表
func InitAllTables() error {
	if DB == nil {
		return nil
	}
	return DB.AutoMigrate(
		&WeComApp{},
		&AppAssistant{},
		&WeComUserInfo{},
		&WeComSession{},
	)
}

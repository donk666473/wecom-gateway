// Package model 提供企微应用表 (im_app / wecom_app) 的数据模型和 CRUD 操作。
// 参照 DATRIX 设计文档 5.1 节 IM 应用表定义。
package model

import (
	"errors"
	"time"
)

// WeComApp 企微 IM 应用表，对应设计文档中的 im_app 表。
// 存储企微自建应用的凭证信息和平台特有配置。
type WeComApp struct {
	ID           int64     `gorm:"primary_key;auto_increment" json:"id"`
	AppID        string    `gorm:"type:varchar(64);uniqueIndex:uk_app_id;not null;comment:应用唯一标识(UUID)" json:"app_id"`
	Platform     string    `gorm:"type:varchar(20);not null;comment:平台类型(wecom/dingtalk/feishu)" json:"platform"`
	ClientID     string    `gorm:"type:varchar(500);not null;comment:应用corpid/AppKey" json:"client_id"`
	ClientSecret string    `gorm:"type:varchar(500);comment:应用secret/AppSecret" json:"client_secret"`
	AppName      string    `gorm:"type:varchar(200);comment:应用名称" json:"app_name"`
	ExtraConfig  string    `gorm:"type:text;comment:平台特有配置JSON" json:"extra_config"`
	Status       int       `gorm:"type:tinyint;default:1;comment:状态 0=禁用 1=启用" json:"status"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (WeComApp) TableName() string {
	return "wecom_app"
}

// ============================================================================
// CRUD 操作
// ============================================================================

// GetActiveApps 获取所有启用状态的应用
func GetActiveApps() ([]WeComApp, error) {
	var apps []WeComApp
	err := DB.Where("status = ?", 1).Find(&apps).Error
	return apps, err
}

// GetAppsByPlatform 按平台类型获取所有启用应用
func GetAppsByPlatform(platform string) ([]WeComApp, error) {
	var apps []WeComApp
	err := DB.Where("platform = ? AND status = ?", platform, 1).Find(&apps).Error
	return apps, err
}

// GetAppByClientID 通过平台类型和 client_id 获取应用
func GetAppByClientID(platform, clientID string) (*WeComApp, error) {
	var app WeComApp
	err := DB.Where("platform = ? AND client_id = ?", platform, clientID).First(&app).Error
	return &app, err
}

// GetAppByID 通过 app_id 获取应用
func GetAppByID(appID string) (*WeComApp, error) {
	var app WeComApp
	err := DB.Where("app_id = ?", appID).First(&app).Error
	return &app, err
}

// Create 创建新应用
func (a *WeComApp) Create() error {
	return DB.Create(a).Error
}

// Update 更新应用配置
func (a *WeComApp) Update() error {
	return DB.Model(&WeComApp{}).Where("app_id = ?", a.AppID).Updates(map[string]interface{}{
		"client_secret": a.ClientSecret,
		"app_name":      a.AppName,
		"extra_config":  a.ExtraConfig,
		"status":        a.Status,
		"updated_at":    time.Now(),
	}).Error
}

// Delete 删除应用（软删除）
func (a *WeComApp) Delete() error {
	result := DB.Where("app_id = ?", a.AppID).Delete(&WeComApp{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("record not found")
	}
	return nil
}

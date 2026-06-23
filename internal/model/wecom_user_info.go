// Package model 提供企微用户信息表 (im_user_info) 的数据模型和 CRUD 操作。
// 参照 DATRIX 设计文档 5.3 节。
package model

import "time"

// WeComUserInfo IM 用户信息表，对应设计文档中的 im_user_info 表。
// 存储 IM 用户与 DATRIX 平台用户的绑定关系。
type WeComUserInfo struct {
	ID             int64     `gorm:"primary_key;auto_increment" json:"id"`
	Platform       string    `gorm:"type:varchar(20);uniqueIndex:uk_platform_staff;not null;comment:平台类型" json:"platform"`
	StaffID        string    `gorm:"type:varchar(200);uniqueIndex:uk_platform_staff;not null;comment:IM用户ID" json:"staff_id"`
	UnionID        string    `gorm:"type:varchar(200);index:idx_unionid;comment:IM平台UnionID" json:"union_id"`
	Mobile         string    `gorm:"type:varchar(50);comment:手机号(脱敏存储)" json:"mobile"`
	DatrixUserID   string    `gorm:"type:varchar(100);comment:DATRIX用户ID" json:"datrix_user_id"`
	DatrixUserName string    `gorm:"type:varchar(200);comment:DATRIX用户名" json:"datrix_user_name"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (WeComUserInfo) TableName() string {
	return "wecom_user_info"
}

// GetIMUserInfo 获取 IM 用户信息
func GetIMUserInfo(platform, staffID string) (*WeComUserInfo, error) {
	var info WeComUserInfo
	err := DB.Where("platform = ? AND staff_id = ?", platform, staffID).First(&info).Error
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// SaveOrUpdateIMUserInfo 保存或更新 IM 用户信息
func SaveOrUpdateIMUserInfo(info *WeComUserInfo) error {
	var existing WeComUserInfo
	err := DB.Where("platform = ? AND staff_id = ?", info.Platform, info.StaffID).First(&existing).Error
	if err != nil {
		// 不存在则创建
		return DB.Create(info).Error
	}
	// 存在则更新
	return DB.Model(&existing).Updates(map[string]interface{}{
		"union_id":         info.UnionID,
		"mobile":           info.Mobile,
		"datrix_user_id":   info.DatrixUserID,
		"datrix_user_name": info.DatrixUserName,
		"updated_at":       time.Now(),
	}).Error
}

// DeleteUserBinding 删除 IM 用户绑定关系
func DeleteUserBinding(platform, staffID string) error {
	return DB.Where("platform = ? AND staff_id = ?", platform, staffID).Delete(&WeComUserInfo{}).Error
}

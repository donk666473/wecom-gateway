// Package model 提供会话表 (im_session) 的数据模型和 CRUD 操作。
// 参照 DATRIX 设计文档 5.4 节。
package model

import "time"

// WeComSession 会话表，对应设计文档中的 im_session 表。
// 管理 IM 用户与 DATRIX 智能体的对话会话。
type WeComSession struct {
	ID             int64     `gorm:"primary_key;auto_increment" json:"id"`
	AppID          string    `gorm:"type:varchar(64);uniqueIndex:uk_app_staff_conv_group;not null;comment:IM应用ID" json:"app_id"`
	StaffID        string    `gorm:"type:varchar(200);uniqueIndex:uk_app_staff_conv_group;not null;comment:IM用户ID" json:"staff_id"`
	ConversationID string    `gorm:"type:varchar(200);uniqueIndex:uk_app_staff_conv_group;not null;comment:IM会话ID" json:"conversation_id"`
	IsGroup        bool      `gorm:"uniqueIndex:uk_app_staff_conv_group;not null;comment:是否群聊" json:"is_group"`
	DatrixUserID   string    `gorm:"type:varchar(100);comment:DATRIX用户ID" json:"datrix_user_id"`
	Token          string    `gorm:"type:varchar(500);comment:DATRIX Token(加密存储)" json:"token"`
	SessionID      string    `gorm:"type:varchar(200);comment:DATRIX会话ID" json:"session_id"`
	AssistantID    string    `gorm:"type:varchar(100);comment:当前绑定的智能体ID" json:"assistant_id"`
	LastChatAt     time.Time `gorm:"comment:上次对话时间" json:"last_chat_at"`
}

// TableName 指定表名
func (WeComSession) TableName() string {
	return "wecom_session"
}

// GetSessionInfo 获取会话信息
func GetSessionInfo(appID, staffID, conversationID string, isGroup bool) (*WeComSession, error) {
	var session WeComSession
	err := DB.Where("app_id = ? AND staff_id = ? AND conversation_id = ? AND is_group = ?",
		appID, staffID, conversationID, isGroup).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// SaveOrUpdateSession 保存或更新会话
func SaveOrUpdateSession(session *WeComSession) error {
	var existing WeComSession
	query := DB.Where("app_id = ? AND staff_id = ? AND conversation_id = ? AND is_group = ?",
		session.AppID, session.StaffID, session.ConversationID, session.IsGroup)
	err := query.First(&existing).Error
	if err != nil {
		// 不存在则创建
		return DB.Create(session).Error
	}
	// 存在则更新
	return DB.Model(&existing).Updates(map[string]interface{}{
		"datrix_user_id": session.DatrixUserID,
		"token":          session.Token,
		"session_id":     session.SessionID,
		"assistant_id":   session.AssistantID,
		"last_chat_at":   time.Now(),
	}).Error
}

// DeleteSessionsByAppID 删除应用的所有会话（应用删除时调用）
func DeleteSessionsByAppID(appID string) error {
	return DB.Where("app_id = ?", appID).Delete(&WeComSession{}).Error
}

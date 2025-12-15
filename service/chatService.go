package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"synergazing.com/synergazing/config"
	"synergazing.com/synergazing/model"
)

type ChatService struct {
	DB *gorm.DB
}

func NewChatService() *ChatService {
	return &ChatService{
		DB: config.GetDB(),
	}
}

// GetOrCreateChat creates a chat between two users or returns existing one
func (s *ChatService) GetOrCreateChat(user1ID, user2ID uint) (*model.Chat, error) {
	if user1ID == user2ID {
		return nil, errors.New("cannot create chat with yourself")
	}

	// Ensure consistent ordering (smaller ID first)
	if user1ID > user2ID {
		user1ID, user2ID = user2ID, user1ID
	}

	var chat model.Chat

	// Try to find existing chat
	err := s.DB.Preload("User1").Preload("User2").
		Where("(user1_id = ? AND user2_id = ?) OR (user1_id = ? AND user2_id = ?)",
			user1ID, user2ID, user2ID, user1ID).
		First(&chat).Error

	if err == nil {
		return &chat, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("error finding chat: %v", err)
	}

	// Create new chat
	newChat := model.Chat{
		User1ID: user1ID,
		User2ID: user2ID,
	}

	if err := s.DB.Create(&newChat).Error; err != nil {
		return nil, fmt.Errorf("error creating chat: %v", err)
	}

	// Preload users for the new chat
	if err := s.DB.Preload("User1").Preload("User2").First(&newChat, newChat.ID).Error; err != nil {
		return nil, fmt.Errorf("error loading chat users: %v", err)
	}

	return &newChat, nil
}

// GetChatMessages retrieves messages for a specific chat with pagination
func (s *ChatService) GetChatMessages(chatID uint, userID uint, offset, limit int) ([]model.Message, error) {
	// First verify user has access to this chat
	if !s.UserHasAccessToChat(chatID, userID) {
		return nil, errors.New("unauthorized access to chat")
	}

	var messages []model.Message
	err := s.DB.Preload("Sender").Preload("Sender.Profile").
		Where("chat_id = ?", chatID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("error retrieving messages: %v", err)
	}

	return messages, nil
}

// SendMessage creates a new message in a chat
func (s *ChatService) SendMessage(chatID uint, senderID uint, content string) (*model.Message, error) {
	// Verify user has access to this chat
	if !s.UserHasAccessToChat(chatID, senderID) {
		return nil, errors.New("unauthorized access to chat")
	}

	if content == "" {
		return nil, errors.New("message content cannot be empty")
	}

	message := model.Message{
		ChatID:   chatID,
		SenderID: senderID,
		Content:  content,
		IsRead:   false,
	}

	if err := s.DB.Create(&message).Error; err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	// Preload sender information
	if err := s.DB.Preload("Sender").Preload("Sender.Profile").First(&message, message.ID).Error; err != nil {
		return nil, fmt.Errorf("error loading message sender: %v", err)
	}

	return &message, nil
}

// MarkMessagesAsRead marks messages as read for a specific user
func (s *ChatService) MarkMessagesAsRead(chatID uint, userID uint) error {
	// Verify user has access to this chat
	if !s.UserHasAccessToChat(chatID, userID) {
		return errors.New("unauthorized access to chat")
	}

	// Mark messages as read (messages not sent by the current user)
	err := s.DB.Model(&model.Message{}).
		Where("chat_id = ? AND sender_id != ? AND is_read = false", chatID, userID).
		Update("is_read", true).Error

	if err != nil {
		return fmt.Errorf("error marking messages as read: %v", err)
	}

	return nil
}

// GetUserChats retrieves all chats for a user
func (s *ChatService) GetUserChats(userID uint) ([]model.Chat, error) {
	var chats []model.Chat

	err := s.DB.Preload("User1").Preload("User2").Preload("User1.Profile").Preload("User2.Profile").
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(1) // Get last message
		}).
		Where("user1_id = ? OR user2_id = ?", userID, userID).
		Order("updated_at DESC").
		Find(&chats).Error

	if err != nil {
		return nil, fmt.Errorf("error retrieving user chats: %v", err)
	}

	return chats, nil
}

// UserHasAccessToChat checks if a user has access to a specific chat
func (s *ChatService) UserHasAccessToChat(chatID uint, userID uint) bool {
	var count int64
	s.DB.Model(&model.Chat{}).
		Where("id = ? AND (user1_id = ? OR user2_id = ?)", chatID, userID, userID).
		Count(&count)

	return count > 0
}

// GetChatByID retrieves a chat by ID if user has access
func (s *ChatService) GetChatByID(chatID uint, userID uint) (*model.Chat, error) {
	if !s.UserHasAccessToChat(chatID, userID) {
		return nil, errors.New("unauthorized access to chat")
	}

	var chat model.Chat
	err := s.DB.Preload("User1").Preload("User2").Preload("User1.Profile").Preload("User2.Profile").First(&chat, chatID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("chat not found")
		}
		return nil, fmt.Errorf("error retrieving chat: %v", err)
	}

	return &chat, nil
}

// GetChatParticipants retrieves the participants of a chat without authorization check (internal use only)
func (s *ChatService) GetChatParticipants(chatID uint) (uint, uint, error) {
	var chat model.Chat
	// Select only the user IDs to be efficient
	err := s.DB.Select("user1_id", "user2_id").First(&chat, chatID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, 0, errors.New("chat not found")
		}
		return 0, 0, fmt.Errorf("error retrieving chat participants: %v", err)
	}

	return chat.User1ID, chat.User2ID, nil
}

// GetUnreadNotifications gets unread message notifications for a user
func (s *ChatService) GetUnreadNotifications(userID uint) ([]map[string]interface{}, error) {
	var notifications []map[string]interface{}

	// Get all chats where user is a participant and has unread messages
	rows, err := s.DB.Raw(`
		SELECT 
			c.id as chat_id,
			CASE 
				WHEN c.user1_id = ? THEN c.user2_id 
				ELSE c.user1_id 
			END as other_user_id,
			CASE 
				WHEN c.user1_id = ? THEN u2.name 
				ELSE u1.name 
			END as other_user_name,
			COUNT(m.id) as unread_count,
			MAX(m.created_at) as last_message_time,
			(SELECT content FROM messages WHERE chat_id = c.id ORDER BY created_at DESC LIMIT 1) as last_message_content
		FROM chats c
		LEFT JOIN users u1 ON c.user1_id = u1.id
		LEFT JOIN users u2 ON c.user2_id = u2.id
		LEFT JOIN messages m ON c.id = m.chat_id AND m.sender_id != ? AND m.is_read = false
		WHERE (c.user1_id = ? OR c.user2_id = ?)
		AND EXISTS (SELECT 1 FROM messages WHERE chat_id = c.id AND sender_id != ? AND is_read = false)
		GROUP BY c.id, other_user_id, other_user_name
		ORDER BY last_message_time DESC
	`, userID, userID, userID, userID, userID, userID).Rows()

	if err != nil {
		return nil, fmt.Errorf("error getting unread notifications: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var chatID, otherUserID uint
		var otherUserName, lastMessageContent string
		var unreadCount int
		var lastMessageTime interface{}

		err := rows.Scan(&chatID, &otherUserID, &otherUserName, &unreadCount, &lastMessageTime, &lastMessageContent)
		if err != nil {
			return nil, fmt.Errorf("error scanning notification row: %v", err)
		}

		notification := map[string]interface{}{
			"chat_id":              chatID,
			"other_user_id":        otherUserID,
			"other_user_name":      otherUserName,
			"unread_count":         unreadCount,
			"last_message_time":    lastMessageTime,
			"last_message_content": lastMessageContent,
		}

		notifications = append(notifications, notification)
	}

	return notifications, nil
}

// GetTotalUnreadCount gets the total number of unread messages for a user
func (s *ChatService) GetTotalUnreadCount(userID uint) (int, error) {
	var count int64

	err := s.DB.Model(&model.Message{}).
		Joins("JOIN chats ON messages.chat_id = chats.id").
		Where("(chats.user1_id = ? OR chats.user2_id = ?) AND messages.sender_id != ? AND messages.is_read = false",
			userID, userID, userID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("error getting total unread count: %v", err)
	}

	return int(count), nil
}

// GetUnreadUsersCount gets the count of users who have sent unread messages to the current user
func (s *ChatService) GetUnreadUsersCount(userID uint) (int, error) {
	var count int64

	// Count distinct sender IDs from unread messages in chats where the user is a participant
	err := s.DB.Model(&model.Message{}).
		Joins("JOIN chats ON messages.chat_id = chats.id").
		Where("(chats.user1_id = ? OR chats.user2_id = ?) AND messages.sender_id != ? AND messages.is_read = false",
			userID, userID, userID).
		Distinct("messages.sender_id").
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("error getting unread users count: %v", err)
	}

	return int(count), nil
}

// GetUnreadMessagesCount gets the total number of unread messages for a user (same as GetTotalUnreadCount but with clearer naming)
func (s *ChatService) GetUnreadMessagesCount(userID uint) (int, error) {
	var count int64

	err := s.DB.Model(&model.Message{}).
		Joins("JOIN chats ON messages.chat_id = chats.id").
		Where("(chats.user1_id = ? OR chats.user2_id = ?) AND messages.sender_id != ? AND messages.is_read = false",
			userID, userID, userID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("error getting unread messages count: %v", err)
	}

	return int(count), nil
}

// GetUnreadMessagesCountByUser gets unread message counts grouped by sender (like WhatsApp/Telegram)
func (s *ChatService) GetUnreadMessagesCountByUser(userID uint) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	// Get unread message counts grouped by sender with user details
	rows, err := s.DB.Raw(`
		SELECT 
			m.sender_id,
			u.name as sender_name,
			COUNT(m.id) as unread_count
		FROM messages m
		JOIN chats c ON m.chat_id = c.id
		JOIN users u ON m.sender_id = u.id
		WHERE (c.user1_id = ? OR c.user2_id = ?) 
		  AND m.sender_id != ? 
		  AND m.is_read = false
		GROUP BY m.sender_id, u.name
		ORDER BY unread_count DESC
	`, userID, userID, userID).Rows()

	if err != nil {
		return nil, fmt.Errorf("error getting unread messages count by user: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var senderID uint
		var senderName string
		var unreadCount int

		err := rows.Scan(&senderID, &senderName, &unreadCount)
		if err != nil {
			return nil, fmt.Errorf("error scanning unread count row: %v", err)
		}

		result := map[string]interface{}{
			"user_id":      senderID,
			"user_name":    senderName,
			"unread_count": unreadCount,
		}

		results = append(results, result)
	}

	return results, nil
}

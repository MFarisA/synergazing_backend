package controller

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"synergazing.com/synergazing/helper"
	"synergazing.com/synergazing/service"
)

type ChatController struct {
	ChatService *service.ChatService
	connections map[uint]*websocket.Conn // userID -> connection
	mutex       sync.RWMutex
}

type WebSocketMessage struct {
	Type    string      `json:"type"`
	ChatID  uint        `json:"chat_id,omitempty"`
	Content string      `json:"content,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type MessageResponse struct {
	ID        uint   `json:"id"`
	ChatID    uint   `json:"chat_id"`
	SenderID  uint   `json:"sender_id"`
	Content   string `json:"content"`
	IsRead    bool   `json:"is_read"`
	CreatedAt string `json:"created_at"`
	Sender    struct {
		ID     uint   `json:"id"`
		Name   string `json:"name"`
		Avatar string `json:"avatar,omitempty"`
	} `json:"sender"`
}

func NewChatController(chatService *service.ChatService) *ChatController {
	return &ChatController{
		ChatService: chatService,
		connections: make(map[uint]*websocket.Conn),
	}
}

// WebSocket upgrade handler
func (ctrl *ChatController) WebSocketUpgrade(c *fiber.Ctx) error {
	// Check if the request is a WebSocket upgrade
	if websocket.IsWebSocketUpgrade(c) {
		c.Locals("allowed", true)
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

// WebSocket connection handler
func (ctrl *ChatController) HandleWebSocket(c *websocket.Conn) {
	// Set connection timeouts
	c.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	c.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Get user ID and token from query params
	userIDStr := c.Query("user_id")
	token := c.Query("token")

	if userIDStr == "" {
		log.Println("No user_id provided in WebSocket connection")
		c.Close()
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		log.Printf("Invalid user_id: %s", userIDStr)
		c.Close()
		return
	}

	currentUserID := uint(userID)

	// If token is provided, verify it
	if token != "" {
		claims, err := helper.VerifyJWTToken(token)
		if err != nil {
			log.Printf("Invalid JWT token for user %d: %v", currentUserID, err)
			c.Close()
			return
		}

		// Verify the token belongs to the user
		if claims.UserID != currentUserID {
			log.Printf("Token user ID mismatch: token=%d, provided=%d", claims.UserID, currentUserID)
			c.Close()
			return
		}

		log.Printf("User %d authenticated via JWT token", currentUserID)
	} else {
		log.Printf("User %d connected without token (test mode)", currentUserID)
	}

	// Store connection
	ctrl.mutex.Lock()
	ctrl.connections[currentUserID] = c
	ctrl.mutex.Unlock()

	// Remove connection on close
	defer func() {
		ctrl.mutex.Lock()
		delete(ctrl.connections, currentUserID)
		ctrl.mutex.Unlock()
		c.Close()
	}()

	log.Printf("User %d connected to WebSocket", currentUserID)

	// Send welcome message
	welcomeMsg := WebSocketMessage{
		Type: "connected",
		Data: fiber.Map{"message": "Connected to chat server"},
	}
	ctrl.sendToUser(currentUserID, welcomeMsg)

	// Handle incoming messages
	for {
		var msg WebSocketMessage
		if err := c.ReadJSON(&msg); err != nil {
			log.Printf("Error reading WebSocket message: %v", err)
			break
		}

		// Reset read deadline on any message
		c.SetReadDeadline(time.Now().Add(60 * time.Second))

		switch msg.Type {
		case "ping":
			// Respond with pong
			pongMsg := WebSocketMessage{
				Type: "pong",
				Data: fiber.Map{"timestamp": time.Now().Unix()},
			}
			ctrl.sendToUser(currentUserID, pongMsg)
		case "send_message":
			ctrl.handleSendMessage(currentUserID, msg)
		case "join_chat":
			ctrl.handleJoinChat(currentUserID, msg)
		case "mark_read":
			ctrl.handleMarkRead(currentUserID, msg)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

func (ctrl *ChatController) handleSendMessage(userID uint, msg WebSocketMessage) {
	if msg.ChatID == 0 || msg.Content == "" {
		ctrl.sendError(userID, "Invalid message data")
		return
	}

	// Send message via service
	message, err := ctrl.ChatService.SendMessage(msg.ChatID, userID, msg.Content)
	if err != nil {
		ctrl.sendError(userID, err.Error())
		return
	}

	// Create response
	response := MessageResponse{
		ID:        message.ID,
		ChatID:    message.ChatID,
		SenderID:  message.SenderID,
		Content:   message.Content,
		IsRead:    message.IsRead,
		CreatedAt: message.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Sender: struct {
			ID     uint   `json:"id"`
			Name   string `json:"name"`
			Avatar string `json:"avatar,omitempty"`
		}{
			ID:   message.Sender.ID,
			Name: message.Sender.Name,
			Avatar: func() string {
				if message.Sender.Profile != nil {
					return helper.GetUrlFile(message.Sender.Profile.ProfilePicture)
				}
				return ""
			}(),
		},
	}

	// Send to both users in the chat
	ctrl.broadcastToChat(msg.ChatID, WebSocketMessage{
		Type: "new_message",
		Data: response,
	})
}

func (ctrl *ChatController) handleJoinChat(userID uint, msg WebSocketMessage) {
	if msg.ChatID == 0 {
		ctrl.sendError(userID, "Invalid chat ID")
		return
	}

	// Verify user has access to chat
	chat, err := ctrl.ChatService.GetChatByID(msg.ChatID, userID)
	if err != nil {
		ctrl.sendError(userID, err.Error())
		return
	}

	// Send confirmation
	ctrl.sendToUser(userID, WebSocketMessage{
		Type: "joined_chat",
		Data: fiber.Map{
			"chat_id": chat.ID,
			"message": "Joined chat successfully",
		},
	})
}

func (ctrl *ChatController) handleMarkRead(userID uint, msg WebSocketMessage) {
	if msg.ChatID == 0 {
		ctrl.sendError(userID, "Invalid chat ID")
		return
	}

	err := ctrl.ChatService.MarkMessagesAsRead(msg.ChatID, userID)
	if err != nil {
		ctrl.sendError(userID, err.Error())
		return
	}

	// Notify that messages were marked as read
	ctrl.sendToUser(userID, WebSocketMessage{
		Type: "messages_marked_read",
		Data: fiber.Map{"chat_id": msg.ChatID},
	})
}

func (ctrl *ChatController) sendToUser(userID uint, msg WebSocketMessage) {
	ctrl.mutex.RLock()
	conn, exists := ctrl.connections[userID]
	ctrl.mutex.RUnlock()

	if exists && conn != nil {
		// Set write deadline
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Error sending message to user %d: %v", userID, err)
			// Remove failed connection
			ctrl.mutex.Lock()
			delete(ctrl.connections, userID)
			ctrl.mutex.Unlock()

			// Close the connection gracefully
			conn.Close()
		}
	}
}

func (ctrl *ChatController) sendError(userID uint, errorMsg string) {
	ctrl.sendToUser(userID, WebSocketMessage{
		Type: "error",
		Data: fiber.Map{"error": errorMsg},
	})
}

func (ctrl *ChatController) broadcastToChat(chatID uint, msg WebSocketMessage) {
	// 1. Get the chat participants from the database ONCE
	user1ID, user2ID, err := ctrl.ChatService.GetChatParticipants(chatID)
	if err != nil {
		log.Printf("Error getting chat participants for broadcast: %v", err)
		return
	}

	// 2. Identify target users
	targets := []uint{user1ID, user2ID}

	// 3. Send to targets if they are connected
	for _, targetID := range targets {
		ctrl.mutex.RLock()
		conn, exists := ctrl.connections[targetID]
		ctrl.mutex.RUnlock()

		if exists && conn != nil {
			// No need to check access again, we just fetched them from the chat record itself
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("Error broadcasting to user %d: %v", targetID, err)
			}
		}
	}
}

// REST API endpoints

// GetOrCreateChat handles creating or getting existing chat between users
func (ctrl *ChatController) GetOrCreateChat(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	otherUserIDStr := c.Params("user_id")
	otherUserID, err := strconv.ParseUint(otherUserIDStr, 10, 32)
	if err != nil {
		return helper.Message400("Invalid user ID")
	}

	chat, err := ctrl.ChatService.GetOrCreateChat(userID, uint(otherUserID))
	if err != nil {
		return helper.Message400(err.Error())
	}

	return helper.Message200(c, chat, "Chat retrieved successfully")
}

// GetUserChats retrieves all chats for the authenticated user
func (ctrl *ChatController) GetUserChats(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	chats, err := ctrl.ChatService.GetUserChats(userID)
	if err != nil {
		return helper.Message500(err.Error())
	}

	return helper.Message200(c, chats, "Chats retrieved successfully")
}

// GetChatMessages retrieves messages for a specific chat
func (ctrl *ChatController) GetChatMessages(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseUint(chatIDStr, 10, 32)
	if err != nil {
		return helper.Message400("Invalid chat ID")
	}

	// Parse pagination parameters
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100 // Max limit
	}
	offset := (page - 1) * limit

	messages, err := ctrl.ChatService.GetChatMessages(uint(chatID), userID, offset, limit)
	if err != nil {
		return helper.Message400(err.Error())
	}

	return helper.Message200(c, fiber.Map{
		"messages": messages,
		"page":     page,
		"limit":    limit,
	}, "Messages retrieved successfully")
}

// MarkChatAsRead marks all unread messages in a chat as read
func (ctrl *ChatController) MarkChatAsRead(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseUint(chatIDStr, 10, 32)
	if err != nil {
		return helper.Message400("Invalid chat ID")
	}

	err = ctrl.ChatService.MarkMessagesAsRead(uint(chatID), userID)
	if err != nil {
		return helper.Message400(err.Error())
	}

	return helper.Message200(c, nil, "Messages marked as read")
}

// GetNotifications gets unread message notifications for the authenticated user
func (ctrl *ChatController) GetNotifications(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	notifications, err := ctrl.ChatService.GetUnreadNotifications(userID)
	if err != nil {
		return helper.Message400(err.Error())
	}

	totalUnread, err := ctrl.ChatService.GetTotalUnreadCount(userID)
	if err != nil {
		return helper.Message400(err.Error())
	}

	return helper.Message200(c, fiber.Map{
		"notifications":      notifications,
		"total_unread":       totalUnread,
		"notification_count": len(notifications),
	}, "Notifications retrieved successfully")
}

// GetUnreadCount gets the total unread message count for the authenticated user
func (ctrl *ChatController) GetUnreadCount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	count, err := ctrl.ChatService.GetTotalUnreadCount(userID)
	if err != nil {
		return helper.Message400(err.Error())
	}

	return helper.Message200(c, fiber.Map{
		"unread_count": count,
	}, "Unread count retrieved successfully")
}

// GetUnreadUsersCount gets the count of users who have sent unread messages to the authenticated user
func (ctrl *ChatController) GetUnreadUsersCount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	count, err := ctrl.ChatService.GetUnreadUsersCount(userID)
	if err != nil {
		return helper.Message400(err.Error())
	}

	return helper.Message200(c, fiber.Map{
		"unread_users_count": count,
	}, "Unread users count retrieved successfully")
}

// GetUnreadMessagesCount gets the total unread message count for the authenticated user
func (ctrl *ChatController) GetUnreadMessagesCount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	unreadByUser, err := ctrl.ChatService.GetUnreadMessagesCountByUser(userID)
	if err != nil {
		return helper.Message400(err.Error())
	}

	// Calculate total count
	totalCount := 0
	for _, item := range unreadByUser {
		totalCount += item["unread_count"].(int)
	}

	return helper.Message200(c, fiber.Map{
		"unread_messages": unreadByUser,
		"total_count":     totalCount,
	}, "Unread messages count retrieved successfully")
}

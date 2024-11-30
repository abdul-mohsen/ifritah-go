package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type NotificationResponse struct {
	Id       int    `json:"id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Username string `json:"username"`
}

func (h *handler) GetNotificationAll(c *gin.Context) {

	userSession := GetSessionInfo(c)

	notification := NotificationResponse{
		Id:       1,
		Title:    "Test",
		Body:     "Notification is not active",
		Username: userSession.username,
	}

	c.IndentedJSON(http.StatusOK, []NotificationResponse{notification})
}

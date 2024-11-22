package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type NotificationResponse struct {
  Id int `json:"id"`
  Title string `json:"title"`
  Body string `json:"body"`
  Username string `json:"username"`
}

func (h * handler) GetNotificationAll(c * gin.Context) {

  token, err := VerifyToken(c)
  if err != nil {
    log.Panic(err)
  }
  userSession := GetSessionInfo(*token)

  notification := NotificationResponse{
    Id: 1,
    Title: "Test",
    Body: "Notification is not active",
    Username: userSession.username,
  }

  c.IndentedJSON(http.StatusOK, []NotificationResponse{notification})
}

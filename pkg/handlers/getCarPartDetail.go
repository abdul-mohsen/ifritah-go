package handlers

import (
	"fmt"
	"log"
	"net/http"

	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
)

func (h * handler) GetCarPartDetail(c *gin.Context) {
  
  rows, err := h.DB.Query("SELECT * FROM user")
  if err != nil {
    log.Fatal(err)
    return 
  }
  var users []model.User
  for rows.Next() {
    var user model.User
    if err := rows.Scan(&user.ID, &user.Username, &user.Password); err != nil {
      log.Fatal(err)
      return 
    }
    fmt.Println(user);
    users = append(users, user)
  }
  defer rows.Close()
  c.IndentedJSON(http.StatusOK, users)

}

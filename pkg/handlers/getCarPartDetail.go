package handlers

import (
	"fmt"
	"net/http"

	"ifritah/web-service-gin/pkg/model"

	"github.com/gin-gonic/gin"
)

func (h * handler) GetCarPartDetail(c *gin.Context) {
  
  rows, err := h.DB.Query("SELECT * FROM user")
  var name = "hi";
  if err != nil {
    fmt.Errorf("hahahah %q: %v", name, err)
  }
  var users []model.User
  for rows.Next() {
    var user model.User
    if err := rows.Scan(&user.ID, &user.Username, &user.Password); err != nil {
      fmt.Println("albumsByArtist %q: %v", name, err)
    }
    fmt.Println(user);
    users = append(users, user)
  }
  defer rows.Close()
  c.IndentedJSON(http.StatusOK, users)

}

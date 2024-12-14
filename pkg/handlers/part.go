package handlers

import (
	"log"

	"github.com/gin-gonic/gin"
)

func (h *handler) GetPartType(c *gin.Context) {

	query := `select distinct genericArticleDescription from articles`
	rows, err := h.DB.Query(query)

	if err != nil {
		log.Panic(err)
	}

	var response []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			log.Panic(err)
		}

		response = append(response, text)

	}
}

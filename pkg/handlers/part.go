package handlers

import (
	"log"
	"net/http"

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

	c.JSON(http.StatusOK, response)
}

type PartByType struct {
	Query    string `json:"query"`
	Page     int    `json:"page_number"`
	PageSize int    `json:"page_size"`
}

func (h *handler) GetPart(c *gin.Context) {

	request := PartByType{
		Page:     0,
		PageSize: 10,
	}

	if err := c.BindJSON(&request); err != nil {
		log.Panic(err)
	}
	query := `
	select distinct articles.legacyArticleId, o.number, articles.genericArticleDescription, al.url as link, p.url 
	from oem_number o
	left join articles on o.articleId = articles.legacyArticleId 
	left join articlelinks al on al.legacyArticleId = articles.legacyArticleId 
	left join articlepdfs p on p.legacyArticleId = articles.legacyArticleId 
	where ? != NULL and o.number like ?
	limit ? offset ?
	`
	rows, err := h.DB.Query(query, request.Query, request.Query+"%", request.PageSize, request.Page)
	if err != nil {
		log.Panic(err)
	}

	var parts []Part
	for rows.Next() {

		var part Part
		err = rows.Scan(&part.Id, &part.OemNumber, &part.Type, &part.Link, &part.Url)
		if err != nil {
			log.Panic(err)
		}

		parts = append(parts, part)
	}

	defer rows.Close()
	c.JSON(http.StatusOK, parts)

}

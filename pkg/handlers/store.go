package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Store struct {
	Id        int     `json:"id"`
	AddressId *int    `json:"address_id"`
	Name      *string `json:"name"`
}

func (h *handler) getStores(user userSession) []Store {

	rows, err := h.DB.Query(`select store.id, addressId, store.namefrom store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id`, user.id)

	if err != nil {
		log.Panic(err)
	}

	var stores []Store

	for rows.Next() {
		var store Store
		if err := rows.Scan(&store.Id, &store.AddressId, &store.Name); err != nil {
			log.Panic(err)
		}
		stores = append(stores, store)
	}

	return stores

}

func (h *handler) getStoreIds(c *gin.Context) []int {

	userSession := GetSessionInfo(c)
	rows, err := h.DB.Query(`select store.id from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id`, userSession.id)

	if err != nil {
		log.Panic(err)
	}

	var ids []int

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			log.Panic(err)
		}
		ids = append(ids, id)
	}

	return ids
}

func (h *handler) GetStores(c *gin.Context) {
	c.JSON(http.StatusOK, h.getStores(GetSessionInfo(c)))
}

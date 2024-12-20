package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Store struct {
	Id        int
	AddressId *int
}

func (h *handler) getStores(user userSession) []Store {

	println("user_id", user.id)
	rows, err := h.DB.Query(`select store.id, addressId from store join company on store.company_id = company.id join user on user.id= ? and company.id=user.company_id`, user.id)

	if err != nil {
		log.Panic(err)
	}

	var stores []Store

	for rows.Next() {
		var store Store
		if err := rows.Scan(&store.Id, &store.AddressId); err != nil {
			log.Panic(err)
		}
		stores = append(stores, store)
	}

	return stores

}

func (h *handler) GetStores(c *gin.Context) {
	c.JSON(http.StatusOK, h.getStores(GetSessionInfo(c)))
}

package handlers

import "log"

type Store struct {
	Id        int
	AddressId int
}

func (h *handler) getStoresForUser(user userSession) []Store {

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
	}
	return stores

}

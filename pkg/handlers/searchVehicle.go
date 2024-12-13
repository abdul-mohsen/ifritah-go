package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func (h *handler) SearchByVin(c *gin.Context) {
	body := h.searchByVin(c)
	fmt.Println(body)
	c.Data(200, "json", body)
}

type PartByVin struct {
	Page     int `json:"page_number"`
	PageSize int `json:"page_size"`
}

type CarModel struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (h *handler) GetCarsByVin(c *gin.Context) {
	manu := "Honda"
	modelName := "Accord"
	madeYear := 1998
	query := `
	select distinct linkageTargetId, linkageTargetType, vehicleModelSeriesName
	from manufacturers m join
	modelseries s on manuName like ? and m.manuId=s.manuId and modelname like ? and (yearOfConstrTo is Null or yearOfConstrTo <= ?) and yearOfConstrFrom >= ? join
	linkagetargets l on vehicleModelSeriesId = s.modelId and lang='en';`
	rows, err := h.DB.Query(query, manu, "%"+modelName+"%", madeYear*100+12, madeYear*100)
	if err != nil {
		log.Panic(err)
	}

	var response []CarModel
	for rows.Next() {
		var model CarModel
		if err := rows.Scan(&model.Id, &model.Name, &model.Type); err != nil {
			log.Panic(err)
		}

		response = append(response, model)
	}
	c.IndentedJSON(http.StatusOK, response)
}

// TODO
// func (h *handler) GetPartByVin(c *gin.Context) {
// 	request := PartByVin {
// 		Page: 0,
// 		PageSize: 100,
// 	}
// 	manu := "Honda"
// 	modelName := "Accord"
// 	madeYear := 1998
// 	query := `
// 	select
// 	from manufacturers m join
// 	modelseries s on manuName like ? and m.manuId=s.manuId like '%?%' and (yearOfConstrTo is Null or yearOfConstrTo <= ?12) and yearOfConstrFrom >= ?00 join
// 	linkagetargets l on vehicleModelSeriesId = s.modelId and lang='en' join
// 	articlesvehicletrees a on a.linkingTargetId=l.linkageTargetId join
// 	articles on articles.legacyArticleId = a.legacyArticleId
// 	limit ? offset ?
// 	`
// 	rows, err := h.DB.Query(query, manu, modelName, madeYear, madeYear, request.PageSize, request.Page)
// 	if ; err != nil {
// 		log.Panic(err)
// 	}
//
//
// 	for rows.Next()
//
//
//
//
// }

func (h *handler) searchByVin(c *gin.Context) []byte {
	baseurl := os.Getenv("VEHICLE_DATABASES")
	europe := "/europe-vin-decode/"
	global := "/vin-decode/"
	var vin string = c.Param("vin")
	body, err := getBody(baseurl + global + vin)
	if err != nil {
		fmt.Println("Error: received non-200 response status:", err)
		body, err := getBody(baseurl + europe + vin)
		if err != nil {
			fmt.Println("Error: received non-200 response status:", err)
		}
		c.Data(200, "json", body)
	}
	return body
}

func getBody(url string) ([]byte, error) {

	key := os.Getenv("VEHICLE_DATABASES_KEY")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil, err
	}

	req.Header.Add("x-AuthKey", key)

	// Create an HTTP client and perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return nil, err
	}
	defer resp.Body.Close() // Ensure the response body is closed

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error: received non-200 response status:", resp.Status)
		return nil, err
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil, err
	}

	return body, nil
}

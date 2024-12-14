package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func (h *handler) SearchByVin(c *gin.Context) {
	body := h.searchByVinRaw(c)
	fmt.Println(body)
	c.Data(200, "json", body)
}

type PartByVin struct {
	Page     int `json:"page_number"`
	PageSize int `json:"page_size"`
}

type BaseModel struct {
	Vin   string
	Make  string
	Model string
	Year  string
}

type CarModel struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (h *handler) GetCarsByVin(c *gin.Context) {
	model := h.searchByVin(c)
	query := `
	select distinct linkageTargetId, linkageTargetType, vehicleModelSeriesName
	from manufacturers m join
	modelseries s on manuName like ? and m.manuId=s.manuId and modelname like ? and (yearOfConstrTo is Null or yearOfConstrTo <= ?) and yearOfConstrFrom >= ? join
	linkagetargets l on vehicleModelSeriesId = s.modelId and lang='en';`
	rows, err := h.DB.Query(query, model.Make, "%"+model.Model+"%", model.Year+"12", model.Year+"00")
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

type VehicleResponse struct {
	Status string `json:"status"`
	Data   struct {
		Intro struct {
			VIN string `json:"vin"`
		} `json:"intro"`
		Basic struct {
			Make        string `json:"make"`
			Model       string `json:"model"`
			Year        string `json:"year"`
			Trim        string `json:"trim"`
			BodyType    string `json:"body_type"`
			VehicleType string `json:"vehicle_type"`
			VehicleSize string `json:"vehicle_size"`
		} `json:"basic"`
		Engine struct {
			EngineSize        string `json:"engine_size"`
			EngineDescription string `json:"engine_description"`
			EngineCapacity    string `json:"engine_capacity"`
		} `json:"engine"`
		Manufacturer struct {
			Manufacturer string `json:"manufacturer"`
			Region       string `json:"region"`
			Country      string `json:"country"`
			PlantCity    string `json:"plant_city"`
		} `json:"manufacturer"`
		Transmission struct {
			TransmissionStyle string `json:"transmission_style"`
		} `json:"transmission"`
		Restraint struct {
			Others string `json:"others"`
		} `json:"restraint"`
		Dimensions struct {
			GVWR string `json:"gvwr"`
		} `json:"dimensions"`
		Drivetrain struct {
			DriveType string `json:"drive_type"`
		} `json:"drivetrain"`
		Fuel struct {
			FuelType string `json:"fuel_type"`
		} `json:"fuel"`
	} `json:"data"`
}

func (h *handler) searchByVinRaw(c *gin.Context) []byte {
	baseurl := os.Getenv("VEHICLE_DATABASES")
	europe := "/europe-vin-decode/"
	global := "/vin-decode/"
	var vin string = c.Param("vin")

	query := `select data from vin_cache where vin like ? limit 1`
	row := h.DB.QueryRow(query, vin+"%")
	var data string
	err := row.Scan(&data)

	if err == nil {
		return []byte(data)
	}
	fmt.Println("Cache miss", err)

	body, err := getBody(baseurl + global + vin)
	if err == nil {
		h.saveRequest(vin, body)
		return body
	}

	fmt.Println("Error: received non-200 response status:", err)

	body, err = getBody(baseurl + europe + vin)
	if err == nil {
		h.saveRequest(vin, body)
		return body
	}

	log.Panic("Error: received non-200 response status:", err)
	return nil
}

func (h *handler) saveRequest(vin string, body []byte) {
	query := `INSERT INTO vin_cache (vin, data) values (?, ?)`
	fmt.Println(string(body))
	if _, err := h.DB.Exec(query, vin, string(body)); err != nil {
		log.Panic(err)
	}
}

func (h *handler) searchByVin(c *gin.Context) BaseModel {
	body := h.searchByVinRaw(c)

	var response VehicleResponse

	if err := json.Unmarshal(body, &response); err != nil {
		log.Panic(err)
	}

	fmt.Println("This is the response", response)
	model := BaseModel{
		Vin:   response.Data.Intro.VIN,
		Make:  response.Data.Basic.Make,
		Model: response.Data.Basic.Model,
		Year:  response.Data.Basic.Year,
	}

	fmt.Println("This is the model", model)
	return model
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

func (h *handler) GetAllCachedVin(c *gin.Context) {
	query := `select vin from vin_cache`
	var vins []string
	rows, err := h.DB.Query(query)
	if err != nil {
		log.Panic(err)
	}
	for rows.Next() {

		var vin string
		err = rows.Scan(&vin)
		if err != nil {
			log.Panic(err)
		}

		vins = append(vins, vin)
	}

	c.JSON(http.StatusOK, vins)

}

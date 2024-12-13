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
	Status            string `json:"status"`
	VIN               string `json:"data.intro.vin"`
	Make              string `json:"data.basic.make"`
	Model             string `json:"data.basic.model"`
	Year              string `json:"data.basic.year"`
	Trim              string `json:"data.basic.trim"`
	BodyType          string `json:"data.basic.body_type"`
	VehicleType       string `json:"data.basic.vehicle_type"`
	VehicleSize       string `json:"data.basic.vehicle_size"`
	EngineSize        string `json:"data.engine.engine_size"`
	EngineDescription string `json:"data.engine.engine_description"`
	EngineCapacity    string `json:"data.engine.engine_capacity"`
	Manufacturer      string `json:"data.manufacturer.manufacturer"`
	Region            string `json:"data.manufacturer.region"`
	Country           string `json:"data.manufacturer.country"`
	PlantCity         string `json:"data.manufacturer.plant_city"`
	TransmissionStyle string `json:"data.transmission.transmission_style"`
	RestraintOthers   string `json:"data.restraint.others"`
	GVWR              string `json:"data.dimensions.gvwr"`
	DriveType         string `json:"data.drivetrain.drive_type"`
	FuelType          string `json:"data.fuel.fuel_type"`
}

func (h *handler) searchByVinRaw(c *gin.Context) []byte {
	baseurl := os.Getenv("VEHICLE_DATABASES")
	europe := "/europe-vin-decode/"
	global := "/vin-decode/"
	var vin string = c.Param("vin")
	body, err := getBody(baseurl + global + vin)
	if err != nil {
		fmt.Println("Error: received non-200 response status:", err)
		body, err := getBody(baseurl + europe + vin)
		if err != nil {
			log.Panic("Error: received non-200 response status:", err)
		}
		return body
	}

	return body
}

func (h *handler) searchByVin(c *gin.Context) BaseModel {
	baseurl := os.Getenv("VEHICLE_DATABASES")
	// europe := "/europe-vin-decode/"
	global := "/vin-decode/"
	var vin string = c.Param("vin")
	body, err := getBody(baseurl + global + vin)
	if err != nil {
		log.Panic("Error: received non-200 response status:", err)
		// body, err := getBody(baseurl + europe + vin)
		// if err != nil {
		// 	log.Panic("Error: received non-200 response status:", err)
		// }
	}

	var response VehicleResponse

	if err := json.Unmarshal(body, &response); err != nil {
		log.Panic(err)
	}

	fmt.Println("This is the response", response)
	fmt.Println("This is the response", response.VIN)
	fmt.Println("This is the response", response.Status)
	model := BaseModel{
		Vin:   response.VIN,
		Make:  response.Make,
		Model: response.Model,
		Year:  response.Year,
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

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
	Id           int    `json:"id"`
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer_name"`
	Type         string `json:"type"`
}

func (h *handler) GetCarsByVin(c *gin.Context) {
	model := h.searchByVin(c)
	query := `
	select distinct linkageTargetId, vehicleModelSeriesName, m.manuName, linkageTargetType 
	from manufacturers m join
	modelseries s on manuName like ? and m.manuId=s.manuId and modelname like ? and (? = '' or yearOfConstrTo is Null or yearOfConstrTo <= ?) and (? = '' or yearOfConstrFrom >= ?) join
	linkagetargets l on vehicleModelSeriesId = s.modelId and lang='en';`
	rows, err := h.DB.Query(query, model.Make, "%"+model.Model+"%", model.Year, model.Year+"12", model.Year, model.Year+"00")

	if err != nil {
		log.Panic(err)
	}

	var response []CarModel
	for rows.Next() {
		var model CarModel
		if err := rows.Scan(&model.Id, &model.Name, &model.Manufacturer, &model.Type); err != nil {
			log.Panic(err)
		}

		response = append(response, model)
	}
	defer rows.Close()
	c.IndentedJSON(http.StatusOK, response)
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

	fmt.Println("Try global first")
	body, err := getBody(baseurl + global + vin)
	if body != nil {
		fmt.Println("Try to save global ")
		h.saveRequest(vin, body)
		return body
	}

	fmt.Println("Try europe first")
	body, err = getBody(baseurl + europe + vin)
	if body != nil {
		fmt.Println("Try to europe global ")
		h.saveRequest(vin, body)
		return body
	}

	log.Panic("Failed to find data", err)
	return nil
}

func (h *handler) saveRequest(vin string, body []byte) {
	query := `INSERT INTO vin_cache (vin, data) values (?, ?)`
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

	if response.Data.Intro.VIN != nil {
		fmt.Println("This is the response", response)
		model := BaseModel{
			Vin:   *response.Data.Intro.VIN,
			Make:  response.Data.Basic.Make,
			Model: response.Data.Basic.Model,
			Year:  response.Data.Basic.Year,
		}

		fmt.Println("This is the model", model)
		return model
	}

	var europeVehicle EuropeVehicle

	if err := json.Unmarshal(body, &europeVehicle); err != nil {
		log.Panic(err)
	}
	fmt.Println("This is the response", europeVehicle)
	model := BaseModel{
		Vin:   europeVehicle.VIN,
		Make:  europeVehicle.Data.GeneralInformation.Make,
		Model: europeVehicle.Data.GeneralInformation.Model,
		Year:  europeVehicle.Data.GeneralInformation.ModelYear,
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

	defer rows.Close()
	c.JSON(http.StatusOK, vins)

}

type Part struct {
	Id        int     `json:"id"`
	OemNumber string  `json:"oem_number"`
	Type      string  `json:"type"`
	Url       *string `json:"url"`
	Link      *string `json:"link"`
}

func (h *handler) GetPartByVin(c *gin.Context) {
	request := PartByVin{
		Page:     0,
		PageSize: 100,
	}
	model := h.searchByVin(c)
	query := `
	select distinct articles.legacyArticleId, o.number, articles.genericArticleDescription, al.url as link, p.url, 
	from manufacturers m 
	join modelseries s on  m.manuId=s.manuId and modelname like ? and (? = '' or yearOfConstrTo is Null or yearOfConstrTo <= ?) and (? = '' or yearOfConstrFrom >= ?)
	join linkagetargets l on vehicleModelSeriesId = s.modelId and lang='en' 
	join articlesvehicletrees a on a.linkingTargetId=l.linkageTargetId 
	join articles on articles.legacyArticleId = a.legacyArticleId 
	left join oem_number o on o.articleId = articles.legacyArticleId 
	left jion articlelinks al on al.legacyArticleId = articles.legacyArticleId 
	left join articlepdfs p on p.legacyArticleId = articles.legacyArticleId 
	where manuName like ?
	limit ? offset ?
	`
	rows, err := h.DB.Query(query, "%"+model.Model+"%", model.Year, model.Year+"12", model.Year, model.Year+"00", model.Make, request.PageSize, request.Page)
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

type VehicleResponse struct {
	Status string `json:"status"`
	Data   struct {
		Intro struct {
			VIN *string `json:"vin"`
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

type EuropeVehicle struct {
	VIN  string `json:"vin"`
	Data struct {
		Manufacturer struct {
			Note         string `json:"Note"`
			Region       string `json:"Region"`
			Country      string `json:"Country"`
			Manufacturer string `json:"Manufacturer"`
			AddressLine1 string `json:"Adress line 1"`
			AddressLine2 string `json:"Adress line 2"`
		} `json:"Manufacturer"`
		OptionalEquipment []string `json:"Optional equipment"`
		StandardEquipment []string `json:"Standard equipment"`
		VinNumberAnalyze  struct {
			VDS            string `json:"VDS"`
			WMI            string `json:"WMI"`
			VINType        string `json:"VIN type"`
			SquishVIN      string `json:"Squish VIN"`
			CheckDigit     string `json:"Check digit"`
			EnteredVIN     string `json:"Entered VIN"`
			CorrectedVIN   string `json:"Corrected VIN"`
			SerialNumber   string `json:"Serial number"`
			VISIdentifier  string `json:"VIS identifier"`
			YearIdentifier string `json:"Year identifier"`
		} `json:"Vin number analize"`
		GeneralInformation struct {
			Make           string `json:"Make"`
			Model          string `json:"Model"`
			BodyStyle      string `json:"Body style"`
			ModelYear      string `json:"Model year"`
			Transmission   string `json:"Transmission"`
			VehicleType    string `json:"Vehicle type"`
			VehicleClass   string `json:"Vehicle class"`
			ManufacturedIn string `json:"Manufactured in"`
		} `json:"General Information"`
		VehicleSpecification struct {
			BodyType            string `json:"Body type"`
			Driveline           string `json:"Driveline"`
			FuelType            string `json:"Fuel type"`
			GVWRRange           string `json:"GVWR range"`
			EngineType          string `json:"Engine type"`
			EngineValves        string `json:"Engine valves"`
			DisplacementSI      string `json:"Displacement SI"`
			NumberOfDoors       string `json:"Number of doors"`
			NumberOfSeats       string `json:"Number of seats"`
			DisplacementCID     string `json:"Displacement CID"`
			EngineKiloWatts     string `json:"Engine KiloWatts"`
			EngineCylinders     string `json:"Engine cylinders"`
			AntiBrakeSystem     string `json:"Anti-Brake System"`
			EngineHorsePower    string `json:"Engine HorsePower"`
			DisplacementNominal string `json:"Displacement Nominal"`
		} `json:"Vehicle specification"`
	} `json:"data"`
	Status string `json:"status"`
}

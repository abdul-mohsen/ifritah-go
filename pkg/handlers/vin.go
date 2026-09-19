package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	log "ifritah/web-service-gin/pkg/logging"
	"ifritah/web-service-gin/pkg/telemetry"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *handler) SearchByVin(c *gin.Context) {
	body := h.searchByVin(c)
	c.JSON(http.StatusOK, body)
}

func (h *handler) SearchByVinSkipCache(c *gin.Context) {
	body := h.searchByVinRawSkipCache(c)
	if body == nil || len(body) == 0 {
		c.Status(http.StatusBadRequest)
	} else {
		c.JSON(200, body)
	}
}

type PartByVin struct {
	Query    string `json:"query"`
	Page     int    `json:"page_number"`
	PageSize int    `json:"page_size"`
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

func (h *handler) GetCarInfoByVin(c *gin.Context) {
	model := h.searchByVin(c)
	c.JSON(http.StatusOK, model)
}

func (h *handler) GetCarsByVin(c *gin.Context) {
	model := h.searchByVin(c)
	ctx := c.Request.Context()
	query := `
	select distinct linkageTargetId, vehicleModelSeriesName, m.manuName, linkageTargetType 
	from manufacturers m 
	join modelseries s on manuName like ? and m.manuId=s.manuId and model_name like ? and (? = '' or end_year is Null or end_year >= ?) and (? = '' or start_year is Null or start_year<= ?) 
	join linkagetargets l on vehicleModelSeriesId = s.modelId and lang='en';`
	rows, err := h.DB.QueryContext(ctx, query, model.Make, "%"+model.Model+"%", model.Year, model.Year+"12", model.Year, model.Year+"00")

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var response []CarModel
	for rows.Next() {
		var model CarModel
		if err := rows.Scan(&model.Id, &model.Name, &model.Manufacturer, &model.Type); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		response = append(response, model)
	}
	defer rows.Close()
	c.IndentedJSON(http.StatusOK, response)
}

func (h *handler) searchByVinRaw(c *gin.Context) []byte {
	var vin string = c.Param("vin")
	ctx := c.Request.Context()

	query := `select data from vin_cache where vin like ? limit 1`
	row := h.DB.QueryRowContext(ctx, query, vin+"%")
	var data string
	err := row.Scan(&data)

	if err == nil {
		return []byte(data)
	}
	return h.searchByVinRawSkipCache(c)
}

func (h *handler) searchByVinRawSkipCache(c *gin.Context) []byte {
	baseurl := os.Getenv("VEHICLE_DATABASES")
	europe := "/europe-vin-decode/"
	global := "/vin-decode/"
	ctx := c.Request.Context()
	vin := strings.ToUpper(strings.TrimSpace(c.Param("vin")))
	if !isValidVIN(vin) {
		log.LogWarn(ctx, "vin.invalid")
		return nil
	}
	escapedVIN := url.PathEscape(vin)

	body, err := getBody(ctx, baseurl+global+escapedVIN)
	if err != nil {
		log.LogError(ctx, "vin.global_lookup_failed", err)
		return nil
	}
	if body != nil {
		h.saveRequest(ctx, vin, body)
		return body
	}

	body, err = getBody(ctx, baseurl+europe+escapedVIN)
	if err != nil {
		log.LogError(ctx, "vin.europe_lookup_failed", err)
		return nil
	}
	if body != nil {
		h.saveRequest(ctx, vin, body)
		return body
	}

	return nil
}

func (h *handler) saveRequest(ctx context.Context, vin string, body []byte) {
	query := `INSERT INTO vin_cache (vin, data) values (?, ?)`
	if _, err := h.DB.ExecContext(ctx, query, vin, string(body)); err != nil {
		log.LogError(ctx, "vin.cache_write_failed", err)
	}
}

func (h *handler) searchByVin(c *gin.Context) BaseModel {
	vin := strings.ToUpper(strings.TrimSpace(c.Param("vin")))
	if !isValidVIN(vin) {
		log.LogWarn(c.Request.Context(), "vin.invalid")
		return BaseModel{}
	}

	body := h.searchByVinRaw(c)

	var response VehicleResponse

	if err := json.Unmarshal(body, &response); err != nil {
		log.LogError(c.Request.Context(), "vin.response_decode_failed", err,
			slog.String("response_variant", "global"))
		return BaseModel{}
	}

	if response.Data.Intro.VIN != nil {
		model := BaseModel{
			Vin:   *response.Data.Intro.VIN,
			Make:  response.Data.Basic.Make,
			Model: strings.ReplaceAll(response.Data.Basic.Model, "-", " "),
			Year:  getYear(vin),
		}

		return model
	}

	var europeVehicle EuropeVehicle

	if err := json.Unmarshal(body, &europeVehicle); err != nil {
		log.LogError(c.Request.Context(), "vin.response_decode_failed", err,
			slog.String("response_variant", "europe"))
		return BaseModel{}
	}

	model := BaseModel{
		Vin:   europeVehicle.VIN,
		Make:  europeVehicle.Data.GeneralInformation.Make,
		Model: europeVehicle.Data.GeneralInformation.Model,
		Year:  getYear(vin),
	}

	return model

}

func isValidVIN(value string) bool {
	if len(value) != 17 {
		return false
	}
	const allowed = "ABCDEFGHJKLMNPRSTUVWXYZ0123456789"
	for _, character := range value {
		if !strings.ContainsRune(allowed, character) {
			return false
		}
	}
	return true
}

func getYear(c string) string {
	switch c[9] {
	case '1':
		return "2001"
	case '2':
		return "2002"
	case '3':
		return "2003"
	case '4':
		return "2004"
	case '5':
		return "2005"
	case '6':
		return "2006"
	case '7':
		return "2007"
	case '8':
		return "2008"
	case '9':
		return "2009"
	case 'A':
		return "2010"
	case 'B':
		return "2011"
	case 'C':
		return "2012"
	case 'D':
		return "2013"
	case 'E':
		return "2014"
	case 'F':
		return "2015"
	case 'G':
		return "2016"
	case 'H':
		return "2017"
	case 'J':
		return "2018"
	case 'K':
		return "2019"
	case 'L':
		return "2020"
	case 'M':
		return "2021"
	case 'N':
		return "2022"
	case 'P':
		return "2023"
	case 'R':
		return "2024"
	case 'S':
		return "2025"
	case 'T':
		return "2026"
	case 'V':
		return "2027"
	case 'W':
		return "2028"
	case 'X':
		return "2029"
	case 'Y':
		return "2030"
	default:
		return ""
		// ch:eck how to handle this case
	}
}

func getBody(ctx context.Context, url string) ([]byte, error) {

	key := os.Getenv("VEHICLE_DATABASES_KEY")

	if ctx == nil {
		ctx = context.Background()
	}
	spanCtx, span := telemetry.StartClientSpan(ctx, "vehicle_database.lookup")
	defer span.End()
	telemetry.SetHTTPClientRequest(span, http.MethodGet, url)

	req, err := http.NewRequestWithContext(spanCtx, http.MethodGet, url, nil)
	if err != nil {
		telemetry.RecordError(spanCtx, err, "vehicle_database.request_create")
		log.LogError(spanCtx, "vin.request_create_failed", err)
		return nil, err
	}

	req.Header.Add("x-AuthKey", key)
	telemetry.InjectHTTP(spanCtx, req.Header)

	// Create an HTTP client and perform the request
	client := &http.Client{}
	resp, err := client.Do(req) // lgtm[go/request-forgery]
	if err != nil {
		telemetry.RecordError(spanCtx, err, "vehicle_database.request")
		log.LogError(spanCtx, "vin.request_failed", err)
		return nil, err
	}
	defer resp.Body.Close() // Ensure the response body is closed
	telemetry.SetHTTPClientResponse(spanCtx, resp.StatusCode, 0)

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		log.LogWarn(spanCtx, "vin.upstream_unexpected_status",
			slog.Int("status", resp.StatusCode))
		return nil, err
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		telemetry.RecordError(spanCtx, err, "vehicle_database.response_read")
		log.LogError(spanCtx, "vin.response_read_failed", err)
		return nil, err
	}
	telemetry.SetHTTPClientResponse(spanCtx, resp.StatusCode, len(body))
	log.LogInfo(spanCtx, "vin.upstream_response_received",
		slog.Int("status", resp.StatusCode),
		slog.Int("bytes", len(body)))

	return body, nil
}

func (h *handler) GetAllCachedVin(c *gin.Context) {
	query := `select vin from vin_cache`
	var vins []string
	rows, err := h.DB.QueryContext(c.Request.Context(), query)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	for rows.Next() {

		var vin string
		err = rows.Scan(&vin)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		vins = append(vins, vin)
	}

	defer rows.Close()
	c.JSON(http.StatusOK, vins)

}

type Part struct {
	Id        *int    `json:"id"`
	OemNumber string  `json:"oem_number"`
	Type      *string `json:"type"`
	Url       *string `json:"url"`
	Link      *string `json:"link"`
}

func (h *handler) GetPartByVinDetails(c *gin.Context) {

	request := PartByVin{
		Page:     0,
		PageSize: 100,
	}

	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	model := h.searchByVin(c)
	parts := h.getPartDetailsByVinQuery(c.Request.Context(), model, request.Query, request.PageSize, request.Page)
	c.JSON(http.StatusOK, parts)

}

func (h *handler) getPartDetailsByVinQuery(ctx context.Context, model BaseModel, q string, page, pageSize int) []Part {

	query := `
	select distinct a.legacyArticleId, o.number, a.genericArticleDescription, al.url as link, p.url 
	from manufacturers m 
	join modelseries s on  m.manuId=s.manuId and match(model_name) against(?) and (? = '' or yearOfConstrTo is Null or yearOfConstrTo <= ?) and (? = '' or yearOfConstrFrom >= ?)
	join article_car t on vehicleModelSeriesId = s.modelId 
	join articles a on a.legacyArticleId = t.legacyArticleId 
	left join oem_number o on o.articleId = a.legacyArticleId 
	where manuName like ? and (? = NULL or o.clean_number like ?)
	limit ? offset ?
	`
	q = strings.ReplaceAll(strings.ReplaceAll(q, "-", ""), " ", "")
	rows, err := h.DB.QueryContext(ctx, query, model.Model, model.Year, model.Year+"12", model.Year, model.Year+"00", model.Make, q, q+"%", pageSize, page)
	if err != nil {
		log.LogError(ctx, "vin.part_details_query_failed", err)
		return nil
	}

	var parts []Part = make([]Part, 0)
	for rows.Next() {

		var part Part
		err = rows.Scan(&part.Id, &part.OemNumber, &part.Type, &part.Link, &part.Url)
		if err != nil {
			log.LogError(ctx, "vin.part_details_scan_failed", err)
			return nil
		}

		parts = append(parts, part)
	}

	defer rows.Close()
	return parts
}

func (h *handler) GetPartByVin(c *gin.Context) {

	request := PartByVin{
		Page:     0,
		PageSize: 100,
	}

	if err := c.BindJSON(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	model := h.searchByVin(c)
	parts := h.getPartByVinQuery(c.Request.Context(), model, request.Query, request.PageSize, request.Page)
	c.JSON(http.StatusOK, parts)

}

func (h *handler) getPartByVinQuery(ctx context.Context, model BaseModel, q string, pageSize, page int) []Part {

	year, _ := strconv.Atoi(model.Year)
	query := `
	select distinct a.legacyArticleId, o.number, a.genericArticleDescription
	from manufacturers m 
	join modelseries s on  m.manuId=s.manuId and match(model_name) against (? in boolean mode) and (? = '' or yearOfConstrTo is Null or yearOfConstrTo <= ?) and (? = '' or yearOfConstrFrom >= ?)
	join article_car t on vehicleModelSeriesId = s.modelId 
	join oem_number o on o.articleId = t.legacyArticleId and match(o.clean_number) against(? in boolean mode)
	join articles a on a.legacyArticleId = t.legacyArticleId 
	where match(manuName) against(? in boolean mode)
	limit ? offset ?
	`
	q = strings.ReplaceAll(strings.ReplaceAll(q, "-", ""), " ", "")
	rows, err := h.DB.QueryContext(ctx, query, "+"+model.Model, year, year, year, year, q+"*", "*"+model.Make+"*", pageSize, page)
	if err != nil {
		log.LogError(ctx, "vin.part_query_failed", err)
		return nil
	}

	var parts []Part
	for rows.Next() {

		var part Part
		err = rows.Scan(&part.Id, &part.OemNumber, &part.Type)
		if err != nil {
			log.LogError(ctx, "vin.part_scan_failed", err)
			return nil
		}

		parts = append(parts, part)
	}

	defer rows.Close()
	return parts
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

func (h *handler) DownloadAllVinPartCSV(c *gin.Context) {
	model := h.searchByVin(c)
	parts := h.getAllPartByVinQuery(c.Request.Context(), model)
	c.Writer.Header().Set("Content-Type", "text/csv")
	c.Writer.Header().Set("Content-Disposition", "attachment;filename=example.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	if err := writer.Write([]string{"legacyArticleId", "number", "type"}); err != nil {
		log.LogError(c.Request.Context(), "vin.csv_header_write_failed", err)
		c.String(http.StatusInternalServerError, "Error writing CSV header")
		return
	}

	for _, item := range parts {
		row := []string{strconv.Itoa(*item.Id), item.OemNumber, *item.Type}
		if err := writer.Write(row); err != nil {
			log.LogError(c.Request.Context(), "vin.csv_row_write_failed", err)
			c.String(http.StatusInternalServerError, "Error writing CSV data")
			return
		}
	}

	c.JSON(http.StatusOK, parts)
}

func (h *handler) getAllPartByVinQuery(ctx context.Context, model BaseModel) []Part {

	year, _ := strconv.Atoi(model.Year)
	query := `
	select distinct a.legacyArticleId, o.number, a.genericArticleDescription
	from manufacturers m 
	join modelseries s on  m.manuId=s.manuId and match(model_name) against (?) and (? = '' or yearOfConstrTo is Null or yearOfConstrTo <= ?) and (? = '' or yearOfConstrFrom >= ?)
	join article_car t on vehicleModelSeriesId = s.modelId 
	join articles a on a.legacyArticleId = t.legacyArticleId 
	join oem_number o on o.articleId = a.legacyArticleId
	where match(manuName) against(?)
	`
	rows, err := h.DB.QueryContext(ctx, query, "+"+model.Model, year, year, year, year, model.Make)
	if err != nil {
		log.LogError(ctx, "vin.all_parts_query_failed", err)
		return nil
	}

	var parts []Part
	for rows.Next() {

		var part Part
		err = rows.Scan(&part.Id, &part.OemNumber, &part.Type)
		if err != nil {
			log.LogError(ctx, "vin.all_parts_scan_failed", err)
			return nil
		}

		parts = append(parts, part)
	}

	defer rows.Close()
	return parts
}

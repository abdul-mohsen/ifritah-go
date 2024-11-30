package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func (h *handler) SearchByVin(c *gin.Context) {
	baseurl := os.Getenv("VEHICLE_DATABASES")
	europe := "/europe-vin-decode/"
	global := "/vin-decode/"
	var vin string = c.Param("vin")
	body, err := getBody(baseurl + global + vin)
	if err != nil {
		fmt.Println("Error: received non-200 response status:", err)
		body, _ := getBody(baseurl + europe + vin)
		if err != nil {
			fmt.Println("Error: received non-200 response status:", err)
		}
		c.Data(200, "json", body)
	}

	fmt.Println(body)
	c.Data(200, "json", body)
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

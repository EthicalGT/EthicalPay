package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type User struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type Transactions struct {
	Email      string `json:"email"`
	Cost       string `json:"cost"`
	HTTPMethod string `json:"httpmethod"`
	Status     string `json:"status"`
}

type Api struct {
	Email      string `json:"email"`
	Project    string `json:"project"`
	Technology string `json:"technology"`
	APIId      string `json:"apiid"`
	APIKey     string `json:"apikey"`
	CreatedOn  string `json:"createdon"`
}

func readData[T any](filePath string) ([]T, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var data []T
	err = json.Unmarshal(file, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func writeData[T any](filePath string, items []T) error {
	jsonData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, jsonData, 0644)
}

func generateRandom(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func GenerateAPIID() string  { return "EP" + generateRandom(8) }
func GenerateAPIKey() string { return generateRandom(32) }

func generateRandomStatus() string {
	rand.Seed(time.Now().UnixNano())
	if rand.Intn(2) == 0 {
		return "failed"
	}
	return "success"
}

func xorEncryptDecrypt(input, key string) string {
	output := make([]byte, len(input))
	keyLen := len(key)

	for i := range input {
		output[i] = input[i] ^ key[i%keyLen]
	}

	return string(output)
}

func main() {
	//key := "GT'SEra"
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	t := &Template{templates: template.Must(template.ParseGlob("templates/*.html"))}
	e.Renderer = t

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})

	e.POST("/userRegistration", func(c echo.Context) error {
		const userFile = "static/db/users.json"
		newUser := User{
			Name:     c.FormValue("tb1"),
			Email:    c.FormValue("tb2"),
			Password: c.FormValue("tb3"),
		}
		users, err := readData[User](userFile)
		if err != nil {
			return c.HTML(http.StatusConflict, "<script>alert('Something went wrong!'); window.location='/';</script>")
		}
		for _, user := range users {
			if user.Email == newUser.Email {
				return c.HTML(http.StatusConflict, "<script>alert('Email already registered!'); window.location='/';</script>")
			}
		}
		users = append(users, newUser)
		if err := writeData(userFile, users); err != nil {
			return c.HTML(http.StatusConflict, "<script>alert('Something went wrong!'); window.location='/';</script>")
		}
		return c.HTML(http.StatusOK, "<script>alert('Registered Successfully.'); window.location='/';</script>")
	})

	e.POST("/userLogin", func(c echo.Context) error {
		const userFile = "static/db/users.json"
		email := c.FormValue("tb1")
		password := c.FormValue("tb2")
		users, err := readData[User](userFile)
		if err != nil {
			return c.HTML(http.StatusConflict, "<script>alert('Something went wrong!'); window.location='/';</script>")
		}
		for _, user := range users {
			if user.Email == email && user.Password == password {
				encoded := base64.StdEncoding.EncodeToString([]byte(email))
				cookie := new(http.Cookie)
				cookie.Name = "email"
				cookie.Value = encoded
				cookie.Expires = time.Now().Add(12 * time.Hour)
				cookie.HttpOnly = true
				c.SetCookie(cookie)
				fmt.Println("Encrypted:", encoded)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
				}
				msg := `<html><body>
				<script>
					const form = document.createElement("form");
					form.setAttribute("method", "POST");
					form.setAttribute("action", "/home");
		
					document.body.appendChild(form);
					form.submit(); 
				</script>
				</body></html>`
				return c.HTML(http.StatusAccepted, msg)
			}
		}

		return c.HTML(http.StatusAccepted, "<script>alert('Invalid username or password!'); window.location='/';</script>")
	})

	e.POST("/home", func(c echo.Context) error {
		return c.Render(http.StatusAccepted, "home.html", nil)
	})

	e.GET("/api", func(c echo.Context) error {
		cookie, err := c.Cookie("email")
		if err != nil {
			return c.String(http.StatusUnauthorized, "No active session found, kindly login!")
		}
		fmt.Println(cookie.Value)
		decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
		if err != nil {
			fmt.Println("Error decoding base64:", err)
		} else {
			fmt.Println("Decoded:", string(decoded))
		}
		return c.Render(http.StatusOK, "apiPage.html", nil)
	})
	e.GET("/apiData", func(c echo.Context) error {
		cookie, err := c.Cookie("email")
		if err != nil {
			return c.HTML(http.StatusUnauthorized, "<script>alert('No active session found!'); window.location='/';</script>")
		}
		decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
		if err != nil {
			return c.HTML(http.StatusBadRequest, "<script>alert('Invalid session data!'); window.location='/';</script>")
		}
		email := string(decoded)
		fmt.Println("Decoded email:", email)
		const apiFile = "static/db/api.json"
		apis, err := readData[Api](apiFile)
		if err != nil {
			fmt.Println("Error reading API data:", err)
			return c.HTML(http.StatusInternalServerError, "<script>alert('Error loading API data!'); window.location='/api';</script>")
		}
		fmt.Printf("APIs Data: %+v\n", apis)
		var filteredAPIs []Api
		for _, api := range apis {
			if api.Email == email {
				filteredAPIs = append(filteredAPIs, api)
			}
		}
		fmt.Printf("Filtered APIs for %s: %+v\n", email, filteredAPIs)
		data := map[string]interface{}{
			"apis": filteredAPIs,
		}
		return c.Render(http.StatusOK, "apiList.html", data)
	})

	e.POST("/generateAPI", func(c echo.Context) error {
		const apiFile = "static/db/api.json"

		cookie, err := c.Cookie("email")
		if err != nil {
			return c.HTML(http.StatusUnauthorized, "<script>alert('No active session found!'); window.location='/';</script>")
		}

		decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
		if err != nil {
			fmt.Println("Error decoding base64:", err)
			return c.HTML(http.StatusBadRequest, "<script>alert('Invalid session data!'); window.location='/';</script>")
		}

		project := strings.TrimSpace(c.FormValue("tb1"))
		technology := strings.TrimSpace(c.FormValue("tb2"))

		if project == "" || technology == "" {
			return c.HTML(http.StatusBadRequest, "<script>alert('Project and Technology fields are required!'); window.location='/api';</script>")
		}

		apis, err := readData[Api](apiFile)
		if err != nil || len(apis) == 0 {
			newAPI := Api{
				Email:      string(decoded),
				Project:    strings.ToLower(project),
				Technology: technology,
				APIId:      GenerateAPIID(),
				APIKey:     GenerateAPIKey(),
				CreatedOn:  time.Now().Format("2006-01-02 15:04:05"),
			}

			if err := writeData(apiFile, []Api{newAPI}); err != nil {
				return c.HTML(http.StatusInternalServerError, "<script>alert('Failed to save API record!'); window.location='/api';</script>")
			}

			return c.HTML(http.StatusOK, "<script>alert('API Key Generated Successfully!'); window.location='/api';</script>")
		}

		for _, api := range apis {
			if strings.EqualFold(api.Project, project) {
				return c.HTML(http.StatusConflict, "<script>alert('Project already exists!'); window.location='/api';</script>")
			}
		}

		newAPI := Api{
			Email:      string(decoded),
			Project:    strings.ToLower(project),
			Technology: technology,
			APIId:      GenerateAPIID(),
			APIKey:     GenerateAPIKey(),
			CreatedOn:  time.Now().Format("2006-01-02 15:04:05"),
		}

		apis = append(apis, newAPI)

		if err := writeData(apiFile, apis); err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Failed to save API record!'); window.location='/api';</script>")
		}

		return c.HTML(http.StatusOK, "<script>alert('API Key Generated Successfully!'); window.location='/api';</script>")
	})

	e.POST("/paymentpage", func(c echo.Context) error {
		cost := c.FormValue("cost")
		callbackURL := c.FormValue("callbackurl")
		fmt.Println("\nCost: "+cost+"\nCallback URL: ", callbackURL)
		return c.Render(http.StatusOK, "paymentPage.html", map[string]string{
			"cost":        cost,
			"callbackurl": callbackURL,
		})
	})

	e.GET("/verifycred", func(c echo.Context) error {
		const apiFile = "static/db/api.json"
		api := c.QueryParam("tb1")
		key := c.QueryParam("tb2")
		cost := c.QueryParam("tb3")
		callbackURL := c.QueryParam("tb4")
		fmt.Println("\nReceived params:", "API:", api, "Key:", key, "Cost:", cost, "callbackURL:", callbackURL)
		msg := `<html><body><script>
    alert('Correct credentials!');
    const form = document.createElement("form");
    form.setAttribute("method", "POST");
    form.setAttribute("action", "/paymentpage");

    const inputCost = document.createElement("input");
    inputCost.type = "hidden";
    inputCost.name = "cost";
    inputCost.value = "` + template.JSEscapeString(cost) + `"; 
    form.appendChild(inputCost);
    const inputCallback = document.createElement("input");
    inputCallback.type = "hidden";
    inputCallback.name = "callbackurl";
    inputCallback.value = "` + template.JSEscapeString(callbackURL) + `"; 
    form.appendChild(inputCallback);
    document.body.appendChild(form);
    form.submit();
</script></body></html>`

		apis, err := readData[Api](apiFile)
		if err != nil {
			return c.HTML(http.StatusConflict, "<script>alert('Something went wrong!');</script>")
		}

		for _, myapi := range apis {
			if myapi.APIId == api && myapi.APIKey == key {
				return c.HTML(http.StatusAccepted, msg)
			}
		}

		return c.HTML(http.StatusUnauthorized, "<script>alert('Invalid API Credentials.'); window.location='/api';</script>")
	})

	e.POST("/paymentres", func(c echo.Context) error {
		if c.FormValue("upibtn") != "" || c.FormValue("dcbtn") != "" {
			status := generateRandomStatus()
			response := map[string]string{
				"status":  status,
				"message": "Transaction processed successfully in test mode.",
			}
			fmt.Println("\nThe Response Status: " + response[status])
			const transactionFile = "static/db/transactions.json"
			trans, err := readData[Transactions](transactionFile)
			if err != nil {
				fmt.Println("Error Reading Data!")
			}
			cookie, err2 := c.Cookie("email")
			if err2 != nil {
				return c.HTML(http.StatusUnauthorized, "<script>alert('No active session found!'); window.location='/';</script>")
			}
			decoded, err3 := base64.StdEncoding.DecodeString(cookie.Value)
			if err3 != nil {
				return c.HTML(http.StatusBadRequest, "<script>alert('Invalid session data!'); window.location='/';</script>")
			}
			email := string(decoded)

			data := Transactions{
				Email:      email,
				Cost:       c.FormValue("cost"),
				HTTPMethod: c.Request().Method,
				Status:     status,
			}
			trans = append(trans, data)
			if err := writeData(transactionFile, trans); err != nil {
				return c.HTML(http.StatusInternalServerError, "<script>alert('Failed to save API record!'); window.location='/api';</script>")
			}
			msg := `<html><body><script>
    alert('Transaction Status: ` + template.JSEscapeString(status) + `'); 
    setTimeout(() => {  // Add a short delay
        const form = document.createElement("form");
        form.setAttribute("method", "POST");
        form.setAttribute("action", "` + c.FormValue("url") + `");

        const inputI = document.createElement("input");
        inputI.type = "hidden";
        inputI.name = "result";
        inputI.value = "` + template.JSEscapeString(status) + `"; 
        form.appendChild(inputI);
        document.body.appendChild(form);
        form.submit();
    }, 500);  // Delay of 500ms
</script></body></html>`

			return c.HTML(http.StatusOK, msg)

		}
		return c.HTML(http.StatusBadRequest, "<script>alert('Request Failed.');</script>")
	})

	e.POST("/QRpaymentres", func(c echo.Context) error {
		status := generateRandomStatus()
		fmt.Println("\nurl:", c.FormValue("myurl"))
		const transactionFile = "static/db/transactions.json"
		trans, err := readData[Transactions](transactionFile)
		if err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Error reading transaction data. Please try again later.'); window.location='/api';</script>")
		}

		cookie, err2 := c.Cookie("email")
		if err2 != nil {
			return c.HTML(http.StatusUnauthorized, "<script>alert('Session expired. Please log in again.'); window.location='/';</script>")
		}

		decoded, err3 := base64.StdEncoding.DecodeString(cookie.Value)
		if err3 != nil {
			return c.HTML(http.StatusBadRequest, "<script>alert('Invalid session detected. Please log in again.'); window.location='/';</script>")
		}

		email := string(decoded)

		data := Transactions{
			Email:      email,
			Cost:       c.FormValue("cost"),
			HTTPMethod: c.Request().Method,
			Status:     status,
		}

		trans = append(trans, data)

		if err := writeData(transactionFile, trans); err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Transaction recorded failed. Please try again.'); window.location='/api';</script>")
		}
		return c.HTML(http.StatusOK, "<script>alert('Transaction Status: "+status+"'); window.location.href='"+c.FormValue("myurl")+"';</script>")
	})
	e.GET("/transactions", func(c echo.Context) error {
		const transactionFile = "static/db/transactions.json"
		trans, err := readData[Transactions](transactionFile)
		if err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Error reading transaction data. Please try again later.'); window.location='/api';</script>")
		}

		cookie, err2 := c.Cookie("email")
		if err2 != nil {
			return c.HTML(http.StatusUnauthorized, "<script>alert('Session expired. Please log in again.'); window.location='/';</script>")
		}

		decoded, err3 := base64.StdEncoding.DecodeString(cookie.Value)
		if err3 != nil {
			return c.HTML(http.StatusBadRequest, "<script>alert('Invalid session detected. Please log in again.'); window.location='/';</script>")
		}

		email := string(decoded)
		var transactionsData []Transactions
		for _, i := range trans {
			if i.Email == email {
				transactionsData = append(transactionsData, i)
			}
		}
		data := map[string]interface{}{
			"tdata": transactionsData,
		}
		return c.Render(http.StatusOK, "transactionsPage.html", data)
	})
	e.GET("/documentation", func(c echo.Context) error {
		return c.File("static/documentation/EthicalPay Documentation.pdf")
	})
	e.Static("/static", "static")
	fmt.Println("EthicalPay running at http://localhost:8000")
	e.Logger.Fatal(e.Start(":8000"))
}

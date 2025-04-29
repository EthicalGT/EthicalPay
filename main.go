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
	"strconv"
	"strings"
	"time"

	"log"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/gomail.v2"
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
	Email       string `json:"email"`
	Cost        string `json:"cost"`
	HTTPMethod  string `json:"httpmethod"`
	Status      string `json:"status"`
	DateTime    string `json:"datetime"`
	PaymentMode string `json:"paymentmode"`
}

type Api struct {
	Email      string `json:"email"`
	Project    string `json:"project"`
	Technology string `json:"technology"`
	APIId      string `json:"apiid"`
	APIKey     string `json:"apikey"`
	CreatedOn  string `json:"createdon"`
}

type OTP struct {
	Email     string `json:"email"`
	Otp       int64  `json:otp`
	CreatedOn string `json:"createdon"`
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

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func verifyCredentials(c echo.Context) error {
	const apiFile = "static/db/api.json"
	var api, key, cost, callbackURL string

	if c.Request().Method == http.MethodGet {
		api = c.QueryParam("tb1")
		key = c.QueryParam("tb2")
		cost = c.QueryParam("tb3")
		callbackURL = c.QueryParam("tb4")
	} else if c.Request().Method == http.MethodPost {
		api = c.FormValue("tb1")
		key = c.FormValue("tb2")
		cost = c.FormValue("tb3")
		callbackURL = c.FormValue("tb4")
	} else {
		return c.HTML(http.StatusMethodNotAllowed, "<script>alert('405 Method Not Allowed');</script>")
	}
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
const inputMethod = document.createElement("input");
inputMethod.type = "hidden";
inputMethod.name = "httpmethod";
inputMethod.value = "` + template.JSEscapeString(c.Request().Method) + `"; 
form.appendChild(inputMethod);
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
}

func sendEmailHandler(c echo.Context, otp int64, to string, subject string, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", "mypyschbuddy@gmail.com")
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	myotp := fmt.Sprintf("%v", otp)
	m.SetBody("text/plain", body+myotp)
	d := gomail.NewDialer("smtp.gmail.com", 587, "mypyschbuddy@gmail.com", "aoclddetchfgkscg")
	if err := d.DialAndSend(m); err != nil {
		return c.HTML(http.StatusInternalServerError, "<script>alert('Could not send email right now! Please try after some time.'); window.location='/';</script>")
	} else {
		return c.HTML(http.StatusAccepted, "<script>alert('Email sent successfully!');</script>")
	}
}
func saveDataToGitHub() {
	files := []struct {
		localPath string
		repoPath  string
	}{
		{"static/db/users.json", "static/db/users.json"},
		{"static/db/otp.json", "static/db/otp.json"},
		{"static/db/transactions.json", "static/db/transactions.json"},
		{"static/db/api.json", "static/db/api.json"},
	}

	for _, f := range files {
		go func(f struct{ localPath, repoPath string }) {
			err := pushFileToGitHub(f.localPath, f.repoPath)
			if err != nil {
				log.Println("❌ Failed to push:", f.repoPath, "-", err)
			} else {
				log.Println("✅ Successfully pushed:", f.repoPath)
			}
		}(f)
	}
}

func main() {
	//key := "GT'SEra"
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	fmt.Println("\n ENV : " + os.Getenv("GITHUB_TOKEN"))
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	t := &Template{templates: template.Must(template.ParseGlob("templates/*.html"))}
	e.Renderer = t

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})
	e.POST("/otpVerify", func(c echo.Context) error {
		rand.Seed(time.Now().UnixNano())
		otp := rand.Intn(900000) + 100000

		email := c.FormValue("tb2")

		const userFile = "static/db/users.json"
		users, err := readData[User](userFile)

		if err != nil {
			fmt.Println("No users found in the file or error reading file, proceeding with OTP generation.")
		} else {
			for _, user := range users {
				if user.Email == email {
					return c.HTML(http.StatusConflict, "<script>alert('Email already registered! Please try to login.'); window.location='/';</script>")
				}
			}
		}

		if err := sendEmailHandler(c, int64(otp), email, "EthicalPay - OTP Code for Verification", "Greetings, Your One-Time Password (OTP) for verification is: "); err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Failed to send OTP email!'); window.location='/';</script>")
		}

		pwd, err := HashPassword(c.FormValue("tb3"))
		if err != nil {
			log.Println("Error encrypting password:", err)
			return c.HTML(http.StatusInternalServerError, "<script>alert('Internal Error!'); window.location='/';</script>")
		}

		const OTPFile = "static/db/otp.json"
		otps, err := readData[OTP](OTPFile)

		if err != nil {
			otps = []OTP{}
			fmt.Println("OTP file is empty or error reading OTP data.")
		}

		newOTP := OTP{
			Email:     email,
			Otp:       int64(otp),
			CreatedOn: time.Now().String(),
		}

		otps = append(otps, newOTP)

		if err := writeData(OTPFile, otps); err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Failed to update OTP DB!'); window.location='/';</script>")
		}
		fmt.Println("DB updated with OTP.")

		msg := `<html><body><script>
		alert('OTP Sent to your email. Kindly verify to continue.');
		const form = document.createElement("form");
		form.setAttribute("method", "POST");
		form.setAttribute("action", "/otp");
		
		const fullname = document.createElement("input");
		fullname.type = "hidden";
		fullname.name = "tb1";
		fullname.value = "` + template.JSEscapeString(c.FormValue("tb1")) + `"; 
		form.appendChild(fullname);
		
		const emailInput = document.createElement("input");
		emailInput.type = "hidden";
		emailInput.name = "tb2";
		emailInput.value = "` + template.JSEscapeString(c.FormValue("tb2")) + `"; 
		form.appendChild(emailInput);
		
		const password = document.createElement("input");
		password.type = "hidden";
		password.name = "tb3";
		password.value = "` + template.JSEscapeString(pwd) + `"; 
		form.appendChild(password);
		
		document.body.appendChild(form);
		form.submit();
		</script></body></html>`
		saveDataToGitHub()
		return c.HTML(http.StatusOK, msg)
	})

	e.POST("/otp", func(c echo.Context) error {
		fmt.Println("\n" + c.FormValue("tb1") + " " + c.FormValue("tb2") + " " + c.FormValue("tb3"))
		return c.Render(http.StatusOK, "otp.html", map[string]interface{}{
			"name":  c.FormValue("tb1"),
			"email": c.FormValue("tb2"),
			"pwd":   c.FormValue("tb3"),
		})
	})

	e.POST("/userRegistration", func(c echo.Context) error {
		const userFile = "static/db/users.json"
		const otpFile = "static/db/otp.json"

		email := c.FormValue("tb2")
		enteredOtp := c.FormValue("tb")

		otps, err := readData[OTP](otpFile)
		if err != nil {
			return c.HTML(http.StatusConflict, "<script>alert('Something went wrong reading OTPs!'); window.location='/';</script>")
		}

		otpMatched := false
		for _, otpRecord := range otps {
			if otpRecord.Email == email && strconv.FormatInt(otpRecord.Otp, 10) == enteredOtp {
				otpMatched = true
				break
			}
		}

		if !otpMatched {
			return c.HTML(http.StatusConflict, "<script>alert('OTP verification failed! Please try again.'); window.location='/';</script>")
		}

		newUser := User{
			Name:     c.FormValue("tb1"),
			Email:    email,
			Password: c.FormValue("tb3"),
		}

		users, err := readData[User](userFile)
		if err != nil {
			fmt.Println("Something went wrong reading users, Maybe there is no user registered yet!")
		}

		for _, user := range users {
			if user.Email == newUser.Email {
				return c.HTML(http.StatusConflict, "<script>alert('Email already registered!'); window.location='/';</script>")
			}
		}

		users = append(users, newUser)
		if err := writeData(userFile, users); err != nil {
			return c.HTML(http.StatusConflict, "<script>alert('Something went wrong while registering!'); window.location='/';</script>")
		}
		saveDataToGitHub()
		return c.HTML(http.StatusOK, "<script>alert('Registered Successfully. Kindly Login.'); window.location='/';</script>")
	})

	e.POST("/userLogin", func(c echo.Context) error {
		const userFile = "static/db/users.json"
		email := c.FormValue("tb1")
		password := c.FormValue("tb2")

		users, err := readData[User](userFile)
		if err != nil {
			return c.HTML(http.StatusConflict, "<script>alert('Something went wrong! Please try to sign up first.'); window.location='/';</script>")
		}

		var newpwd string
		for _, user := range users {
			if user.Email == email {
				newpwd = user.Password
				break
			}
		}

		if newpwd == "" {
			return c.HTML(http.StatusConflict, "<script>alert('Invalid username or password!'); window.location='/';</script>")
		}

		mypwd := CheckPasswordHash(password, newpwd)
		if !mypwd {
			return c.HTML(http.StatusConflict, "<script>alert('Invalid username or password!'); window.location='/';</script>")
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(email))
		cookie := new(http.Cookie)
		cookie.Name = "email"
		cookie.Value = encoded
		cookie.Expires = time.Now().Add(1 * time.Hour)
		cookie.HttpOnly = true
		c.SetCookie(cookie)

		fmt.Println("Encrypted:", encoded)

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
	})

	e.POST("/home", func(c echo.Context) error {
		if c.Request().Method == "POST" || c.Request().Method == "GET" {
			fmt.Println("\nFormValues: " + c.FormValue("cost") + c.FormValue("httpmethod") + c.FormValue("datetime") + c.FormValue("result") + c.FormValue("paymode"))
		}
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
			return c.HTML(http.StatusInternalServerError, "<script>alert('Error loading API data! Please create an API first.'); window.location='/api';</script>")
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
			saveDataToGitHub()
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
		saveDataToGitHub()
		return c.HTML(http.StatusOK, "<script>alert('API Key Generated Successfully!'); window.location='/api';</script>")
	})

	e.POST("/paymentpage", func(c echo.Context) error {
		cost := c.FormValue("cost")
		method := c.FormValue("httpmethod")
		callbackURL := c.FormValue("callbackurl")
		ck1 := new(http.Cookie)
		ck1.Name = "cost"
		ck1.Value = cost
		ck1.Expires = time.Now().Add(12 * time.Hour)
		ck1.HttpOnly = true
		c.SetCookie(ck1)
		ck2 := new(http.Cookie)
		ck2.Name = "method"
		ck2.Value = method
		ck2.Expires = time.Now().Add(12 * time.Hour)
		ck2.HttpOnly = true
		c.SetCookie(ck2)
		ck3 := new(http.Cookie)
		ck3.Name = "callback"
		ck3.Value = callbackURL
		ck3.Expires = time.Now().Add(12 * time.Hour)
		ck3.HttpOnly = true
		c.SetCookie(ck3)
		fmt.Println("\nCost: "+cost+"\nCallback URL: ", callbackURL)
		return c.Render(http.StatusOK, "paymentPage.html", map[string]string{
			"cost":        cost,
			"callbackurl": callbackURL,
			"httpmethod":  method,
		})
	})

	e.POST("/verifycred", verifyCredentials)
	e.GET("/verifycred", verifyCredentials)

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
			mode := ""
			if c.FormValue("upibtn") != "" {
				mode = "UPI"
			} else {
				mode = "Debit/Credit Card"
			}
			data := Transactions{
				Email:       email,
				Cost:        c.FormValue("cost"),
				HTTPMethod:  c.FormValue("httpmethod"),
				Status:      status,
				DateTime:    time.Now().String(),
				PaymentMode: mode,
			}
			trans = append(trans, data)
			if err := writeData(transactionFile, trans); err != nil {
				return c.HTML(http.StatusInternalServerError, "<script>alert('Failed to save API record!'); window.location='/api';</script>")
			}
			msg := `<html><body><script>
    alert('Transaction Status: ` + template.JSEscapeString(status) + `'); 
        const form = document.createElement("form");
        form.setAttribute("method", "POST");
        form.setAttribute("action", "` + c.FormValue("url") + `");
        const inputI = document.createElement("input");
        inputI.type = "hidden";
        inputI.name = "result";
        inputI.value = "` + template.JSEscapeString(status) + `"; 
        
		const inputII = document.createElement("input");
        inputII.type = "hidden";
        inputII.name = "paymode";
        inputII.value = "` + template.JSEscapeString(mode) + `"; 
        
		const inputIII = document.createElement("input");
        inputIII.type = "hidden";
        inputIII.name = "datetime";
        inputIII.value = "` + template.JSEscapeString(time.Now().String()) + `";

		const inputIV = document.createElement("input");
        inputIV.type = "hidden";
        inputIV.name = "cost";
        inputIV.value = "` + template.JSEscapeString(c.FormValue("cost")) + `";
		
		const inputV = document.createElement("input");
        inputV.type = "hidden";
        inputV.name = "httpmethod";
        inputV.value = "` + template.JSEscapeString(c.FormValue("httpmethod")) + `";

		form.appendChild(inputI); 
		form.appendChild(inputII);
        form.appendChild(inputIII);
		form.appendChild(inputIV);
		form.appendChild(inputV);
		
        document.body.appendChild(form);
        form.submit();
</script></body></html>`

			return c.HTML(http.StatusOK, msg)

		}
		return c.HTML(http.StatusBadRequest, "<script>alert('Request Failed.');</script>")
	})

	e.GET("/QRpaymentres", func(c echo.Context) error {
		status := generateRandomStatus()
		fmt.Println("\nurl:", c.FormValue("myurl"))

		const transactionFile = "static/db/transactions.json"
		trans, err := readData[Transactions](transactionFile)
		if err != nil {
			fmt.Println("\nError reading transaction data.")
		}

		methodCookie, err := c.Cookie("method")
		if err != nil {
			return c.HTML(http.StatusBadRequest, "<script>alert('Missing method cookie.'); window.location='/';</script>")
		}
		costCookie, err := c.Cookie("cost")
		if err != nil {
			return c.HTML(http.StatusBadRequest, "<script>alert('Missing cost cookie.'); window.location='/';</script>")
		}
		callbackCookie, err := c.Cookie("callback")
		if err != nil {
			return c.HTML(http.StatusBadRequest, "<script>alert('Missing callback cookie.'); window.location='/';</script>")
		}
		emailCookie, err := c.Cookie("email")
		if err != nil {
			return c.HTML(http.StatusUnauthorized, "<script>alert('Session expired. Please log in again.'); window.location='/';</script>")
		}

		decodedEmail, err := base64.StdEncoding.DecodeString(emailCookie.Value)
		if err != nil {
			return c.HTML(http.StatusBadRequest, "<script>alert('Invalid session detected. Please log in again.'); window.location='/';</script>")
		}

		email := string(decodedEmail)

		data := Transactions{
			Email:       email,
			Cost:        costCookie.Value,
			HTTPMethod:  methodCookie.Value,
			Status:      status,
			DateTime:    time.Now().Format(time.RFC3339),
			PaymentMode: "QRPay",
		}

		trans = append(trans, data)

		if err := writeData(transactionFile, trans); err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Transaction failed. Please try again.'); window.location='/api';</script>")
		}

		callbackURL := callbackCookie.Value
		if !strings.HasPrefix(callbackURL, "http") {
			return c.HTML(http.StatusBadRequest, "<script>alert('Invalid callback URL.'); window.location='/';</script>")
		}

		paymode := "QRPay"
		msg := fmt.Sprintf(`<html><body><script>
		alert('Transaction Status: %s'); 
		const form = document.createElement("form");
		form.method = "POST";
		form.action = "%s";
	
		const inputI = document.createElement("input");
		inputI.type = "hidden";
		inputI.name = "result";
		inputI.value = "%s"; 
		form.appendChild(inputI);
	
		const inputII = document.createElement("input");
		inputII.type = "hidden";
		inputII.name = "paymode";
		inputII.value = "%s"; 
		form.appendChild(inputII);
	
		const inputIII = document.createElement("input");
		inputIII.type = "hidden";
		inputIII.name = "datetime";
		inputIII.value = "%s";
		form.appendChild(inputIII);
		
		const inputIV = document.createElement("input");
		inputIV.type = "hidden";
		inputIV.name = "httpmethod";
		inputIV.value = "%s";
		form.appendChild(inputIV);

		const inputV = document.createElement("input");
		inputV.type = "hidden";
		inputV.name = "cost";
		inputV.value = "%s";
		form.appendChild(inputV); 

		document.body.appendChild(form);
		form.submit();
	</script></body></html>`,
			template.JSEscapeString(status),
			template.JSEscapeString(callbackCookie.Value),
			template.JSEscapeString(status),
			template.JSEscapeString(paymode),
			template.JSEscapeString(time.Now().Format(time.RFC3339)),
			template.JSEscapeString(methodCookie.Value),
			template.JSEscapeString(costCookie.Value))

		return c.HTML(http.StatusOK, msg)
	})

	e.GET("/transactions", func(c echo.Context) error {
		const transactionFile = "static/db/transactions.json"
		trans, err := readData[Transactions](transactionFile)
		if err != nil {
			return c.HTML(http.StatusInternalServerError, "<script>alert('Error reading transaction data. No Transaction performed yet!'); window.location='/api';</script>")
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

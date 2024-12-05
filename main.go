package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"regexp"
	"strconv"
	"text/template"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// store cookie
var store = sessions.NewCookieStore([]byte("your-secret-key"))

var userdb *sql.DB
var vehicledb *sql.DB
var reservationdb *sql.DB
var billingdb *sql.DB
var promotiondb *sql.DB

// Initialize separate connections for each service
func init() {
	var err error

	// User Service DB
	dsnUser := "root:password123@tcp(127.0.0.1:3306)/user_service_db"
	userdb, err = sql.Open("mysql", dsnUser)
	if err != nil {
		log.Fatalf("Error connecting to the User Service database: %v", err)
	}
	if err := userdb.Ping(); err != nil {
		log.Fatalf("Error verifying User Service database: %v", err)
	}
	fmt.Println("User Service database connected successfully!")

	// Vehicle Service DB
	dsnVehicle := "root:password123@tcp(127.0.0.1:3306)/vehicle_service_db"
	vehicledb, err = sql.Open("mysql", dsnVehicle)
	if err != nil {
		log.Fatalf("Error connecting to the Vehicle Service database: %v", err)
	}
	if err := vehicledb.Ping(); err != nil {
		log.Fatalf("Error verifying Vehicle Service database: %v", err)
	}
	fmt.Println("Vehicle Service database connected successfully!")

	// Reservation Service DB
	dsnReservation := "root:password123@tcp(127.0.0.1:3306)/reservation_service_db"
	reservationdb, err = sql.Open("mysql", dsnReservation)
	if err != nil {
		log.Fatalf("Error connecting to the Reservation Service database: %v", err)
	}
	if err := reservationdb.Ping(); err != nil {
		log.Fatalf("Error verifying Reservation Service database: %v", err)
	}
	fmt.Println("Reservation Service database connected successfully!")

	// Billing Service DB
	dsnBilling := "root:password123@tcp(127.0.0.1:3306)/billing_service_db"
	billingdb, err = sql.Open("mysql", dsnBilling)
	if err != nil {
		log.Fatalf("Error connecting to the Billing Service database: %v", err)
	}
	if err := billingdb.Ping(); err != nil {
		log.Fatalf("Error verifying Billing Service database: %v", err)
	}
	fmt.Println("Billing Service database connected successfully!")
	// Promotion Service DB
	dsnPromotion := "root:password123@tcp(127.0.0.1:3306)/promotion_service_db"
	promotiondb, err = sql.Open("mysql", dsnPromotion)
	if err != nil {
		log.Fatalf("Error connecting to the Promotion Service database: %v", err)
	}
	if err := promotiondb.Ping(); err != nil {
		log.Fatalf("Error verifying Promotion Service database: %v", err)
	}
	fmt.Println("Promotion Service database connected successfully!")
}

// struct to represent vehicle
type Vehicle struct {
	ID           int     `json:"id"`
	LicensePlate string  `json:"license_plate"`
	Model        string  `json:"model"`
	Location     string  `json:"location"`
	HourlyRate   float64 `json:"hourly_rate"`
}

// Define Reservation struct and VehicleInfo struct
type VehicleInfo struct {
	LicensePlate string  `json:"license_plate"`
	Model        string  `json:"model"`
	Status       string  `json:"status"`
	Location     string  `json:"location"`
	HourlyRate   float64 `json:"hourly_rate"`
}

type Reservation struct {
	ID          int         `json:"id"`
	VehicleID   int         `json:"vehicle_id"`
	StartTime   string      `json:"start_time"`
	EndTime     string      `json:"end_time"`
	TotalPrice  float64     `json:"total_price"`
	Status      string      `json:"status"`
	VehicleInfo VehicleInfo `json:"vehicle_info"`
}

// Hash password
func hashPassword(password string) string {
	hash := sha256.New()
	hash.Write([]byte(password))
	return hex.EncodeToString(hash.Sum(nil))
}

// generate token
func generateVerificationToken() string {
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		log.Fatalf("Error generating token: %v", err)
	}
	return base64.URLEncoding.EncodeToString(token)
}

// Send the verification email
func sendVerificationEmail(email, token string) {
	log.Printf("Simulating email sending to %s with token: %s\n", email, token)
	// Generate the verification link
	link := fmt.Sprintf("http://localhost:8080/verify?token=%s", token)

	// Construct the message
	message := fmt.Sprintf("Subject: Email Verification\n\nClick on the following link to verify your email: %s", link)

	// Set up the email sender and SMTP server
	senderEmail := "bettercallvolt@gmail.com" // sender email
	senderPassword := "qxfcqajpzeutxvxm"      //password retrieved from app password
	smtpServer := "smtp.gmail.com"
	smtpPort := "587" // Gmail's SMTP port

	// Authentication for the SMTP server
	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpServer)

	// Send the email
	err := smtp.SendMail(
		smtpServer+":"+smtpPort,
		auth,
		senderEmail,
		[]string{email}, // recipient email
		[]byte(message), // email content
	)

	if err != nil {
		log.Printf("Error sending email: %v\n", err)
	} else {
		log.Printf("Sent email to %s with verification link: %s\n", email, link)
	}
}

// Verify Handler
func verifyHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	var userID int

	// Check if the token matches any user
	err := userdb.QueryRow("SELECT id FROM users WHERE verification_token = ?", token).Scan(&userID)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	// Mark the user as verified
	_, err = userdb.Exec("UPDATE users SET is_verified = TRUE WHERE id = ?", userID)
	if err != nil {
		http.Error(w, "Error verifying user", http.StatusInternalServerError)
		return
	}
	// Render the verification success page
	tmpl, err := template.ParseFiles("verify.html")
	if err != nil {
		log.Printf("Template parsing error: %v\n", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, struct {
		Message string
	}{Message: "Your email has been successfully verified! You can now log in."})
}

// validate email address when user registers
func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// validate password when user registers
func isValidPassword(password string) bool {
	return len(password) >= 8 // Add more complexity checks if needed
}

// register form
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Serve login page with an empty error message
		tmpl, err := template.ParseFiles("register.html")
		if err != nil {
			log.Printf("Template parsing error: %v\n", err)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, struct {
			ErrorMessage string
		}{ErrorMessage: ""})
		return
	}
	if r.Method == http.MethodPost {
		// Parse form data
		username := r.FormValue("username")
		email := r.FormValue("email")
		password := r.FormValue("password")
		log.Printf("Received registration data: username=%s, email=%s", username, email)

		// Check if the username already exists in the database
		var existingUsername string
		err := userdb.QueryRow("SELECT username FROM users WHERE username = ?", username).Scan(&existingUsername)
		if err != sql.ErrNoRows {
			// If the username exists, send an error message
			tmpl, err := template.ParseFiles("register.html")
			if err != nil {
				log.Printf("Template parsing error: %v\n", err)
				http.Error(w, "Internal server error.", http.StatusInternalServerError)
				return
			}
			// Pass error message to the template
			tmpl.Execute(w, struct {
				ErrorMessage string
			}{ErrorMessage: "Username is already taken. Please try another one."})
			return
		}
		// Check if the email already exists in the database
		var existingEmail string
		err = userdb.QueryRow("SELECT email FROM users WHERE email = ?", email).Scan(&existingEmail)
		if err != sql.ErrNoRows {
			// If the email exists, send an error message
			tmpl, err := template.ParseFiles("register.html")
			if err != nil {
				log.Printf("Template parsing error: %v\n", err)
				http.Error(w, "Internal server error.", http.StatusInternalServerError)
				return
			}
			// Pass error message to the template
			tmpl.Execute(w, struct {
				ErrorMessage string
			}{ErrorMessage: "Email is already in use. Please try another one."})
			return
		} else if !isValidEmail(email) {
			http.Error(w, "Invalid email format", http.StatusBadRequest)
			// If the email is not valid (e.g. incorrect format) throw an error message
			tmpl, err := template.ParseFiles("register.html")
			if err != nil {
				log.Printf("Template parsing error: %v\n", err)
				http.Error(w, "Internal server error.", http.StatusInternalServerError)
				return
			}
			// Pass error message to the template
			tmpl.Execute(w, struct {
				ErrorMessage string
			}{ErrorMessage: "Invalid email format."})
			return
		}
		//if password is not 8 characters or more, throw an error message
		if !isValidPassword(password) {
			http.Error(w, "Password needs to be at least 8 characters long", http.StatusBadRequest)
			// If the email is not valid (e.g. incorrect format) throw an error message
			tmpl, err := template.ParseFiles("register.html")
			if err != nil {
				log.Printf("Template parsing error: %v\n", err)
				http.Error(w, "Internal server error.", http.StatusInternalServerError)
				return
			}
			// Pass error message to the template
			tmpl.Execute(w, struct {
				ErrorMessage string
			}{ErrorMessage: "Password needs to be at least 8 characters long."})
			return
		}
		// Hash the password
		passwordHash := hashPassword(password)
		log.Println("Password hashed successfully")
		verificationToken := generateVerificationToken()
		log.Printf("Generated verification token: %s", verificationToken)

		// Insert into the database
		_, err = userdb.Exec("INSERT INTO users (username, email, password_hash, verification_token) VALUES (?, ?, ?, ?)",
			username, email, passwordHash, verificationToken)
		if err != nil {
			http.Error(w, "Failed to register user. Please try again.", http.StatusInternalServerError)
			log.Printf("Error inserting user: %v\n", err)
			return
		}
		sendVerificationEmail(email, verificationToken) // Send the verification email

		// Registration successful
		fmt.Fprintln(w, "Registration successful! Please check your email to verify your account.")
	}
}

// login form
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Serve login page with an empty error message
		tmpl, err := template.ParseFiles("login.html")
		if err != nil {
			log.Printf("Template parsing error: %v\n", err)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, struct {
			ErrorMessage string
		}{ErrorMessage: ""})
		return
	}

	if r.Method == http.MethodPost {
		// Retrieve user input
		email := r.FormValue("email")
		password := r.FormValue("password")

		// Hash user password
		passwordHash := hashPassword(password)

		// Retrieve user info from database
		var username string
		var membershipTierID, userId int
		var storedPasswordHash string
		var isVerified bool

		err := userdb.QueryRow("SELECT id, username, membership_tier_id, password_hash, is_verified FROM users WHERE email = ?", email).
			Scan(&userId, &username, &membershipTierID, &storedPasswordHash, &isVerified)

		if err == sql.ErrNoRows || passwordHash != storedPasswordHash {
			// If user does not exist or password does not match
			tmpl, err := template.ParseFiles("login.html")
			if err != nil {
				log.Printf("Template parsing error: %v\n", err)
				http.Error(w, "Internal server error.", http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, struct {
				ErrorMessage string
			}{ErrorMessage: "Invalid email or password"})
			return
		}
		if !isVerified {
			tmpl, err := template.ParseFiles("login.html")
			if err != nil {
				log.Printf("Template parsing error: %v\n", err)
				http.Error(w, "Internal server error.", http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, struct {
				ErrorMessage string
			}{ErrorMessage: "Please verify your email first."})
			return
		} else if err != nil {
			// Database error
			log.Printf("Database error: %v\n", err)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
		//save user details in session
		session, _ := store.Get(r, "user-session")
		session.Values["username"] = username
		session.Values["loggedIn"] = true
		session.Values["membershipTier"] = membershipTierID
		session.Values["UserID"] = userId
		// Debug log to verify session values
		log.Printf("Session values after login: %v", session.Values)

		err = session.Save(r, w)
		if err != nil {
			log.Printf("Error saving session: %v", err)
			http.Error(w, "Failed to save session", http.StatusInternalServerError)
			return
		}

		// Login successful
		log.Printf("User logged in: %s with Membership Tier ID: %d and user id: %d \n", username, membershipTierID, userId)
		http.Redirect(w, r, "/home", http.StatusSeeOther) // Redirect to homepage or dashboard
	}
}

// renders the home page
func homeHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	log.Printf("Session values after login: %v", session.Values)
	//check if user is logged in
	loggedIn, _ := session.Values["loggedIn"].(bool)
	username, _ := session.Values["username"].(string)

	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	//retrieve user's memmbersip tier
	var membershipTier string
	err := userdb.QueryRow("SELECT membership_tiers.name FROM users INNER JOIN membership_tiers ON users.membership_tier_id = membership_tiers.id WHERE users.username = ?", username).Scan(&membershipTier)
	if err != nil {
		log.Printf("Error retrieving membership tier: %v\n", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("home.html")
	if err != nil {
		log.Printf("Template parsing error: %v\n", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, struct {
		isLoggedIn     bool
		Username       string
		MembershipTier string
	}{
		isLoggedIn:     loggedIn,
		Username:       username,
		MembershipTier: membershipTier,
	})
	response := struct {
		Message string `json:"message"`
	}{"Login successful"}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

}

// profile page
func profileHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	log.Printf("Session values after login: %v", session.Values)
	loggedIn, _ := session.Values["loggedIn"].(bool)
	username, _ := session.Values["username"].(string)

	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		//fetch user details
		var email, membershipTier string
		err := userdb.QueryRow("select email, (select name from membership_tiers where id = membership_tier_id) as membership_tier from users where username = ?", username).Scan(&email, &membershipTier)
		if err != nil {
			log.Printf("Error fetching user details: %v", err)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
		// Render the profile page
		tmpl, err := template.ParseFiles("profile.html")
		if err != nil {
			log.Printf("Template parsing error: %v", err)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, struct {
			Username       string
			Email          string
			MembershipTier string
			ErrorMessage   string
		}{
			Username:       username,
			Email:          email,
			MembershipTier: membershipTier,
			ErrorMessage:   "",
		})
	} else if r.Method == http.MethodPost {
		//handle profile update
		newUsername := r.FormValue("username")
		newEmail := r.FormValue("email")
		newPassword := r.FormValue("password")

		//check if new username is taken
		if newUsername != username {
			var existingId int
			err := userdb.QueryRow("select id from users where username = ?", newUsername).Scan(&existingId)
			if err == nil {
				tmpl, _ := template.ParseFiles("profile.html")
				tmpl.Execute(w, struct {
					Username       string
					Email          string
					MembershipTier string
					ErrorMessage   string
				}{
					Username:       username,
					Email:          newEmail,
					MembershipTier: "",
					ErrorMessage:   "Username already taken.",
				})
				return
			}
		}

		// Update the database
		query := "UPDATE users SET username = ?, email = ?"
		args := []interface{}{newUsername, newEmail}

		if newPassword != "" {
			passwordHash := hashPassword(newPassword)
			query += ", password_hash = ?"
			args = append(args, passwordHash)
		}

		query += " WHERE username = ?"
		args = append(args, username)

		_, err := userdb.Exec(query, args...)
		if err != nil {
			log.Printf("Error updating user profile: %v", err)
			http.Error(w, "Failed to update profile. Please try again.", http.StatusInternalServerError)
			return
		}
		// Update the session username
		session.Values["username"] = newUsername
		session.Save(r, w)

		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	}

}

// MembershipTier represents a membership tier
type MembershipTier struct {
	ID           int
	Name         string
	Benefits     string
	DiscountRate float64
	Price        float64
	IsCurrent    bool
}

// Handle upgrading the user's membership tier
func upgradeMembershipHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	// Ensure the request is PUT
	if r.Method != http.MethodPut {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var userID int
	// Get the membership tier ID from URL
	membershipTierID := mux.Vars(r)["id"]
	userID = session.Values["UserID"].(int) // Get the user ID from session

	// Update the user's membership in the database
	_, err := userdb.Exec("UPDATE users SET membership_tier_id = ? WHERE id = ?", membershipTierID, userID)
	if err != nil {
		http.Error(w, "Error upgrading membership", http.StatusInternalServerError)
		log.Printf("Error updating membership: %v\n", err)
		return
	}
	// Update the session with the new membership tier
	session.Values["membershipTier"] = membershipTierID
	session.Save(r, w)

	// Respond with success message
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Membership upgraded successfully!",
	})
}

// membershipHandler renders the membership page
func membershipHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	loggedIn, _ := session.Values["loggedIn"].(bool)
	username, _ := session.Values["username"].(string)
	//check if user is logged in
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	// Retrieve the session
	membershipTier, _ := session.Values["membershipTier"].(int)

	//fetch all membership tiers from database
	rows, err := userdb.Query("select id, name, benefits, discount_rate,price from membership_tiers")
	if err != nil {
		http.Error(w, "Error loading membership tiers", http.StatusInternalServerError)
		log.Printf("Database error: %v\n", err)
		return
	}
	defer rows.Close()
	var tiers []MembershipTier
	for rows.Next() {
		// retrieve membership tier information and store under a struct
		var tier MembershipTier
		err := rows.Scan(&tier.ID, &tier.Name, &tier.Benefits, &tier.DiscountRate, &tier.Price)
		if err != nil {
			http.Error(w, "Error reading membership tiers", http.StatusInternalServerError)
			log.Printf("Database error: %v\n", err)
			return
		}
		// Mark the user's current membership
		tier.IsCurrent = (tier.ID == membershipTier)
		tiers = append(tiers, tier)
	}
	//parse and render the html page for membership page
	tmpl, err := template.ParseFiles("membership.html")
	if err != nil {
		http.Error(w, "Error loading page", http.StatusInternalServerError)
		log.Printf("Template error: %v\n", err)
		return
	}

	err = tmpl.Execute(w, struct {
		Username string
		Tiers    []MembershipTier
	}{Username: username,
		Tiers: tiers})
	if err != nil {
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		log.Printf("Template rendering error: %v\n", err)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	// Clear session
	session.Values["loggedIn"] = false
	session.Values["username"] = nil
	session.Values["UserID"] = nil
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// view available vehicles in real-time
func availableVehiclesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	//query for available vehicles
	rows, err := vehicledb.Query("select id, license_plate, model, location, hourly_rate from vehicles where status = 'available'")
	if err != nil {
		http.Error(w, "Failed to retrieve available vehicles", http.StatusInternalServerError)
		log.Printf("Database query error: %v\n", err)
		return
	}
	defer rows.Close()

	var vehicles []Vehicle
	//store vehicle data as a struct
	for rows.Next() {
		var vehicle Vehicle
		err := rows.Scan(&vehicle.ID, &vehicle.LicensePlate, &vehicle.Model, &vehicle.Location, &vehicle.HourlyRate)
		if err != nil {
			http.Error(w, "Error reading vehicle data", http.StatusInternalServerError)
			log.Printf("Row scan error: %v\n", err)
			return
		}
		vehicles = append(vehicles, vehicle)
	}
	//respond with available vehicles in JSON format
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(vehicles)
	if err != nil {
		http.Error(w, "Error encoding JSON response", http.StatusInternalServerError)
		log.Printf("JSON encoding error: %v\n", err)
	}

}

// reservation service: get available vehicles (Get/reservations/vehicle)
func availableReservationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	//call vehicle service to get available vehicles
	resp, err := http.Get("http://localhost:8081/vehicles/available")
	if err != nil {
		http.Error(w, "Error contacting Vehicle Service", http.StatusInternalServerError)
		log.Printf("Error contacting Vehicle Service: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var vehicles []Vehicle
	err = json.NewDecoder(resp.Body).Decode(&vehicles)
	if err != nil {
		http.Error(w, "Error decoding Vehicle Service response", http.StatusInternalServerError)
		log.Printf("Error decoding response: %v\n", err)
		return
	}
	// Respond with the available vehicles in JSON format
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(vehicles)
	if err != nil {
		http.Error(w, "Error encoding JSON response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v\n", err)
	}
}

// Reservation Service: Create Reservation and Reserve Vehicle (POST /reservations)
func createReservationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	// struct to represent reservation
	var reservation struct {
		UserID     int     `json:"user_id"`
		VehicleID  int     `json:"vehicle_id"`
		StartTime  string  `json:"start_time"`
		EndTime    string  `json:"end_time"`
		TotalPrice float64 `json:"total_price"`
	}

	// Decode the reservation details from the request body
	err := json.NewDecoder(r.Body).Decode(&reservation)
	if err != nil {
		http.Error(w, "Error decoding reservation data", http.StatusInternalServerError)
		log.Printf("Error decoding request body: %v\n", err)
		return
	}
	// Create reservation in the Reservation Service database
	_, err = reservationdb.Exec("INSERT INTO reservations (user_id, vehicle_id, start_time, end_time, total_price) VALUES (?, ?, ?, ?, ?)",
		reservation.UserID, reservation.VehicleID, reservation.StartTime, reservation.EndTime, reservation.TotalPrice)
	if err != nil {
		http.Error(w, "Error creating reservation", http.StatusInternalServerError)
		log.Printf("Received reservation data: %+v\n", reservation)

		log.Printf("Error inserting reservation into database: %v\n", err)
		return
	}
	// Update reservation status to 'active'
	_, err = reservationdb.Exec("UPDATE reservations SET status = 'active' WHERE vehicle_id = ? AND user_id = ? AND start_time = ?",
		reservation.VehicleID, reservation.UserID, reservation.StartTime)
	if err != nil {
		http.Error(w, "Error updating reservation status", http.StatusInternalServerError)
		log.Printf("Error updating reservation status: %v\n", err)
		return
	}

	// Call Vehicle Service to reserve the vehicle (update its status)
	vehicleServiceURL := fmt.Sprintf("http://localhost:8080/vehicles/reserve/%d", reservation.VehicleID)
	resp, err := http.Post(vehicleServiceURL, "application/json", nil)
	if err != nil {
		log.Printf("Error calling Vehicle Service: %v\n", err)
		http.Error(w, "Error reserving vehicle", http.StatusInternalServerError)
		return
	}
	log.Printf("Vehicle Service Response: %v\n", resp.StatusCode) //log the status code of the response
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Error reserving vehicle", http.StatusInternalServerError)
		return
	}

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Reservation successful, vehicle reserved.",
	})
}

// Vehicle Service: Reserve Vehicle (POST /vehicles/reserve/{vehicle_id})
func reserveVehicleHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "user-session")
	log.Printf("Session values: %v", session.Values)

	log.Println("About to handle vehicle reservation")
	// Extract vehicle ID from the URL path
	vehicleID := mux.Vars(r)["vehicle_id"]
	log.Printf("Vehicle ID from URL: %s", vehicleID)
	// Update the vehicle status in the Vehicle Service database
	_, err = vehicledb.Exec("UPDATE vehicles SET status = 'reserved' WHERE id = ?", vehicleID)
	if err != nil {
		http.Error(w, "Error updating vehicle status", http.StatusInternalServerError)
		log.Printf("Error updating vehicle status in Vehicle Service: %v\n", err)
		return
	}
	log.Printf("Vehicle ID %s status updated to 'reserved'", vehicleID)
	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Vehicle successfully reserved.",
	})
}

// availableVehiclesPageHandler serves the Available Vehicles page
func VehiclesPageHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	loggedIn, _ := session.Values["loggedIn"].(bool)
	userID, _ := session.Values["UserID"].(int)

	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Serve the HTML page for available vehicles
	tmpl, err := template.ParseFiles("availableVehicles.html")
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		log.Printf("Template parsing error: %v", err)
		return
	}

	// Pass the user ID to the template
	tmpl.Execute(w, map[string]interface{}{
		"UserID": userID,
	})
	log.Printf("user id: %d \n", userID)
}

// retrieve vehicle information to display reservation details
func getVehicleDetailsHandler(w http.ResponseWriter, r *http.Request) {
	vehicleID := mux.Vars(r)["vehicle_id"]

	var vehicle struct {
		ID           int     `json:"id"`
		LicensePlate string  `json:"license_plate"`
		Model        string  `json:"model"`
		Status       string  `json:"status"`
		Location     string  `json:"location"`
		HourlyRate   float64 `json:"hourly_rate"`
	}

	err := vehicledb.QueryRow(
		"SELECT id, license_plate, model, status, location, hourly_rate FROM vehicles WHERE id = ?",
		vehicleID,
	).Scan(&vehicle.ID, &vehicle.LicensePlate, &vehicle.Model, &vehicle.Status, &vehicle.Location, &vehicle.HourlyRate)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Vehicle not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching vehicle details", http.StatusInternalServerError)
			log.Printf("Error querying vehicle: %v\n", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vehicle)
}

// Handler to get reservations
func getReservationsHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "user-session")
	log.Printf("Session values after login: %v", session.Values)
	loggedIn, _ := session.Values["loggedIn"].(bool)
	userID, _ := session.Values["UserID"].(int)

	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Fetch reservations for the user
	rows, err := reservationdb.Query("SELECT id, vehicle_id, start_time, end_time, total_price, status FROM reservations WHERE user_id = ? and status = 'active'", userID)
	if err != nil {
		http.Error(w, "Error fetching reservations", http.StatusInternalServerError)
		log.Printf("Database query error: %v\n", err)
		return
	}
	defer rows.Close()

	var reservations []struct {
		ID          int     `json:"id"`
		VehicleID   int     `json:"vehicle_id"`
		StartTime   string  `json:"start_time"`
		EndTime     string  `json:"end_time"`
		TotalPrice  float64 `json:"total_price"`
		Status      string  `json:"status"`
		VehicleInfo struct {
			LicensePlate string  `json:"license_plate"`
			Model        string  `json:"model"`
			Status       string  `json:"status"`
			Location     string  `json:"location"`
			HourlyRate   float64 `json:"hourly_rate"`
		} `json:"vehicle_info"`
	}

	// Loop through each reservation
	for rows.Next() {
		var reservation struct {
			ID          int     `json:"id"`
			VehicleID   int     `json:"vehicle_id"`
			StartTime   string  `json:"start_time"`
			EndTime     string  `json:"end_time"`
			TotalPrice  float64 `json:"total_price"`
			Status      string  `json:"status"`
			VehicleInfo struct {
				LicensePlate string  `json:"license_plate"`
				Model        string  `json:"model"`
				Status       string  `json:"status"`
				Location     string  `json:"location"`
				HourlyRate   float64 `json:"hourly_rate"`
			} `json:"vehicle_info"`
		}

		// Scan reservation data
		err := rows.Scan(&reservation.ID, &reservation.VehicleID, &reservation.StartTime, &reservation.EndTime, &reservation.TotalPrice, &reservation.Status)
		if err != nil {
			http.Error(w, "Error scanning reservations", http.StatusInternalServerError)
			log.Printf("Error scanning row: %v\n", err)
			return
		}

		// Fetch vehicle details from Vehicle Service (assuming vehicle info is available through API)
		vehicleServiceURL := fmt.Sprintf("http://localhost:8080/vehicles/%d", reservation.VehicleID)
		resp, err := http.Get(vehicleServiceURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("Error fetching vehicle details for vehicle_id %d: %v\n", reservation.VehicleID, err)
			continue
		}

		// Decode vehicle details
		err = json.NewDecoder(resp.Body).Decode(&reservation.VehicleInfo)
		if err != nil {
			log.Printf("Error decoding vehicle details: %v\n", err)
			continue
		}

		// Append the reservation to the list
		reservations = append(reservations, reservation)
	}

	// Render the reservations page with the fetched data
	tmpl, err := template.ParseFiles("reservations.html")
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		log.Printf("Template parsing error: %v", err)
		return
	}

	// Pass the user ID and reservations data to the template
	tmpl.Execute(w, map[string]interface{}{
		"UserID":       userID,
		"Reservations": reservations, // Pass reservations to the template
	})
}

// Check if the modification is allowed (e.g., within 1 hour before the reservation start time)
func isModificationAllowed(reservationID string) bool {
	// Get the reservation start time from the database
	var startTime string
	query := "SELECT start_time FROM reservations WHERE id = ?"
	err := reservationdb.QueryRow(query, reservationID).Scan(&startTime)
	if err != nil {
		log.Printf("Error fetching reservation start time: %v\n", err)
		return false // If an error occurs, disallow modification
	}

	// Parse the start time into a time.Time object
	reservationStartTime, err := time.Parse("2006-01-02 15:04:05", startTime)
	if err != nil {
		log.Printf("Error parsing reservation start time: %v\n", err)
		return false // If parsing fails, disallow modification
	}

	// Get the current time
	currentTime := time.Now()

	// Ensure the current time is in the same timezone as reservationStartTime (set both times to UTC or local)
	// Ensure the current time is in the same timezone as reservationStartTime
	if reservationStartTime.Sub(currentTime) <= 1*time.Hour {
		return false // reservation is not allowed within 1 hour of the start time
	}

	// Check if the current time is within 1 hour of the reservation start time
	if reservationStartTime.Sub(currentTime) <= 1*time.Hour {
		log.Printf("Modification not allowed: current time (%v) is within 1 hour of reservation start time (%v)\n", currentTime, reservationStartTime)
		return false // Modification is not allowed within 1 hour of the start time
	}

	log.Printf("Current Time: %v, Reservation Start Time: %v", currentTime, reservationStartTime)
	return true // Reservation can be modified if it's more than 1 hour before the start time
}

// update reservation in the database based on user input
func updateReservationHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure the request is PUT
	if r.Method != http.MethodPut {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Get reservation ID from URL
	reservationID := mux.Vars(r)["id"]

	// Parse the request body
	var reservation struct {
		StartTime  string  `json:"start_time"`
		EndTime    string  `json:"end_time"`
		TotalPrice float64 `json:"total_price"`
	}

	err := json.NewDecoder(r.Body).Decode(&reservation)
	if err != nil {
		http.Error(w, "Error decoding request body", http.StatusInternalServerError)
		return
	}

	// Check if modification is allowed (within 1 hour policy)
	if !isModificationAllowed(reservationID) {
		http.Error(w, "Modification not allowed within 1 hour of start time", http.StatusBadRequest)
		return
	}

	// Parse the start time and end time
	startTime, err := time.Parse("2006-01-02 15:04:05", reservation.StartTime)
	if err != nil {
		http.Error(w, "Invalid start time format", http.StatusBadRequest)
		return
	}

	endTime, err := time.Parse("2006-01-02 15:04:05", reservation.EndTime)
	if err != nil {
		http.Error(w, "Invalid end time format", http.StatusBadRequest)
		return
	}

	// Update the reservation in the database
	_, err = reservationdb.Exec("UPDATE reservations SET start_time = ?, end_time = ?, total_price = ? WHERE id = ?", startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"), reservation.TotalPrice, reservationID)
	if err != nil {
		http.Error(w, "Error updating reservation", http.StatusInternalServerError)
		return
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Reservation updated successfully!",
	})
}

// updateVehicleAvailability updates vehicle status to available
func updateVehicleAvailability(vehicleID int) error {
	_, err := vehicledb.Exec("UPDATE vehicles SET status = 'available' WHERE id = ?", vehicleID)
	return err
}

// isCancellationAllowed checks if cancellation is allowed based on the policy
func isCancellationAllowed(reservationID string) bool {
	var startTime string
	query := "SELECT start_time FROM reservations WHERE id = ?"
	err := reservationdb.QueryRow(query, reservationID).Scan(&startTime)
	if err != nil {
		log.Printf("Error fetching reservation start time: %v\n", err)
		return false // If an error occurs, disallow cancellation
	}

	// Parse the start time into a time.Time object
	reservationStartTime, err := time.Parse("2006-01-02 15:04:05", startTime)
	if err != nil {
		log.Printf("Error parsing reservation start time: %v\n", err)
		return false // If parsing fails, disallow cancellation
	}

	// Get the current time
	currentTime := time.Now()

	// Ensure the current time is in the same timezone as reservationStartTime
	if reservationStartTime.Sub(currentTime) <= 1*time.Hour {
		return false // Cancellation is not allowed within 1 hour of the start time
	}
	log.Printf("Current Time: %v, Reservation Start Time: %v", currentTime, reservationStartTime)

	return true // Cancellation is allowed if it's more than 1 hour before the start time
}

// Cancel Reservation Handler with Policies
func cancelReservationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	reservationID := mux.Vars(r)["id"]

	// Check if the cancellation is allowed (e.g., within 1 hour1 before the reservation start time)
	if !isCancellationAllowed(reservationID) {
		http.Error(w, "Cancellation not allowed within 1 hour of start time", http.StatusBadRequest)
		return
	}

	// Mark the reservation as canceled in the database
	_, err := reservationdb.Exec("UPDATE reservations SET status = 'canceled' WHERE id = ?", reservationID)
	if err != nil {
		http.Error(w, "Error canceling reservation", http.StatusInternalServerError)
		return
	}
	// Get vehicle ID associated with the reservation
	var vehicleID int
	err = reservationdb.QueryRow("SELECT vehicle_id FROM reservations WHERE id = ?", reservationID).Scan(&vehicleID)
	if err != nil {
		http.Error(w, "Error fetching vehicle ID", http.StatusInternalServerError)
		return
	}

	// Update the vehicle availability to 'available'
	err = updateVehicleAvailability(vehicleID)
	if err != nil {
		http.Error(w, "Error updating vehicle availability", http.StatusInternalServerError)
		return
	}
	// Send success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Reservation canceled and vehicle status updated to 'available' successfully!",
	})
}

// getReservationDetails fetches reservation details and returns them as a struct
func getReservationDetails(reservationID int) (Reservation, error) {

	// Fetch reservation details from the database
	rows, err := reservationdb.Query("SELECT id, vehicle_id, start_time, end_time, total_price, status FROM reservations WHERE id = ? and status = 'active'", reservationID)
	if err != nil {
		return Reservation{}, fmt.Errorf("Error fetching reservations: %v", err)
	}
	defer rows.Close()

	var reservations Reservation
	if rows.Next() {
		// Scan reservation data
		err = rows.Scan(&reservations.ID, &reservations.VehicleID, &reservations.StartTime, &reservations.EndTime, &reservations.TotalPrice, &reservations.Status)
		if err != nil {
			return Reservation{}, fmt.Errorf("Error scanning reservations: %v", err)
		}

		// Fetch vehicle details from Vehicle Service (assuming vehicle info is available through API)
		vehicleServiceURL := fmt.Sprintf("http://localhost:8080/vehicles/%d", reservations.VehicleID)
		resp, err := http.Get(vehicleServiceURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			return Reservation{}, fmt.Errorf("Error fetching vehicle details for vehicle_id %d: %v", reservations.VehicleID, err)
		}

		// Decode vehicle details
		err = json.NewDecoder(resp.Body).Decode(&reservations.VehicleInfo)
		if err != nil {
			return Reservation{}, fmt.Errorf("Error decoding vehicle details: %v", err)
		}
	} else {
		// No reservation found
		return Reservation{}, fmt.Errorf("Reservation with ID %d not found", reservationID)
	}

	// Check for any error that occurred during iteration
	if err := rows.Err(); err != nil {
		return Reservation{}, fmt.Errorf("Error iterating over rows: %v", err)
	}
	return reservations, nil
}

// get discount rate
func getMembershipDiscount(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "user-session")
	if err != nil {
		log.Printf("Error retrieving session: %v", err)
		http.Error(w, "Failed to retrieve session", http.StatusInternalServerError)
		return
	}
	cookie, err := r.Cookie("user-session")
	if err != nil {
		log.Printf("Session cookie missing: %v", err)
	} else {
		log.Printf("Session cookie: %v", cookie.Value)
	}

	log.Printf("Session values before fetching membership discount: %v", session.Values)
	log.Printf("Session values: %v", session.Values)
	membershipTier, _ := session.Values["membershipTier"].(int)

	//http get
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	//query for available vehicles
	rows, err := userdb.Query("select name, discount_rate from membership_tiers where id = ?", membershipTier)
	if err != nil {
		http.Error(w, "Failed to retrieve membership details", http.StatusInternalServerError)
		log.Printf("Database query error: %v\n", err)
		return
	}
	defer rows.Close()
	// Define a struct to hold membership data
	type Membership struct {
		Name         string  `json:"name"`
		DiscountRate float64 `json:"discount_rate"`
	}

	var membership Membership

	// Scan the query result into the struct
	if rows.Next() {
		err := rows.Scan(&membership.Name, &membership.DiscountRate)
		if err != nil {
			http.Error(w, "Error reading membership data", http.StatusInternalServerError)
			log.Printf("Row scan error: %v\n", err)
			return
		}
	} else {
		http.Error(w, "Membership tier not found", http.StatusNotFound)
		return
	}

	// Respond with the membership details as JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(membership)
	if err != nil {
		http.Error(w, "Error encoding JSON response", http.StatusInternalServerError)
		log.Printf("JSON encoding error: %v\n", err)
	}

}

func getMembershipDiscountRate(membershipTierID int, r *http.Request) (float64, string, error) {
	// Construct the URL for the getMembershipDiscount API
	url := fmt.Sprintf("http://localhost:8080/membership/discount/%d", membershipTierID)
	log.Printf("Fetching membership discount from URL: %s", url)

	// Create a new HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, "", fmt.Errorf("error creating request to get membership discount: %v", err)
	}

	// Copy the session cookie from the incoming request
	cookie, err := r.Cookie("user-session")
	if err != nil {
		return 0, "", fmt.Errorf("session cookie missing: %v", err)
	}
	req.AddCookie(cookie)

	// Make the HTTP request using an HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("error making request to get membership discount: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Response Status: %s", resp.Status)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("error reading response body: %v", err)
	}

	log.Printf("Response Body: %s", string(body)) // Log the full response body for debugging

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("received non-OK response from getMembershipDiscount API: %v", resp.Status)
	}

	// Parse the response JSON to get the discount rate and name
	var membership struct {
		Name         string  `json:"name"`
		DiscountRate float64 `json:"discount_rate"`
	}
	err = json.Unmarshal(body, &membership) // Use json.Unmarshal instead of json.NewDecoder
	if err != nil {
		return 0, "", fmt.Errorf("error decoding membership discount response: %v", err)
	}

	// Return the discount rate and membership name
	return membership.DiscountRate, membership.Name, nil
}

func getPromoCodeDiscount(w http.ResponseWriter, r *http.Request) {
	promoCode := mux.Vars(r)["promoCode"]

	// Query to get the promo code details
	var promo struct {
		Code         string  `json:"code"`
		DiscountRate float64 `json:"discount_rate"`
		IsActive     bool    `json:"is_active"`
	}

	// Query to get promotion details
	query := "SELECT code, discount_rate, is_active FROM promotions WHERE code = ?"
	err := promotiondb.QueryRow(query, promoCode).Scan(&promo.Code, &promo.DiscountRate, &promo.IsActive)

	// Error handling if the promo code does not exist or any other query error occurs
	if err == sql.ErrNoRows {
		http.Error(w, "Promo code not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve promotion details", http.StatusInternalServerError)
		log.Printf("Database query error: %v\n", err)
		return
	}

	// Check if promo is active
	if !promo.IsActive {
		http.Error(w, "Promo code is not active", http.StatusBadRequest)
		return
	}

	// Respond with the promo details as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(promo); err != nil {
		http.Error(w, "Error encoding promo code details", http.StatusInternalServerError)
		log.Printf("JSON encoding error: %v\n", err)
	}
}

// Function to get promo code discount rate from the promotion service
func getPromoDiscount(promoCode string) (float64, error) {
	// Construct the URL for the promotion service API
	url := fmt.Sprintf("http://localhost:8080/promotion/discount/%s", promoCode)

	// Send the HTTP GET request to the promotion service
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error making request to get promo code discount: %v", err)
	}
	defer resp.Body.Close()

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("received non-OK response from promotion service: %v", resp.Status)
	}

	// Log the raw response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %v", err)
	}

	// Log the raw response (for debugging purposes)
	log.Printf("Promo code response: %s", string(body))

	// Parse the response JSON to get promo details
	var promo struct {
		DiscountRate float64 `json:"discount_rate"`
	}
	err = json.Unmarshal(body, &promo)
	if err != nil {
		return 0, fmt.Errorf("error decoding promo code response: %v", err)
	}

	return promo.DiscountRate, nil
}

func applyPromoCode(w http.ResponseWriter, r *http.Request) {
	// Ensure the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body to extract promo code
	var promoReq struct {
		PromoCode    string  `json:"promoCode"`    // Optional promo code
		CurrentPrice float64 `json:"currentPrice"` // Ensure the client sends the current price
	}

	// Decode the JSON body
	err := json.NewDecoder(r.Body).Decode(&promoReq)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// If no promo code is provided, return the original price without applying any discount
	if promoReq.PromoCode == "" {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"newPrice": promoReq.CurrentPrice, // Return the current price as is
		})
		if err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}
		return
	}

	// Define the promotion struct
	type Promotion struct {
		Code         string  `json:"code"`
		DiscountRate float64 `json:"discount_rate"`
		IsActive     bool    `json:"is_active"`
	}

	// Get promo details from the database
	promoCode := promoReq.PromoCode
	var promo Promotion

	// Query the promotions table to get the promo details
	query := "SELECT code, discount_rate, is_active FROM promotions WHERE code = ?"
	err = promotiondb.QueryRow(query, promoCode).Scan(&promo.Code, &promo.DiscountRate, &promo.IsActive)

	// Check if promo code exists
	if err == sql.ErrNoRows {
		http.Error(w, "Promo code not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve promo details", http.StatusInternalServerError)
		log.Printf("Database query error: %v\n", err)
		return
	}

	// Check if promo code is active
	if !promo.IsActive {
		http.Error(w, "Promo code is inactive", http.StatusBadRequest)
		return
	}

	// Calculate the new price based on the discount
	if promo.DiscountRate < 0 || promo.DiscountRate > 100 {
		http.Error(w, "Invalid promo code discount", http.StatusBadRequest)
		return
	}
	//calculate new final price if user inputs promo code
	newPrice := promoReq.CurrentPrice * (1 - promo.DiscountRate/100)

	if promoReq.CurrentPrice <= 0 {
		http.Error(w, "Invalid current price", http.StatusBadRequest)
		return
	}

	// Respond with the recalculated price
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"discountRate": promo.DiscountRate,
		"newPrice":     newPrice,
	})
	if err != nil {
		http.Error(w, "Error encoding promo response", http.StatusInternalServerError)
		log.Printf("JSON encoding error: %v\n", err)
		return
	}
}

func billingPageHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "user-session")
	if err != nil {
		log.Printf("Error retrieving session: %v", err)
		http.Error(w, "Failed to retrieve session", http.StatusInternalServerError)
		return
	}

	log.Printf("Session Values on Billing Page: %v", session.Values)
	loggedIn, ok := session.Values["loggedIn"].(bool)
	if !ok || !loggedIn {
		log.Println("User not logged in or session key 'loggedIn' missing.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID, ok := session.Values["UserID"].(int)
	if !ok {
		log.Println("Session key 'UserID' missing or invalid type.")
		http.Error(w, "Session issue: User ID not found", http.StatusInternalServerError)
		return
	}

	membershipTierID, ok := session.Values["membershipTier"].(int)
	if !ok {
		log.Println("Session key 'membershipTier' missing or invalid type.")
		http.Error(w, "Session issue: Membership tier not found", http.StatusInternalServerError)
		return
	}

	// Extract reservationID
	reservationIDStr := r.URL.Query().Get("reservationID")
	if reservationIDStr == "" {
		log.Println("Reservation ID is missing in query parameters.")
		http.Error(w, "Reservation ID is missing", http.StatusBadRequest)
		return
	}

	reservationID, err := strconv.Atoi(reservationIDStr)
	if err != nil {
		log.Printf("Error converting reservation ID to integer: %v", err)
		http.Error(w, "Invalid reservation ID", http.StatusBadRequest)
		return
	}

	log.Printf("Fetching reservation details for Reservation ID: %d", reservationID)

	// Fetch reservation details
	reservationDetails, err := getReservationDetails(reservationID)
	if err != nil {
		log.Printf("Error fetching reservation details: %v", err)
		http.Error(w, "Error fetching reservation details", http.StatusInternalServerError)
		return
	}

	log.Printf("Reservation details: %+v", reservationDetails)

	// Fetch membership discount
	membershipDiscount, membershipName, err := getMembershipDiscountRate(membershipTierID, r)
	if err != nil {
		log.Printf("Error fetching membership discount: %v", err)
		http.Error(w, "Error fetching membership discount", http.StatusInternalServerError)
		return
	}

	log.Printf("Membership discount: %f, Membership name: %s", membershipDiscount, membershipName)

	// Optional: Fetch promo code discount
	promoCode := r.URL.Query().Get("promoCode")
	var promoDiscount float64
	if promoCode != "" {
		promoDiscount, err = getPromoDiscount(promoCode)
		if err != nil {
			log.Printf("Error fetching promo code discount: %v", err)
			http.Error(w, "Error fetching promo code discount", http.StatusInternalServerError)
			return
		}
		log.Printf("Promo code discount: %f", promoDiscount)
	}

	// Ensure that discounts are valid (if they are zero or negative, set them to zero)
	if membershipDiscount < 0 || membershipDiscount > 100 {
		membershipDiscount = 0
	}
	if promoDiscount < 0 || promoDiscount > 100 {
		promoDiscount = 0
	}

	// Ensure TotalPrice is valid
	if reservationDetails.TotalPrice <= 0 {
		http.Error(w, "Invalid reservation total price", http.StatusBadRequest)
		return
	}

	// Calculate final price
	finalPrice := reservationDetails.TotalPrice * (1 - membershipDiscount/100)

	log.Printf("Calculated Final Price: %f", finalPrice)

	// Render billing template
	tmpl, err := template.ParseFiles("billing.html")
	if err != nil {
		log.Printf("Template parsing error: %v", err)
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, struct {
		ReservationDetail  Reservation
		FinalPrice         float64
		PromoCode          string
		MembershipDiscount float64
		PromoDiscount      float64 // Add this
	}{

		ReservationDetail:  reservationDetails,
		FinalPrice:         finalPrice,
		PromoCode:          promoCode,
		MembershipDiscount: membershipDiscount,
		PromoDiscount:      promoDiscount, // Pass promo discount to template
	})

	if err != nil {
		log.Printf("Template rendering error: %v", err)
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		return
	}

	log.Printf("Billing page successfully rendered for user ID: %d", userID)
}

// reservation Service: update reservation status (POST /reservation/update/{reservation_id})
func ReservationStatusHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "user-session")
	log.Printf("Session values: %v", session.Values)

	log.Println("About to update reservation status")
	// Extract reservation ID from the URL path
	reservationID := mux.Vars(r)["reservationID"]
	log.Printf("reservation ID from URL: %s", reservationID)
	// Update the reservation status in the Reservation Service database
	_, err = reservationdb.Exec("UPDATE reservation SET status = 'complete' WHERE id = ?", reservationID)
	if err != nil {
		http.Error(w, "Error updating reservation status", http.StatusInternalServerError)
		log.Printf("Error updating reservation status in reservation Service: %v\n", err)
		return
	}
	log.Printf("reservation ID %s status updated to 'complete'", reservationID)
	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Reservation successfully complete.",
	})
}

// Vehicle Service: update vehicle status (POST /vehicles/update/{vehicle_id})
func VehicleStatusHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "user-session")
	log.Printf("Session values: %v", session.Values)

	log.Println("About to update vehicle status")
	// Extract vehicle ID from the URL path
	vehicleID := mux.Vars(r)["vehicle_id"]
	log.Printf("Vehicle ID from URL: %s", vehicleID)
	// Update the vehicle status in the Vehicle Service database
	_, err = vehicledb.Exec("UPDATE vehicles SET status = 'available' WHERE id = ?", vehicleID)
	if err != nil {
		http.Error(w, "Error updating vehicle status", http.StatusInternalServerError)
		log.Printf("Error updating vehicle status in Vehicle Service: %v\n", err)
		return
	}
	log.Printf("Vehicle ID %s status updated to 'available'", vehicleID)
	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Vehicle successfully reserved.",
	})
}

// Invoice struct for sending JSON response
type Invoice struct {
	UserID             int     `json:"user_id"`
	ReservationID      int     `json:"reservation_id"`
	MembershipDiscount float64 `json:"membership_discount"`
	PromoDiscount      float64 `json:"promo_discount"`
	FinalAmount        float64 `json:"final_amount"`
	InvoiceDate        string  `json:"invoice_date"`
}

// Handler for creating an invoice
func CreateInvoice(w http.ResponseWriter, r *http.Request) {
	reservationID := mux.Vars(r)["reservationID"]
	var invoice Invoice

	// For simplicity, we assume we get final amounts and discounts from the request (usually via request body or DB)
	err := json.NewDecoder(r.Body).Decode(&invoice)
	if err != nil {
		http.Error(w, "Error decoding invoice data", http.StatusBadRequest)
		return
	}

	// Insert a new invoice record into the database
	query := `INSERT INTO invoices (user_id, reservation_id, membership_discount, promo_discount, final_amount, invoice_date)
              VALUES (?, ?, ?, ?, ?, ?)`
	_, err = billingdb.Exec(query, invoice.UserID, reservationID, invoice.MembershipDiscount, invoice.PromoDiscount, invoice.FinalAmount, time.Now())
	if err != nil {
		http.Error(w, "Error creating invoice", http.StatusInternalServerError)
		log.Printf("Error inserting invoice: %v", err)
		return
	}

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Invoice created successfully",
	})
}
func main() {
	// Create a new router
	r := mux.NewRouter()

	// Static file serving
	r.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Register handlers
	r.HandleFunc("/register", registerHandler)
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/home", homeHandler)
	r.HandleFunc("/profile", profileHandler)
	r.HandleFunc("/membership", membershipHandler)
	r.HandleFunc("/verify", verifyHandler) // Email verification handler
	r.HandleFunc("/logout", logoutHandler) // Logout handler
	// Membership Upgrade API
	r.HandleFunc("/membership/upgrade/{id}", upgradeMembershipHandler).Methods("PUT")
	// Handle available vehicles API
	r.HandleFunc("/vehicles", VehiclesPageHandler)
	r.HandleFunc("/vehicles/available", availableVehiclesHandler)
	r.HandleFunc("/reserve", createReservationHandler)
	r.HandleFunc("/reservations", getReservationsHandler).Methods("GET")
	r.HandleFunc("/vehicles/{vehicle_id}", getVehicleDetailsHandler).Methods("GET")
	//update reservation details
	r.HandleFunc("/reservations/update/{id}", updateReservationHandler).Methods("PUT")
	//cancel reservation details
	r.HandleFunc("/reservations/cancel/{id}", cancelReservationHandler).Methods("DELETE")
	//set vehicle status to 'reserved'
	r.HandleFunc("/vehicles/reserve/{vehicle_id}", reserveVehicleHandler).Methods("POST")
	// Serves the billing page
	r.HandleFunc("/billing", billingPageHandler)
	// Get Membership Discount - Fetches the user's membership discount rate and name
	r.HandleFunc("/membership/discount/{membershipTier}", getMembershipDiscount).Methods("GET")
	//post request for promo code
	r.HandleFunc("/promotion/apply", applyPromoCode).Methods("POST")
	// Get Promo Code Discount - Fetches the discount rate for a given promo code
	r.HandleFunc("/promotion/discount/{promoCode}", getPromoCodeDiscount).Methods("GET")
	r.HandleFunc("/create/invoice/{reservationID}", CreateInvoice).Methods("POST")
	// Apply CORS middleware
	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:8080"}),         // Allow only frontend's origin
		handlers.AllowedMethods([]string{"POST", "GET", "PUT", "DELETE"}),  // Allowed HTTP methods
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}), // Allowed headers
	)(r)))
}

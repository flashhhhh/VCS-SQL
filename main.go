package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Users struct {
	ID       int    `gorm:"primaryKey"`
	Name     string
	Username string `gorm:"unique"`
	Password string
}

type Products struct {
	ID    int    `gorm:"primaryKey"`
	Name  string
	Price int
}

type Orders struct {
	ID		int     		`gorm:"primaryKey"`
	UserID    int     		`gorm:"primaryKey"`
	ProductID int     		`gorm:"primaryKey"`
	Quantity  int     		`gorm:"not null"`
	CreateAt  time.Time

	// Associations
	Users    Users    `gorm:"foreignKey:UserID"`
	Products Products `gorm:"foreignKey:ProductID"`
}

type VisualizeOrder struct {
	Name string `json:"name"`
	Price       int    `json:"price"`
	Quantity    int    `json:"quantity"`
}

func main() {
	db_path := "host=localhost port=5432 user=postgres password=12345678 dbname=example"
	db, err := gorm.Open(postgres.Open(db_path), &gorm.Config{})

	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&Users{}, &Products{}, &Orders{})

	http.HandleFunc("/users/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
			}
			
			data := make(map[string]string)
			json.Unmarshal(body, &data)

			name := data["username"]
			password := data["password"]

			var user Users
			db.Where("username = ?", name).First(&user)
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
				http.Error(w, "Invalid password", http.StatusUnauthorized)
				return
			}

			// Set cookie
			cookie := http.Cookie{
				Name: "session_token",
				Value: strconv.Itoa(user.ID),
				Expires: time.Now().Add(24 * time.Hour),
			}

			http.SetCookie(w, &cookie)

			w.Write([]byte("Login successfully!"))

		}
	})

	http.HandleFunc("/users/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
			}
			
			data := make(map[string]string)
			json.Unmarshal(body, &data)

			name := data["name"]
			username := data["username"]
			password := data["password"]
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "Error hashing password", http.StatusInternalServerError)
				return
			}
			
			user := Users{Name: name, Username: username, Password: string(hashedPassword)}
			db.Create(&user)

			w.Write([]byte("Register successfully!"))
		}
	})

	http.HandleFunc("/users/addOrder", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			cookie, err := r.Cookie("session_token")
			if err != nil {
				http.Error(w, "Error reading cookie", http.StatusInternalServerError)
				return
			}

			userID, err := strconv.Atoi(cookie.Value)
			if err != nil {
				http.Error(w, "Error converting cookie value", http.StatusInternalServerError)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
			}

			var data map[string][]map[string]int
			json.Unmarshal(body, &data)
			
			orders := data["orders"]

			var maxID int
			db.Table("orders").Select("MAX(id)").Scan(&maxID)

			for _, order := range orders {
				productID := order["product_id"]
				quantity := order["quantity"]
				
				newOrder := Orders{ID: maxID + 1, UserID: userID, ProductID: productID, Quantity: quantity, CreateAt: time.Now()}
				db.Create(&newOrder)
			}
		}
	})

	http.HandleFunc("/users/getOrderByID", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			cookie, err := r.Cookie("session_token")
			if err != nil {
				http.Error(w, "Error reading cookie", http.StatusInternalServerError)
				return
			}

			userID, err := strconv.Atoi(cookie.Value)
			if err != nil {
				http.Error(w, "Error converting cookie value", http.StatusInternalServerError)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
			}

			var data map[string]int
			json.Unmarshal(body, &data)

			orderID := data["order_id"]


			var visualizeOrder []VisualizeOrder
			db.Table("orders").Where("orders.id = ? AND user_id = ?", orderID, userID).Joins("JOIN products ON orders.product_id = products.id").Select("products.name, products.price, orders.quantity").Find(&visualizeOrder)

			totalPrice := 0
			for _, order := range visualizeOrder {
				totalPrice += order.Price * order.Quantity
			}

			response := map[string]interface{}{
				"order": visualizeOrder,
				"total_price": totalPrice,
			}

			jsonResponse, err := json.Marshal(response)
			if err != nil {
				http.Error(w, "Error converting response to json", http.StatusInternalServerError)
				return
			}

			w.Write(jsonResponse)
		}
	})

	http.ListenAndServe(":8080", nil)
}
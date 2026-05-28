package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)


type CategoryData struct {
	Id   int
	Name string
}

type PostData struct {
	Id            int
	UserID        int
	CategoryId    int
	Text          string
	CategoryName  string
	AuthorName    string
	LikesCount    int
	DislikesCount int
	CommentsCount int
}

type CommentData struct {
	Id            int
	UserID        int
	PostID        int
	Text          string
	AuthorName    string
	LikesCount    int
	DislikesCount int
}

type HomeTemplateData struct {
	UserConnected   bool
	CurrentUsername string
	Categories      []CategoryData
	Posts           []PostData
}

type TopicTemplateData struct {
	UserConnected   bool
	CurrentUsername string
	Post            PostData
	Comments        []CommentData
}

var db *sql.DB
var templates *template.Template

var sessions = make(map[string]string)

func main() {
	var err error

	db, err = sql.Open("sqlite3", "./bdd/bdd.db")
	if err != nil {
		log.Fatal("Erreur d'ouverture de la BDD :", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Impossible de communiquer avec bdd.db :", err)
	}
	fmt.Println("--> Connexion à bdd.db validée avec succès ! <--")

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM Category").Scan(&count)
	if err == nil && count == 0 {
		fmt.Println("La table Category est vide. Injection de tes catégories...")
		categoriesTest := []string{"Culture générale", "Jeux vidéo", "Les mentors"}
		for _, catName := range categoriesTest {
			_, insErr := db.Exec("INSERT INTO Category (Name) VALUES (?)", catName)
			if insErr != nil {
				fmt.Println("Erreur lors de l'injection d'une catégorie :", insErr)
			}
		}
		fmt.Println("Catégories ajoutées avec succès !")
	}

	templates = template.Must(template.ParseGlob("webapp/pages/*.html"))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/topic", topicHandler)
	
	http.HandleFunc("/post/create", createPostHandler)
	http.HandleFunc("/comment/create", createCommentHandler)

	fmt.Println("Serveur en ligne sur : http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getUserSession(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("forum_session")
	if err != nil {
		return "", false
	}
	username, exists := sessions[cookie.Value]
	return username, exists
}

func getUserID(username string) int {
	var id int
	err := db.QueryRow("SELECT Id FROM Comptes WHERE Username = ?", username).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	username, connected := getUserSession(r)

	var categories []CategoryData
	rowsCat, err := db.Query("SELECT Id, Name FROM Category")
	if err == nil {
		for rowsCat.Next() {
			var c CategoryData
			if errScan := rowsCat.Scan(&c.Id, &c.Name); errScan == nil {
				categories = append(categories, c)
			}
		}
		rowsCat.Close()
	}

	var posts []PostData
	rowsPost, err := db.Query("SELECT Id, UserID, CategoryId, Text FROM Poste")
	if err == nil {
		for rowsPost.Next() {
			var p PostData
			if errScan := rowsPost.Scan(&p.Id, &p.UserID, &p.CategoryId, &p.Text); errScan == nil {
				db.QueryRow("SELECT Username FROM Comptes WHERE Id = ?", p.UserID).Scan(&p.AuthorName)
				db.QueryRow("SELECT Name FROM Category WHERE Id = ?", p.CategoryId).Scan(&p.CategoryName)
				db.QueryRow("SELECT COUNT(*) FROM Commentaire WHERE PostID = ?", p.Id).Scan(&p.CommentsCount)

				posts = append(posts, p)
			}
		}
		rowsPost.Close()
	}

	data := HomeTemplateData{
		UserConnected:   connected,
		CurrentUsername: username,
		Categories:      categories,
		Posts:           posts,
	}

	templates.ExecuteTemplate(w, "index.html", data)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Erreur chiffrement", http.StatusInternalServerError)
		return
	}

	query := `INSERT INTO Comptes (Username, Email, PasswordEncrypted) VALUES (?, ?, ?)`
	_, err = db.Exec(query, username, email, string(hashedPassword))
	if err != nil {
		http.Error(w, "Pseudo ou Email déjà pris.", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		templates.ExecuteTemplate(w, "login.html", nil)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	var dbID int
	var dbPassword string
	err := db.QueryRow("SELECT Id, PasswordEncrypted FROM Comptes WHERE Username = ?", username).Scan(&dbID, &dbPassword)
	if err != nil {
		http.Error(w, "Identifiants incorrects", http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(password))
	if err != nil {
		http.Error(w, "Identifiants incorrects", http.StatusUnauthorized)
		return
	}

	sessionToken := uuid.New().String()
	sessions[sessionToken] = username

	http.SetCookie(w, &http.Cookie{
		Name:     "forum_session",
		Value:    sessionToken,
		Expires:  time.Now().Add(12 * time.Hour),
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("forum_session")
	if err == nil {
		delete(sessions, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "forum_session",
		Value:   "",
		Expires: time.Now().Add(-time.Hour),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	username, connected := getUserSession(r)
	if !connected || r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID := getUserID(username)
	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	text := r.FormValue("text")

	_, err := db.Exec("INSERT INTO Poste (UserID, CategoryId, Text, UserID_Likes, UserID_Dislikes) VALUES (?, ?, ?, '', '')", userID, categoryID, text)
	if err != nil {
		fmt.Println("Erreur création poste :", err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func topicHandler(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.Atoi(r.URL.Query().Get("id"))
	username, connected := getUserSession(r)

	var p PostData
	err := db.QueryRow("SELECT Id, UserID, CategoryId, Text FROM Poste WHERE Id = ?", postID).Scan(&p.Id, &p.UserID, &p.CategoryId, &p.Text)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	db.QueryRow("SELECT Username FROM Comptes WHERE Id = ?", p.UserID).Scan(&p.AuthorName)
	db.QueryRow("SELECT Name FROM Category WHERE Id = ?", p.CategoryId).Scan(&p.CategoryName)

	var comments []CommentData
	rows, err := db.Query("SELECT Id, UserID, PostID, Text FROM Commentaire WHERE PostID = ?", postID)
	if err == nil {
		for rows.Next() {
			var c CommentData
			if errScan := rows.Scan(&c.Id, &c.UserID, &c.PostID, &c.Text); errScan == nil {
				db.QueryRow("SELECT Username FROM Comptes WHERE Id = ?", c.UserID).Scan(&c.AuthorName)
				comments = append(comments, c)
			}
		}
		rows.Close()
	}

	data := TopicTemplateData{
		UserConnected:   connected,
		CurrentUsername: username,
		Post:            p,
		Comments:        comments,
	}

	templates.ExecuteTemplate(w, "topic.html", data)
}

func createCommentHandler(w http.ResponseWriter, r *http.Request) {
	username, connected := getUserSession(r)
	postID, _ := strconv.Atoi(r.URL.Query().Get("topic_id"))

	if !connected || r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userID := getUserID(username)
	text := r.FormValue("text")

	_, err := db.Exec("INSERT INTO Commentaire (UserID, PostID, Text, UserID_Likes, UserID_Dislikes) VALUES (?, ?, ?, '', '')", userID, postID, text)
	if err != nil {
		fmt.Println("Erreur création commentaire :", err)
	}

	http.Redirect(w, r, "/topic?id="+strconv.Itoa(postID), http.StatusSeeOther)
}
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"text/template"

	_ "github.com/glebarez/go-sqlite"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var templates *template.Template

func main() {
	var err error

	db, err = sql.Open("sqlite", "./bdd/bdd.db")
	if err != nil {
		log.Fatal("Erreur d'ouverture de la BDD :", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Impossible de communiquer avec bdd.db :", err)
	}
	fmt.Println("--> Connexion à bdd.db réussie (Sans CGO !) <--")


	templates = template.Must(template.ParseGlob("webapp/pages/*.html"))

	
	http.HandleFunc("/", homeHandler)         
	http.HandleFunc("/login", loginHandler)      
	http.HandleFunc("/register", registerHandler) 
	http.HandleFunc("/topic", topicHandler)     

	fmt.Println("Serveur démarré sur : http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}


func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	templates.ExecuteTemplate(w, "index.html", nil)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		fmt.Println("Tentative de connexion reçue")
		return
	}
	templates.ExecuteTemplate(w, "login.html", nil)
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
		http.Error(w, "Erreur lors du traitement du mot de passe", http.StatusInternalServerError)
		return
	}

	query := `INSERT INTO Comptes (Username, Email, PasswordEncrypted) VALUES (?, ?, ?)`
	_, err = db.Exec(query, username, email, string(hashedPassword))
	
	if err != nil {
		fmt.Println("Erreur BDD à l'insertion :", err)
		http.Error(w, "Erreur d'inscription (Nom d'utilisateur ou Email déjà pris).", http.StatusBadRequest)
		return
	}

	fmt.Printf("Nouveau compte créé avec succès : %s !\n", username)
	
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func topicHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "topic.html", nil)
}
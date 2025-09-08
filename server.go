package main

import (
	"net/http"
	"sae/db"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	database, err := db.InitDB()
	if err != nil {
		panic("Impossible de se connecter à la DB")
	}
	database.AutoMigrate(&db.User{})

	router := gin.Default()

	store := cookie.NewStore([]byte("dihosvsdvibvioboaifbvoidsbvb123123DSFQSCQ45"))
	router.Use(sessions.Sessions("session", store))

	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./static")

	authRequired := func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("user")
		if user == nil {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}
		c.Next()
	}

	router.GET("/", func(c *gin.Context) {
		success := c.Query("success")
		c.HTML(http.StatusOK, "auth.html", gin.H{
			"success": success,
		})
	})

	router.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", nil)
	})

	router.POST("/register", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		if db.CheckUser(database, username) {
			c.HTML(http.StatusBadRequest, "register.html", gin.H{
				"error": "Nom d'utilisateur déjà pris",
			})
			return
		}

		user := db.User{
			Username: username,
			Password: db.HashPassword(password),
		}

		if err := database.Create(&user).Error; err != nil {
			c.HTML(http.StatusInternalServerError, "register.html", gin.H{
				"error": "Erreur serveur",
			})
			return
		}

		c.Redirect(http.StatusFound, "/?success=1")
	})

	router.POST("/auth", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		var user db.User
		if err := database.Where("username = ?", username).First(&user).Error; err != nil {
			c.HTML(http.StatusUnauthorized, "auth.html", gin.H{
				"error": "Nom d'utilisateur ou mot de passe incorrect",
			})
			return
		}

		if !db.CheckPassword(user.Password, password) {
			c.HTML(http.StatusUnauthorized, "auth.html", gin.H{
				"error": "Nom d'utilisateur ou mot de passe incorrect",
			})
			return
		}

		session := sessions.Default(c)
		session.Set("user", user.Username)
		session.Set("token", db.HashPassword(user.Password))
		session.Save()

		c.Redirect(http.StatusFound, "/form")
	})

	router.GET("/form", authRequired, func(c *gin.Context) {
		c.HTML(http.StatusOK, "form.html", nil)
	})

	router.POST("/ticket", authRequired, func(c *gin.Context) {
		title := c.PostForm("title")
		description := c.PostForm("description")
		priority := c.PostForm("priority")

		session := sessions.Default(c)
		username := session.Get("user").(string)

		ticket := db.Ticket{
			Title:       title,
			Description: description,
			User:        username,
			State:       "open",
			Priority:    priority,
		}

		if err := database.Create(&ticket).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Erreur lors de la création du ticket",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "✅ Ticket créé",
			"id":     ticket.ID,
		})
	})

	router.GET("/ticket", authRequired, func(c *gin.Context) {
		c.HTML(http.StatusOK, "ticket.html", nil)
	})

	router.GET("/tickets", authRequired, func(c *gin.Context) {
		var tickets []db.Ticket
		session := sessions.Default(c)
		username := session.Get("user").(string)

		// récupérer uniquement les tickets de l'utilisateur connecté
		if err := database.Where("user = ?", username).Find(&tickets).Error; err != nil {
			c.HTML(http.StatusInternalServerError, "tickets.html", gin.H{
				"error": "Impossible de récupérer les tickets",
			})
			return
		}

		c.HTML(http.StatusOK, "tickets.html", gin.H{
			"tickets": tickets,
		})
	})

	router.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()
		c.Redirect(http.StatusFound, "/")
	})
	router.RunTLS(":333", "cert.pem", "key.pem")
}

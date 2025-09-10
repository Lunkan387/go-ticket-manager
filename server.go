package main

import (
	"net/http"
	"sae/db"
	"strconv"
	"time"

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

	adminRequired := func(c *gin.Context) {
		sessions := sessions.Default(c)
		user := sessions.Get("user")
		if user == nil {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}

		if user.(string) != "alexis" {
			c.String(http.StatusForbidden, "t'es pas alexis fdp")
			c.Abort()
			return
		}
		c.Next()
	}

	router.GET("/admin", authRequired, adminRequired, func(c *gin.Context) {
		var users []db.User
		var tickets []db.Ticket

		database.Find(&users)
		database.Find(&tickets)

		c.HTML(http.StatusOK, "admin.html", gin.H{
			"users":   users,
			"tickets": tickets,
		})
	})

	router.POST("/admin/user/add", authRequired, adminRequired, func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		user := db.User{
			Username: username,
			Password: db.HashPassword(password),
		}
		database.Create(&user)
		c.Redirect(http.StatusFound, "/admin")
	})

	router.POST("/admin/user/edit/:id", authRequired, adminRequired, func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		username := c.PostForm("username")
		password := c.PostForm("password")

		var user db.User
		if err := database.First(&user, id).Error; err != nil {
			c.String(http.StatusNotFound, "Utilisateur introuvable")
			return
		}

		user.Username = username
		if password != "" {
			user.Password = db.HashPassword(password)
		}

		database.Save(&user)
		c.Redirect(http.StatusFound, "/admin")
	})

	router.POST("/admin/user/delete/:id", authRequired, adminRequired, func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		database.Delete(&db.User{}, id)
		c.Redirect(http.StatusFound, "/admin")
	})

	router.POST("/admin/ticket/add", authRequired, adminRequired, func(c *gin.Context) {
		title := c.PostForm("title")
		description := c.PostForm("description")
		user := c.PostForm("user")
		priority := c.PostForm("priority")

		ticket := db.Ticket{
			Title:       title,
			Description: description,
			User:        user,
			State:       "open",
			Priority:    priority,
		}
		database.Create(&ticket)
		c.Redirect(http.StatusFound, "/admin")
	})

	router.POST("/admin/ticket/edit/:id", authRequired, adminRequired, func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		title := c.PostForm("title")
		description := c.PostForm("description")
		priority := c.PostForm("priority")
		state := c.PostForm("state")

		var ticket db.Ticket
		if err := database.First(&ticket, id).Error; err != nil {
			c.String(http.StatusNotFound, "Ticket introuvable")
			return
		}

		session := sessions.Default(c)
		currentUser := session.Get("user").(string)

		// Historique
		if ticket.Title != title {
			db.LogTicketChange(database, ticket, currentUser, "Title", ticket.Title, title)
		}
		if ticket.Description != description {
			db.LogTicketChange(database, ticket, currentUser, "Description", ticket.Description, description)
		}
		if ticket.Priority != priority {
			db.LogTicketChange(database, ticket, currentUser, "Priority", ticket.Priority, priority)
		}
		if ticket.State != state {
			db.LogTicketChange(database, ticket, currentUser, "State", ticket.State, state)
		}

		ticket.Title = title
		ticket.Description = description
		ticket.Priority = priority
		ticket.State = state

		if state == "closed" {
			ticket.ClosedAt = time.Now()
		} else {
			ticket.ClosedAt = time.Time{}
		}

		database.Save(&ticket)
		c.Redirect(http.StatusFound, "/admin")
	})

	router.POST("/admin/ticket/delete/:id", authRequired, adminRequired, func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		database.Delete(&db.Ticket{}, id)
		c.Redirect(http.StatusFound, "/admin")
	})

	router.GET("/ticket/history/:id", authRequired, func(c *gin.Context) {
		ticketID, _ := strconv.Atoi(c.Param("id"))
		var history []db.TicketHistory

		if err := database.Where("ticket_id = ?", ticketID).Order("changed_at desc").Find(&history).Error; err != nil {
			c.String(http.StatusInternalServerError, "Impossible de récupérer l'historique")
			return
		}

		c.HTML(http.StatusOK, "ticket_history.html", gin.H{
			"history": history,
		})
	})

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
			Role:     "Client",
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
	router.RunTLS(":443", "cert.pem", "key.pem")
}

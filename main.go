package main

import (
	"time"

	"github.com/gin-gonic/gin"
)

type Recipe struct {
	Name        string    `json:"name"`
	Tags        []string  `json:"tags"`
	Ingredeints []string  `json:"ingredients"`
	Intructions []string  `json:"instructions"`
	PublishedAt time.Time `json:"publishedAt"`
}

func main() {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello world",
		})
	})

	router.Run()
}

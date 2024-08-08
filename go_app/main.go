package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"net/http"
	"os"
)

type Item struct {
	itemId string `json:"item_id"`
	value  string `json:"value"`
}

func main() {
	conn, err := pgx.Connect(context.Background(), "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	var greeting string
	err = conn.QueryRow(context.Background(), "select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(greeting)

	storage := make(map[string]string, 10)
	//gin.SetMode(gin.ReleaseMode) // Set to gin.DebugMode for development
	router := gin.Default()
	err = router.SetTrustedProxies(nil)
	if err != nil {
		panic(err)
	}
	router.GET("/:item_id", func(c *gin.Context) {
		itemID := c.Param("item_id")
		value, found := storage[itemID]
		if !found {
			c.String(http.StatusNotFound, "Item not found")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"value": value,
		})
	})

	router.POST("/", func(c *gin.Context) {
		var newItem Item
		if err := c.BindJSON(&newItem); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if _, found := storage[newItem.itemId]; found {
			c.String(http.StatusOK, "Item already exists")
		} else {
			storage[newItem.itemId] = newItem.value
			c.String(http.StatusCreated, "Item created")
		}
	})
	router.Run(":8000")
}

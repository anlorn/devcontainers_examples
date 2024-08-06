package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type Item struct {
	itemId string `json:"item_id"`
	value  string `json:"value"`
}

func main() {

	storage := make(map[string]string, 10)
	//gin.SetMode(gin.ReleaseMode) // Set to gin.DebugMode for development
	router := gin.Default()
	err := router.SetTrustedProxies(nil)
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

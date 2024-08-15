package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"os"
)

type Item struct {
	ItemId string `json:"item_id"`
	Value  string `json:"value"`
}

func initDBStructure(ctx context.Context, dbpool *pgxpool.Pool) error {
	if _, err := dbpool.Exec(ctx, "CREATE TABLE IF NOT EXISTS data (id text PRIMARY KEY, value text);"); err != nil {
		return err
	}
	return nil
}

func acquireDBPool(ctx context.Context) (*pgxpool.Pool, error) {
	dbpool, err := pgxpool.New(ctx, "")
	if err != nil {
		return nil, err
	}
	err = dbpool.Ping(ctx)
	if err != nil {
		dbpool.Close()
		return nil, err
	}

	go func() {
		select {
		case <-ctx.Done():
			if dbpool != nil {
				dbpool.Close()
			}
		}
	}()
	return dbpool, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dbpool, err := acquireDBPool(ctx)
	if err != nil {
		log.Printf("Failed to acquire db-pool, error %s", err)
		os.Exit(1)
	}

	err = initDBStructure(ctx, dbpool)
	if err != nil {
		log.Printf("Failed to init db structure, error %s", err)
		os.Exit(1)
	}
	//gin.SetMode(gin.ReleaseMode) // Set to gin.DebugMode for development
	router := gin.Default()
	err = router.SetTrustedProxies(nil)
	if err != nil {
		panic(err)
	}
	router.GET("/:item_id", func(c *gin.Context) {
		itemID := c.Param("item_id")
		var value string
		err := dbpool.QueryRow(c.Request.Context(), "SELECT value FROM data WHERE id = $1", itemID).Scan(&value)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.Status(http.StatusNotFound)
			} else {
				c.JSON(
					http.StatusInternalServerError,
					gin.H{"error": err.Error()},
				)
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"value": value,
		})
	})

	router.POST("/", func(c *gin.Context) {
		var newItem Item
		if err := c.ShouldBindBodyWithJSON(&newItem); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		res, err := dbpool.Exec(
			c.Request.Context(),
			"INSERT INTO data (id, value) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			newItem.ItemId, newItem.Value,
		)

		if err != nil {
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": err.Error()},
			)
			return
		}
		if res.RowsAffected() == 0 {
			c.Status(http.StatusOK)
		} else {
			c.Status(http.StatusCreated)
		}
	})
	err = router.Run(":8000")
	if err != nil {
		log.Printf("Failed to run router. Err %s", err)
	}
}

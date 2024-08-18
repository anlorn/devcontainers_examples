package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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
		for {
			select {
			case <-ctx.Done():
				if dbpool != nil {
					slog.Info("Closing db-pool...")
					dbpool.Close()
					return
				}
			case <-time.After(time.Second * 5):
				slog.Info("Waiting for db-pool to close...")
			}
		}
	}()
	return dbpool, nil
}

func main() {

	// Logging setup
	slog.SetDefault(
		slog.New(
			tint.NewHandler(
				os.Stdout,
				&tint.Options{Level: slog.LevelInfo},
			),
		),
	)

	termination := make(chan os.Signal, 1)
	signal.Notify(termination, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	appCtx, cancel := context.WithCancel(context.Background())
	dbpool, err := acquireDBPool(appCtx)

	if err != nil {
		slog.Error("Failed to create db connections pool", slog.Any("error", err))
		os.Exit(1)
	}

	err = initDBStructure(appCtx, dbpool)
	if err != nil {
		slog.Error("Failed to init DB structure", slog.Any("error", err))
		os.Exit(1)
	}
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

	srv := &http.Server{
		Addr:    ":8000",
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Failed to start server", slog.Any("error", err))
			}
		}
	}()
	<-termination
	slog.Info("Server is shutting down...")
	err = srv.Shutdown(appCtx)
	if err != nil {
		slog.Error("Failed to gracefully shutdown server", slog.Any("error", err))
	}
	cancel()

	os.Exit(0)
}

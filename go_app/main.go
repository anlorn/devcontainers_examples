package main

// The same as python app we keep all code in one file for simplicity
import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Item struct {
	ItemId string `json:"item_id"`
	Value  string `json:"value"`
}

// HttpServerPort Port where we run HTTP server. For simplicity, we keep it static instead of ENV variable for example
var HttpServerPort uint16 = 8000

// OperationsTimeout - default timeout for all operations like DB connections
var OperationsTimeout = 15 * time.Second

// initDBStructure simple replacement for real-world DB migrations, it creates initial DB structure
func initDBStructure(ctx context.Context, dbPool *pgxpool.Pool) error {
	if _, err := dbPool.Exec(ctx, "CREATE TABLE IF NOT EXISTS data (id text PRIMARY KEY, value text);"); err != nil {
		return err
	}
	slog.Info("Database structure initialized")
	return nil
}

// connectToDB creates a new database connection pool and cleans up the pool when done.
// It expects a context and WaitGroup for pool cleanup goroutine
// It returns a channel where bool must be written to clean up the pool.
func connectToDB(ctx context.Context, wg *sync.WaitGroup) (*pgxpool.Pool, chan bool, error) {
	cleanDBPoolChannel := make(chan bool, 1)
	dbPool, err := pgxpool.New(ctx, "") // for simplicity, we use env variable to define connection parameters
	if err != nil {
		return nil, cleanDBPoolChannel, err
	}
	err = dbPool.Ping(ctx)
	if err != nil {
		dbPool.Close()
		return nil, cleanDBPoolChannel, err
	}
	slog.Info("Connected to the database",
		slog.String("host", dbPool.Config().ConnConfig.Host),
		slog.Uint64("port", uint64(dbPool.Config().ConnConfig.Port)),
		slog.String("database", dbPool.Config().ConnConfig.Database),
		slog.String("user", dbPool.Config().ConnConfig.User),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-cleanDBPoolChannel:
				if dbPool != nil {
					slog.Info("Closing db-pool...")
					dbPool.Close()
					return
				}
			case <-time.After(time.Second * 5):
				slog.Debug("Waiting for db-pool to close...")
			}
		}
	}()
	return dbPool, cleanDBPoolChannel, nil
}

// createRouter initializes and configures a Gin router with GET and POST endpoints.
// For simplicity, we keep handlers code inside this function
func createRouter(dbPool *pgxpool.Pool) (*gin.Engine, error) {
	router := gin.Default()

	// In this example, we don't use any proxies
	err := router.SetTrustedProxies(nil)
	if err != nil {
		slog.Error("Failed to set trusted proxies", slog.Any("error", err))
		return nil, err
	}

	router.GET("/:item_id", func(c *gin.Context) {
		itemID := c.Param("item_id")
		var value string
		err := dbPool.QueryRow(c.Request.Context(), "SELECT value FROM data WHERE id = $1", itemID).Scan(&value)
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
		res, err := dbPool.Exec(
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
	return router, nil
}

// startServer starts an HTTP server using the provided Gin router and listens on the specified port.
// It returns the started server and a channel to receive errors that might happen during server startup.
// The server is run in a separate goroutine and the provided WaitGroup is used to wait for the server to stop.
// If an error occurs during server startup, it is sent to the error channel.
func startServer(router *gin.Engine, wg *sync.WaitGroup, port uint16) (*http.Server, chan error) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
	errChan := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("Starting HTTP server", slog.String("port", fmt.Sprintf("%d", port)))
		if err := srv.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				errChan <- err
			}
		}
		close(errChan)
	}()
	return srv, errChan
}

// gracefulShutdown gracefully shuts down the server and database connections.
// It waits for the server to stop and the database pool to close.
// If success is true, it means the shutdown was initiated by an OS signal.
// In this case, it logs a success message and exits with code 0.
// If causedByOSSignal is false, it means the shutdown was initiated by an error.
// In this case, it logs a warning message and exits with code 1.
func gracefulShutdown(success bool, srv *http.Server, wg *sync.WaitGroup, cleanDBPoolChannel chan bool) {
	slog.Info("Server is shutting down...")
	ctx, cancelServerShutdown := context.WithTimeout(context.Background(), OperationsTimeout)
	defer cancelServerShutdown()
	err := srv.Shutdown(ctx)
	if err != nil {
		slog.Error("Failed to gracefully shutdown server", slog.Any("error", err))
	}
	cleanDBPoolChannel <- true // Signal db pool to close when server is shutting down
	wg.Wait()
	if success && err == nil { // we got OS signal to stop, and we didn't get any error during shutdown
		slog.Info("Server gracefully shut down")
		os.Exit(0)
	} else { // something went wrong, channel was just closed by us
		slog.Warn("Server terminated, check logs for errors")
		os.Exit(1)
	}
}

func main() {
	// Basic logging setup, we print INFO and above to STDOUT
	slog.SetDefault(
		slog.New(
			tint.NewHandler(
				os.Stdout,
				&tint.Options{Level: slog.LevelInfo},
			),
		),
	)

	var interruptAppInitialization = false
	// Wait group to wait for db pool to close and for HTTP server to stop
	wg := &sync.WaitGroup{}

	// Create a channel to receive OS signals when we need to stop the server
	// this channel can be CLOSED by main goroutine if app initialization failed and we have to stop right away
	termination := make(chan os.Signal, 1)
	signal.Notify(termination, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Connect to DB and create connections pool for handlers
	ctx, cancelDBConnect := context.WithTimeout(context.Background(), OperationsTimeout)
	defer cancelDBConnect() // ensure we always call it to avoid leakage
	dbPool, cleanDBPoolChannel, err := connectToDB(ctx, wg)
	if err != nil {
		slog.Error("Failed to create db connections pool", slog.Any("error", err))
		close(termination)
		interruptAppInitialization = true
	}

	// Initialize DB structure if DB connection didn't fail'
	if !interruptAppInitialization {
		ctx, cancelInitDB := context.WithTimeout(context.Background(), OperationsTimeout)
		defer cancelInitDB() // ensure we always call it just in case, to avoid leakage
		err = initDBStructure(ctx, dbPool)
		if err != nil {
			slog.Error("Failed to init DB structure", slog.Any("error", err))
			close(termination)
			interruptAppInitialization = true
		}
	}

	// Create a new Gin router with handlers, if app initialization didn't fail
	var router *gin.Engine
	if !interruptAppInitialization {
		// Create a new Gin router and start the server
		router, err = createRouter(dbPool)
		if err != nil {
			slog.Error("Failed to create router", slog.Any("error", err))
			close(termination)
			interruptAppInitialization = true
		}
	}

	// Start HTTP server
	var srv *http.Server
	var serverStartErrChan chan error
	if !interruptAppInitialization {
		srv, serverStartErrChan = startServer(router, wg, HttpServerPort)
		slog.Info("Server started, and ready to serve requests")
	}

	// Wait for one of the signals to stop the app
	select {
	case <-serverStartErrChan: // Server failed to start, stop app with 1 exit code
		slog.Error("Failed to start server", slog.Any("error", serverStartErrChan))
		gracefulShutdown(false, srv, wg, cleanDBPoolChannel)
	case _, ok := <-termination: // App was terminated by an OS signal, or by us closing the channel(which means error)
		slog.Debug("Will stop the app", slog.Bool("caused_by_os_signal", ok))
		gracefulShutdown(ok, srv, wg, cleanDBPoolChannel)
	}
}

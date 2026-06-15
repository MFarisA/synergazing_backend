package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/swagger"
	"github.com/joho/godotenv"
	"synergazing.com/synergazing/config"
	"synergazing.com/synergazing/migrations"
	"synergazing.com/synergazing/routes"
	"synergazing.com/synergazing/service"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Failed to load ENV")
	}

	config.ConnectEnvDBConfig()
	db := config.GetDB()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "fresh":
			log.Println("Running with fresh migration...")
			migrations.MigrateFresh(db)
			return
		case "drop":
			if len(os.Args) < 3 {
				log.Fatal("Please provide the table name to drop: e.g., `go run main.go drop role`")
			}
			tableName := os.Args[2]
			log.Printf("Dropping table: %s", tableName)
			migrations.DropTableByName(db, tableName)
			return
		case "drop-column":
			log.Println("Dropping worker_type column from projects table...")
			err := migrations.DropWorkerTypeColumn(db)
			if err != nil {
				log.Fatalf("Failed to drop worker_type column: %v", err)
			}
			return
		case "migrate-otp":
			log.Println("Running OTP migration...")
			err := migrations.MigrateOTP(db)
			if err != nil {
				log.Fatalf("Failed to migrate OTP: %v", err)
			}
			return
		case "cleanup-otp":
			log.Println("Cleaning up expired OTPs...")
			migrations.CleanupExpiredOTPs(db)
			return
		case "seed":
			log.Println("Running database seeder...")
			migrations.SeedDatabase(db)
			return
		}
	}

	log.Println("Running with auto migration...")
	migrations.AutoMigrate(db)

	go startOTPCleanupRoutine()
	go startNotificationRoutine()

	app := fiber.New()

	// Get allowed origins from environment variable
	allowedOrigins := os.Getenv("FRONTEND_URL")
	if allowedOrigins == "" {
		log.Fatalf("FRONTEND_URL environment variable is required")
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,Access-Control-Allow-Origin,Connection,Upgrade,Sec-WebSocket-Key,Sec-WebSocket-Version,Sec-WebSocket-Protocol",
		AllowCredentials: true,
	}))
	app.Static("/storage", "./storage")

	if _, err := os.Stat("./Api.yml"); os.IsNotExist(err) {
		log.Println("Warning: Api.yml not found in root directory")
	}

	app.Get("/api/docs/doc.yaml", func(c *fiber.Ctx) error {
		absPath, err := filepath.Abs("./Api.yml")
		if err != nil {
			log.Printf("Error getting absolute path: %v", err)
			return c.Status(500).SendString("Error loading API spec")
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			log.Printf("Api.yml not found at: %s", absPath)
			return c.Status(404).SendString("API specification not found")
		}

		c.Set("Content-Type", "application/x-yaml")
		c.Set("Access-Control-Allow-Origin", "*")
		return c.SendFile("./Api.yml")
	})

	app.Get("/api/docs/*", swagger.New(swagger.Config{
		URL:          "/api/docs/doc.yaml",
		DeepLinking:  false,
		DocExpansion: "none",
	}))

	app.Get("/api/docs", func(c *fiber.Ctx) error {
		return c.Redirect("/api/docs/")
	})
	routes.SetupAuthRoutes(app)
	routes.SetupProjectRoutes(app)
	routes.SetupProfileRoutes(app)
	routes.SetupUserRoutes(app)
	routes.SkillRoutes(app)
	routes.SetupChatRoutes(app)
	routes.SetupNotificationRoutes(app)
	routes.SetupProjectMemberRoutes(app)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello World - GORM Connected!")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3002"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))

}

func startOTPCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	otpService := service.NewOTPService()
	otpService.CleanupExpiredOTPs()
	log.Println("Initial OTP cleanup completed")

	for range ticker.C {
		otpService.CleanupExpiredOTPs()
	}
}

func startNotificationRoutine() {
	ticker := time.NewTicker(24 * time.Hour) // Check daily
	defer ticker.Stop()

	db := config.GetDB()
	notificationService := service.NewNotificationService(db)

	// Initial check
	if err := notificationService.CheckAndNotifyApproachingDeadlines(); err != nil {
		log.Printf("Error in initial deadline notification check: %v", err)
	} else {
		log.Println("Initial deadline notification check completed")
	}

	for range ticker.C {
		if err := notificationService.CheckAndNotifyApproachingDeadlines(); err != nil {
			log.Printf("Error checking approaching deadlines: %v", err)
		} else {
			log.Println("Deadline notification check completed")
		}
	}
}
